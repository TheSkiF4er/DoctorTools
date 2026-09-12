package configdoctor

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gitflic.ru/skif4er/doctortools/internal/core"
	"gitflic.ru/skif4er/doctortools/internal/version"
)

type Options struct {
	JSON      bool
	Output    string
	Recursive bool
	MaxFiles  int
}
type FileInfo struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Mode string `json:"mode,omitempty"`
}
type Report struct {
	Tool       string            `json:"tool"`
	Version    string            `json:"tool_version"`
	Platform   string            `json:"platform"`
	Target     string            `json:"target"`
	Command    string            `json:"command"`
	StartedAt  string            `json:"started_at"`
	FinishedAt string            `json:"finished_at"`
	Summary    map[string]int    `json:"summary"`
	Files      []FileInfo        `json:"files"`
	Findings   []core.Finding    `json:"findings"`
	Artifacts  map[string]string `json:"artifacts"`
}
type snapshot struct {
	Root  string
	Files []FileInfo
	Text  map[string]string
}
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		help(stdout)
		return nil
	}
	if args[0] == "version" || args[0] == "--version" || args[0] == "-v" {
		p := version.Products["config"]
		fmt.Fprintf(stdout, "%s %s\nПлатформа: %s %s\nСтатус: %s\n", p.Name, p.Version, version.PlatformName, version.PlatformVersion, p.Status)
		return nil
	}
	cmd := norm(args)
	if !okcmd(cmd) {
		return usageError{"неизвестная команда ConfigDoctor: " + strings.Join(args, " ")}
	}
	target, opts, err := parse(args)
	if err != nil {
		return err
	}
	r, err := Analyze(cmd, target, opts)
	if err != nil {
		return err
	}
	return write(r, stdout, opts)
}
func help(w io.Writer) {
	p := version.Products["config"]
	fmt.Fprintf(w, `%s %s
Аудит Linux/Nginx/Docker/systemd/env/permissions конфигураций.

Использование:
  configdoctor <команда> [путь] [--json] [--output файл] [--recursive]

Команды:
  audit
  nginx audit
  docker audit
  systemd audit
  env audit
  permissions check
  version
`, p.Name, p.Version)
}
func norm(a []string) string {
	if len(a) >= 2 {
		x := a[0] + " " + a[1]
		switch x {
		case "nginx audit", "docker audit", "systemd audit", "env audit", "permissions check":
			return x
		}
	}
	return a[0]
}
func okcmd(c string) bool {
	switch c {
	case "audit", "check", "scan", "nginx audit", "docker audit", "systemd audit", "env audit", "permissions check":
		return true
	}
	return false
}
func parse(a []string) (string, Options, error) {
	target := "."
	o := Options{MaxFiles: 15000}
	skip := 1
	if len(a) >= 2 {
		x := a[0] + " " + a[1]
		if x == "nginx audit" || x == "docker audit" || x == "systemd audit" || x == "env audit" || x == "permissions check" {
			skip = 2
		}
	}
	for i := skip; i < len(a); i++ {
		v := a[i]
		switch {
		case v == "--json":
			o.JSON = true
		case v == "--recursive" || v == "-r":
			o.Recursive = true
		case v == "--output" || v == "-o":
			if i+1 >= len(a) {
				return target, o, usageError{"после " + v + " нужно указать файл"}
			}
			o.Output = a[i+1]
			i++
		case strings.HasPrefix(v, "--output="):
			o.Output = strings.TrimPrefix(v, "--output=")
		case strings.HasPrefix(v, "-"):
			return target, o, usageError{"неизвестный флаг: " + v}
		default:
			target = v
		}
	}
	return target, o, nil
}

func Analyze(cmd, target string, o Options) (Report, error) {
	started := time.Now().UTC()
	s, err := snapshotTarget(target, o)
	if err != nil {
		return Report{}, err
	}
	fs := []core.Finding{{ID: "config.profile.ready", Severity: core.SeverityInfo, Title: "Профиль ConfigDoctor готов", Message: "Выполнен аудит конфигураций.", Path: target, Tags: []string{"configdoctor", "config"}}}
	switch cmd {
	case "nginx audit":
		fs = append(fs, analyzeNginx(s)...)
	case "docker audit":
		fs = append(fs, analyzeDocker(s)...)
	case "systemd audit":
		fs = append(fs, analyzeSystemd(s)...)
	case "env audit":
		fs = append(fs, analyzeEnv(s)...)
	case "permissions check":
		fs = append(fs, analyzePermissions(s)...)
	default:
		fs = append(fs, analyzeNginx(s)...)
		fs = append(fs, analyzeDocker(s)...)
		fs = append(fs, analyzeSystemd(s)...)
		fs = append(fs, analyzeEnv(s)...)
		fs = append(fs, analyzePermissions(s)...)
	}
	fs = dedupe(fs)
	sortFindings(fs)
	return Report{Tool: "ConfigDoctor", Version: version.Products["config"].Version, Platform: version.PlatformName, Target: target, Command: cmd, StartedAt: started.Format(time.RFC3339), FinishedAt: time.Now().UTC().Format(time.RFC3339), Summary: core.Summarize(fs), Files: s.Files, Findings: fs, Artifacts: map[string]string{"engine": "ConfigDoctor analyzer", "engine_version": version.Products["config"].Version, "mode": "offline"}}, nil
}

