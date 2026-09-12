package deploydoctor

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
	Kind string `json:"kind,omitempty"`
	Size int64  `json:"size,omitempty"`
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
	Root      string
	Files     []FileInfo
	Text      map[string]string
	IsArchive bool
}
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		help(stdout)
		return nil
	}
	if args[0] == "version" || args[0] == "--version" || args[0] == "-v" {
		p := version.Products["deploy"]
		fmt.Fprintf(stdout, "%s %s\nПлатформа: %s %s\nСтатус: %s\n", p.Name, p.Version, version.PlatformName, version.PlatformVersion, p.Status)
		return nil
	}
	cmd := norm(args)
	if !okcmd(cmd) {
		return usageError{"неизвестная команда DeployDoctor: " + strings.Join(args, " ")}
	}
	target, o, err := parse(args)
	if err != nil {
		return err
	}
	r, err := Analyze(cmd, target, o)
	if err != nil {
		return err
	}
	return write(r, stdout, o)
}
func help(w io.Writer) {
	p := version.Products["deploy"]
	fmt.Fprintf(w, `%s %s
Pre-deploy проверки проекта и release-архивов.

Использование:
  deploydoctor <команда> [путь] [--json] [--output файл] [--recursive]

Команды:
  check
  release check
  ci check
  env check
  docker check
  archive check
  version
`, p.Name, p.Version)
}
func norm(a []string) string {
	if len(a) >= 2 {
		x := a[0] + " " + a[1]
		switch x {
		case "release check", "ci check", "env check", "docker check", "archive check":
			return x
		}
	}
	return a[0]
}
func okcmd(c string) bool {
	switch c {
	case "check", "scan", "audit", "release check", "ci check", "env check", "docker check", "archive check":
		return true
	}
	return false
}
func parse(a []string) (string, Options, error) {
	target := "."
	o := Options{MaxFiles: 20000}
	skip := 1
	if len(a) >= 2 {
		x := a[0] + " " + a[1]
		if x == "release check" || x == "ci check" || x == "env check" || x == "docker check" || x == "archive check" {
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
	fs := []core.Finding{{ID: "deploy.profile.ready", Severity: core.SeverityInfo, Title: "Профиль DeployDoctor готов", Message: "Выполнена pre-deploy диагностика.", Path: target, Tags: []string{"deploydoctor", "deploy"}}}
	switch cmd {
	case "release check":
		fs = append(fs, analyzeRelease(s)...)
	case "ci check":
		fs = append(fs, analyzeCI(s)...)
	case "env check":
		fs = append(fs, analyzeEnv(s)...)
	case "docker check":
		fs = append(fs, analyzeDocker(s)...)
	case "archive check":
		fs = append(fs, analyzeArchive(s)...)
	default:
		fs = append(fs, analyzeRelease(s)...)
		fs = append(fs, analyzeCI(s)...)
		fs = append(fs, analyzeEnv(s)...)
		fs = append(fs, analyzeDocker(s)...)
		if s.IsArchive {
			fs = append(fs, analyzeArchive(s)...)
		}
	}
	fs = dedupe(fs)
	sortFindings(fs)
	return Report{Tool: "DeployDoctor", Version: version.Products["deploy"].Version, Platform: version.PlatformName, Target: target, Command: cmd, StartedAt: started.Format(time.RFC3339), FinishedAt: time.Now().UTC().Format(time.RFC3339), Summary: core.Summarize(fs), Files: s.Files, Findings: fs, Artifacts: map[string]string{"engine": "DeployDoctor analyzer", "engine_version": version.Products["deploy"].Version, "mode": "offline"}}, nil
}
func snapshotTarget(target string, o Options) (snapshot, error) {
	if o.MaxFiles <= 0 {
		o.MaxFiles = 20000
	}
	info, err := os.Stat(target)
	if err != nil {
		return snapshot{}, err
	}
	if !info.IsDir() {
		if strings.HasSuffix(strings.ToLower(target), ".zip") {
			return snapshotZip(target, o)
		}
		s := snapshot{Root: target, Text: map[string]string{}}
		s.Files = []FileInfo{{Path: target, Kind: kindOf(target), Size: info.Size()}}
		readText(&s, target, target)
		return s, nil
	}
	s := snapshot{Root: target, Text: map[string]string{}}
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
			fi.Size = inf.Size()
		}
		s.Files = append(s.Files, fi)
		if fi.Size < 512*1024 {
			readText(&s, p, rel)
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
	s := snapshot{Root: path, Text: map[string]string{}, IsArchive: true}
	for i, f := range zr.File {
		if i >= o.MaxFiles {
			break
		}
		if f.FileInfo().IsDir() {
			continue
		}
		fi := FileInfo{Path: f.Name, Kind: kindOf(f.Name), Size: int64(f.UncompressedSize64)}
		s.Files = append(s.Files, fi)
		if f.UncompressedSize64 < 512*1024 {
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
func readText(s *snapshot, abs, rel string) {
	b, err := os.ReadFile(abs)
	if err == nil && len(b) < 512*1024 {
		s.Text[rel] = string(b)
	}
}
func kindOf(p string) string {
	b := strings.ToLower(filepath.Base(p))
	switch {
	case b == "readme.md" || b == "changelog.md" || b == "license" || b == "security.md" || b == "release_checklist.md":
		return "release-doc"
	case b == "makefile" || strings.Contains(b, "ci.yml") || strings.Contains(b, "ci.yaml"):
		return "ci"
	case b == ".env" || b == ".env.example" || strings.Contains(b, "env.example"):
		return "env"
	case b == "dockerfile" || strings.HasPrefix(b, "dockerfile.") || strings.Contains(b, "compose") && strings.HasSuffix(b, ".yml") || strings.HasSuffix(b, ".yaml"):
		return "docker"
	case strings.Contains(b, "lock") || b == "go.sum":
		return "lock"
	default:
		return "other"
	}
}
func has(s snapshot, names ...string) bool {
	for _, f := range s.Files {
		b := strings.ToLower(filepath.Base(f.Path))
		for _, n := range names {
			if b == strings.ToLower(n) {
				return true
			}
		}
	}
	return false
}
func hasKind(s snapshot, k string) bool {
	for _, f := range s.Files {
		if f.Kind == k {
			return true
		}
	}
	return false
}
func texts(s snapshot, kind string) map[string]string {
	m := map[string]string{}
	for _, f := range s.Files {
		if f.Kind == kind {
			m[f.Path] = s.Text[f.Path]
		}
	}
	return m
}
func analyzeRelease(s snapshot) []core.Finding {
	fs := []core.Finding{}
	for _, name := range []string{"README.md", "CHANGELOG.md", "LICENSE", "SECURITY.md"} {
		if !has(s, name) {
			fs = append(fs, warn("deploy.release."+strings.ToLower(strings.TrimSuffix(name, ".md"))+".missing", "Нет "+name, "Для product-релиза нужен файл "+name+".", s.Root, "release"))
		}
	}
	if !has(s, "RELEASE_CHECKLIST.md") {
		fs = append(fs, info("deploy.release.checklist.missing", "Нет RELEASE_CHECKLIST.md", "Release checklist снижает риск неполного архива.", s.Root, "release"))
	}
	return fs
}
func analyzeCI(s snapshot) []core.Finding {
	fs := []core.Finding{}
	if !hasKind(s, "ci") {
		fs = append(fs, warn("deploy.ci.missing", "Не найдена CI/Makefile автоматизация", "Добавьте Makefile или CI со сборкой, тестами и smoke-проверкой.", s.Root, "ci"))
	}
	return fs
}
func analyzeEnv(s snapshot) []core.Finding {
	fs := []core.Finding{}
	if !has(s, ".env.example") {
		fs = append(fs, warn("deploy.env.example.missing", "Нет .env.example", "Релиз должен содержать безопасный шаблон переменных окружения.", s.Root, "env"))
	}
	if has(s, ".env") {
		fs = append(fs, danger("deploy.env.real.present", "Обнаружен .env", "Реальный .env не должен попадать в релизный архив.", s.Root, "env", "secrets"))
	}
	return fs
}
func analyzeDocker(s snapshot) []core.Finding {
	fs := []core.Finding{}
	if !hasKind(s, "docker") {
		return []core.Finding{info("deploy.docker.missing", "Docker-конфиг не найден", "Docker не обязателен, но для воспроизводимого production-деплоя полезен Dockerfile/compose.", s.Root, "docker")}
	}
	if !has(s, ".dockerignore") {
		fs = append(fs, warn("deploy.dockerignore.missing", "Нет .dockerignore", "Release build может включить лишние файлы.", s.Root, "docker"))
	}
	for p, t := range texts(s, "docker") {
		low := strings.ToLower(t)
		if strings.Contains(low, ":latest") {
			fs = append(fs, warn("deploy.docker.latest", "Используется latest-tag", "Зафиксируйте версию base image.", p, "docker"))
		}
		if !strings.Contains(low, "healthcheck") && strings.Contains(strings.ToLower(filepath.Base(p)), "dockerfile") {
			fs = append(fs, info("deploy.docker.healthcheck.missing", "Нет HEALTHCHECK", "Добавьте healthcheck для production-контейнера.", p, "docker"))
		}
	}
	return fs
}
func analyzeArchive(s snapshot) []core.Finding {
	fs := []core.Finding{}
	if !s.IsArchive {
		fs = append(fs, info("deploy.archive.not_zip", "Цель не ZIP-архив", "archive check эффективнее использовать с .zip архивом.", s.Root, "archive"))
		return fs
	}
	if !has(s, "README.md") {
		fs = append(fs, warn("deploy.archive.readme.missing", "В архиве нет README.md", "Клиентский архив должен содержать инструкцию запуска.", s.Root, "archive"))
	}
	if !has(s, ".env.example") {
		fs = append(fs, warn("deploy.archive.env.example.missing", "В архиве нет .env.example", "Добавьте шаблон окружения без секретов.", s.Root, "archive"))
	}
	for _, f := range s.Files {
		p := strings.ToLower(f.Path)
		switch {
		case strings.Contains(p, "/.git/") || strings.HasPrefix(p, ".git/"):
			fs = append(fs, danger("deploy.archive.git.present", "В архив попал .git", "Удалите VCS-историю из клиентского архива.", f.Path, "archive", "secrets"))
		case strings.Contains(p, "node_modules/"):
			fs = append(fs, warn("deploy.archive.node_modules.present", "В архив попал node_modules", "Обычно зависимости должны устанавливаться package manager-ом.", f.Path, "archive"))
		case strings.Contains(p, "vendor/"):
			fs = append(fs, info("deploy.archive.vendor.present", "В архив попал vendor", "Проверьте, требуется ли поставлять vendor в этом релизе.", f.Path, "archive"))
		case filepath.Base(p) == ".env":
			fs = append(fs, danger("deploy.archive.env.present", "В архив попал .env", "Удалите реальные секреты из архива.", f.Path, "archive", "secrets"))
		case strings.HasSuffix(p, "~") || strings.HasSuffix(p, ".tmp") || strings.HasSuffix(p, ".bak"):
			fs = append(fs, warn("deploy.archive.temp.present", "В архиве временный файл", "Удалите временные/backup-файлы из релиза.", f.Path, "archive"))
		}
	}
	return fs
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