func snapshotTarget(target string, o Options) (snapshot, error) {
	if o.MaxFiles <= 0 {
		o.MaxFiles = 15000
	}
	info, err := os.Stat(target)
	if err != nil {
		return snapshot{}, err
	}
	s := snapshot{Root: target, Text: map[string]string{}}
	if !info.IsDir() {
		if strings.HasSuffix(strings.ToLower(target), ".zip") {
			return snapshotZip(target, o)
		}
		kind := kindOf(target)
		s.Files = []FileInfo{{Path: target, Kind: kind, Mode: info.Mode().String()}}
		readText(&s, target)
		return s, nil
	}
	err = filepath.WalkDir(target, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if len(s.Files) >= o.MaxFiles {
			return filepath.SkipAll
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".git" || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			if !o.Recursive && p != target {
				return filepath.SkipDir
			}
			return nil
		}
		inf, _ := d.Info()
		rel, _ := filepath.Rel(target, p)
		fi := FileInfo{Path: rel, Kind: kindOf(rel)}
		if inf != nil {
			fi.Mode = inf.Mode().String()
		}
		s.Files = append(s.Files, fi)
		if fi.Kind != "other" {
			readTextAbs(&s, p, rel)
		}
		return nil
	})
	return s, err
}
func snapshotZip(path string, o Options) (snapshot, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return snapshot{}, err
	}
	defer zr.Close()
	s := snapshot{Root: path, Text: map[string]string{}}
	for i, f := range zr.File {
		if i >= o.MaxFiles {
			break
		}
		if f.FileInfo().IsDir() {
			continue
		}
		fi := FileInfo{Path: f.Name, Kind: kindOf(f.Name), Mode: f.Mode().String()}
		s.Files = append(s.Files, fi)
		if fi.Kind != "other" && f.UncompressedSize64 < 512*1024 {
			rc, err := f.Open()
			if err == nil {
				b, _ := io.ReadAll(io.LimitReader(rc, 512*1024))
				rc.Close()
				s.Text[f.Name] = string(b)
			}
		}
	}
	return s, nil
}
func kindOf(p string) string {
	n := strings.ToLower(filepath.Base(p))
	ext := strings.ToLower(filepath.Ext(p))
	switch {
	case n == "nginx.conf" || ext == ".conf":
		return "nginx"
	case n == "dockerfile" || strings.HasPrefix(n, "dockerfile.") || strings.Contains(n, "compose") && strings.HasSuffix(n, ".yml") || strings.HasSuffix(n, ".yaml"):
		return "docker"
	case ext == ".service" || ext == ".timer" || ext == ".socket":
		return "systemd"
	case strings.HasPrefix(n, ".env") || strings.Contains(n, "env.example"):
		return "env"
	case ext == ".sh" || ext == ".bash":
		return "shell"
	default:
		return "other"
	}
}
func readText(s *snapshot, p string) { readTextAbs(s, p, p) }
func readTextAbs(s *snapshot, abs, rel string) {
	b, err := os.ReadFile(abs)
	if err == nil && len(b) < 512*1024 {
		s.Text[rel] = string(b)
	}
}
func files(s snapshot, kind string) []FileInfo {
	out := []FileInfo{}
	for _, f := range s.Files {
		if f.Kind == kind {
			out = append(out, f)
		}
	}
	return out
}
func text(s snapshot, f FileInfo) string { return s.Text[f.Path] }
func hasLower(t, sub string) bool        { return strings.Contains(strings.ToLower(t), strings.ToLower(sub)) }

func analyzeNginx(s snapshot) []core.Finding {
	fs := []core.Finding{}
	ng := files(s, "nginx")
	if len(ng) == 0 {
		return []core.Finding{info("config.nginx.missing", "Nginx-конфиги не найдены", "Не обнаружены nginx.conf или .conf файлы.", s.Root, "nginx")}
	}
	for _, f := range ng {
		t := text(s, f)
		if !hasLower(t, "server_name") {
			fs = append(fs, warn("config.nginx.server_name.missing", "Нет server_name", "Для server-блока нужно явно задать server_name.", f.Path, "nginx"))
		}
		if hasLower(t, "proxy_pass") && !hasLower(t, "proxy_set_header host") {
			fs = append(fs, warn("config.nginx.proxy.host.missing", "Нет proxy_set_header Host", "Reverse proxy должен передавать исходный Host.", f.Path, "nginx", "proxy"))
		}
		if hasLower(t, "proxy_pass") && !hasLower(t, "x-forwarded-for") {
			fs = append(fs, warn("config.nginx.proxy.forwarded.missing", "Нет X-Forwarded-For", "Добавьте передачу X-Forwarded-For для корректных IP за proxy.", f.Path, "nginx", "proxy"))
		}
		if !hasLower(t, "access_log") || !hasLower(t, "error_log") {
			fs = append(fs, info("config.nginx.logs.partial", "Логи Nginx заданы не полностью", "Проверьте access_log и error_log для production-диагностики.", f.Path, "nginx", "logs"))
		}
		if hasLower(t, "listen 80") && !hasLower(t, "return 301 https") {
			fs = append(fs, info("config.nginx.https.redirect.missing", "HTTPS redirect не обнаружен", "Для публичного сервиса обычно нужен редирект HTTP → HTTPS.", f.Path, "nginx", "tls"))
		}
		if hasLower(t, "ssl_protocols") && hasLower(t, "tlsv1 ") {
			fs = append(fs, danger("config.nginx.tls.legacy", "Обнаружен устаревший TLS", "TLSv1/TLSv1.1 не должны использоваться в современной production-конфигурации.", f.Path, "nginx", "tls"))
		}
	}
	return fs
}
func analyzeDocker(s snapshot) []core.Finding {
	fs := []core.Finding{}
	ds := files(s, "docker")
	if len(ds) == 0 {
		return []core.Finding{info("config.docker.missing", "Docker-конфиги не найдены", "Не обнаружены Dockerfile или compose-файлы.", s.Root, "docker")}
	}
	hasIgnore := false
	for _, f := range s.Files {
		if filepath.Base(f.Path) == ".dockerignore" {
			hasIgnore = true
		}
	}
	for _, f := range ds {
		t := text(s, f)
		low := strings.ToLower(t)
		if strings.Contains(strings.ToLower(filepath.Base(f.Path)), "dockerfile") {
			if !hasIgnore {
				fs = append(fs, warn("config.dockerignore.missing", "Нет .dockerignore", "Docker build может случайно включить .env, .git, node_modules или временные файлы.", f.Path, "docker"))
			}
			if !hasLower(t, "healthcheck") {
				fs = append(fs, info("config.docker.healthcheck.missing", "Нет HEALTHCHECK", "Для production-контейнера полезен healthcheck.", f.Path, "docker"))
			}
			if !hasLower(t, "user ") {
				fs = append(fs, warn("config.docker.user.root", "Не задан USER", "Контейнер может запускаться от root.", f.Path, "docker", "security"))
			}
			if strings.Contains(low, "from ") && strings.Contains(low, ":latest") {
				fs = append(fs, warn("config.docker.latest", "Используется latest-tag", "Зафиксируйте версию base image вместо latest.", f.Path, "docker", "reproducibility"))
			}
			if hasLower(t, "copy . .") {
				fs = append(fs, info("config.docker.copy.all", "Широкий COPY . .", "Проверьте .dockerignore и состав build context.", f.Path, "docker"))
			}
		} else if strings.Contains(low, "privileged: true") {
			fs = append(fs, danger("config.compose.privileged", "Compose включает privileged", "Режим privileged требует явного обоснования.", f.Path, "docker", "security"))
		}
	}
	return fs
}
func analyzeSystemd(s snapshot) []core.Finding {
	ss := files(s, "systemd")
	if len(ss) == 0 {
		return []core.Finding{info("config.systemd.missing", "systemd unit-файлы не найдены", "Не обнаружены .service/.timer/.socket файлы.", s.Root, "systemd")}
	}
	fs := []core.Finding{}
	for _, f := range ss {
		t := text(s, f)
		if !hasLower(t, "restart=") {
			fs = append(fs, warn("config.systemd.restart.missing", "Нет Restart policy", "Для долгоживущего сервиса задайте Restart=on-failure или аналогичный режим.", f.Path, "systemd"))
		}
		if !hasLower(t, "user=") {
			fs = append(fs, warn("config.systemd.user.missing", "Нет User", "Сервис не должен по умолчанию запускаться от root без причины.", f.Path, "systemd", "security"))
		}
		if !hasLower(t, "workingdirectory=") {
			fs = append(fs, info("config.systemd.workdir.missing", "Нет WorkingDirectory", "Явный WorkingDirectory упрощает поддержку и диагностику.", f.Path, "systemd"))
		}
	}
	return fs
}
func analyzeEnv(s snapshot) []core.Finding {
	fs := []core.Finding{}
	hasExample := false
	for _, f := range files(s, "env") {
		base := strings.ToLower(filepath.Base(f.Path))
		if strings.Contains(base, "example") {
			hasExample = true
		}
		if base == ".env" {
			fs = append(fs, danger("config.env.real.present", "Обнаружен реальный .env", "Реальный .env не должен попадать в репозиторий или release-архив.", f.Path, "env", "secrets"))
		}
		t := text(s, f)
		if strings.Contains(t, "CHANGE_ME") || strings.Contains(t, "your-token") || strings.Contains(t, "example-token") {
			fs = append(fs, info("config.env.placeholders", "Найдены placeholder значения", "Проверьте, что placeholder значения задокументированы.", f.Path, "env"))
		}
		if looksSecret(t) && !strings.Contains(base, "example") {
			fs = append(fs, danger("config.env.secret.pattern", "Похожее на секрет значение", "Файл содержит token/key/password-like значения.", f.Path, "env", "secrets"))
		}
	}
	if !hasExample {
		fs = append(fs, warn("config.env.example.missing", "Нет .env.example", "Добавьте безопасный шаблон переменных окружения.", s.Root, "env"))
	}
	return fs
}
func analyzePermissions(s snapshot) []core.Finding {
	fs := []core.Finding{}
	for _, f := range s.Files {
		if strings.Contains(f.Mode, "rwxrwxrwx") || strings.HasSuffix(f.Mode, "-rwxrwxrwx") {
			fs = append(fs, danger("config.permissions.world_writable", "World-writable файл", "Права 777 недопустимы для production-конфигураций.", f.Path, "permissions", "security"))
		}
	}
	if len(fs) == 0 {
		fs = append(fs, info("config.permissions.checked", "Критичные права не обнаружены", "World-writable файлы в проверенной области не найдены.", s.Root, "permissions"))
	}
	return fs
}
func looksSecret(t string) bool {
	l := strings.ToLower(t)
	return strings.Contains(l, "token=") || strings.Contains(l, "password=") || strings.Contains(l, "secret=") || strings.Contains(l, "api_key=")
}
func warn(id, title, msg, path string, tags ...string) core.Finding {
	return core.Finding{ID: id, Severity: core.SeverityWarn, Title: title, Message: msg, Path: path, Tags: tags}
}
func danger(id, title, msg, path string, tags ...string) core.Finding {
	return core.Finding{ID: id, Severity: core.SeverityDanger, Title: title, Message: msg, Path: path, Tags: tags}
}
func info(id, title, msg, path string, tags ...string) core.Finding {
	return core.Finding{ID: id, Severity: core.SeverityInfo, Title: title, Message: msg, Path: path, Tags: tags}
}
func dedupe(in []core.Finding) []core.Finding {
	seen := map[string]bool{}
	out := []core.Finding{}
	for _, f := range in {
		k := f.ID + "|" + f.Path
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, f)
	}
	return out
}
func sortFindings(fs []core.Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		if fs[i].Severity == fs[j].Severity {
			return fs[i].ID < fs[j].ID
		}
		return rank(fs[i].Severity) < rank(fs[j].Severity)
	})
}
func rank(s core.Severity) int {
	switch s {
	case core.SeverityCritical:
		return 0
	case core.SeverityDanger:
		return 1
	case core.SeverityWarn:
		return 2
	default:
		return 3
	}
}
func write(r Report, w io.Writer, o Options) error {
	if o.JSON || o.Output != "" {
		return core.WriteJSONFileOrStdout(r, w, o.Output)
	}
	fmt.Fprintf(w, "%s %s: %s, файлов: %d\n", r.Tool, r.Version, r.Command, len(r.Files))
	fmt.Fprintf(w, "Итог: critical=%d, danger=%d, warn=%d, info=%d\n", r.Summary["critical"], r.Summary["danger"], r.Summary["warn"], r.Summary["info"])
	for _, f := range r.Findings {
		fmt.Fprintf(w, "[%s] %s — %s", f.Severity, f.Title, f.Message)
		if f.Path != "" {
			fmt.Fprintf(w, " (%s)", f.Path)
		}
		fmt.Fprintln(w)
	}
	return nil
}
