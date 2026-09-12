package depdoctor

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
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
type Manifest struct {
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	LockFile string `json:"lock_file,omitempty"`
}
type Finding struct {
	ID       string        `json:"id"`
	Severity core.Severity `json:"severity"`
	Title    string        `json:"title"`
	Message  string        `json:"message"`
	Path     string        `json:"path"`
	Tags     []string      `json:"tags,omitempty"`
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
	Manifests  []Manifest        `json:"manifests"`
	Findings   []Finding         `json:"findings"`
	Artifacts  map[string]string `json:"artifacts"`
}

type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		help(stdout)
		return nil
	}
	if args[0] == "version" || args[0] == "--version" || args[0] == "-v" {
		p := version.Products["dep"]
		fmt.Fprintf(stdout, "%s %s\nПлатформа: %s %s\nСтатус: %s\n", p.Name, p.Version, version.PlatformName, version.PlatformVersion, p.Status)
		return nil
	}
	cmd := norm(args)
	if !okcmd(cmd) {
		return usageError{"неизвестная команда DepDoctor: " + strings.Join(args, " ")}
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
	p := version.Products["dep"]
	fmt.Fprintf(w, `%s %s
Offline-аудит зависимостей и supply-chain рисков.

Использование:
  depdoctor <команда> [путь] [--json] [--output файл] [--recursive]

Команды:
  audit
  lock check
  scripts audit
  licenses
  outdated
  supply-chain scan
  version
`, p.Name, p.Version)
}
func norm(a []string) string {
	if len(a) >= 2 {
		x := a[0] + " " + a[1]
		if x == "lock check" || x == "scripts audit" || x == "supply-chain scan" {
			return x
		}
	}
	return a[0]
}
func okcmd(c string) bool {
	switch c {
	case "audit", "check", "lock check", "scripts audit", "licenses", "outdated", "supply-chain scan", "scan":
		return true
	}
	return false
}
func parse(a []string) (string, Options, error) {
	t := "."
	o := Options{MaxFiles: 5000}
	skip := 1
	if len(a) >= 2 {
		x := a[0] + " " + a[1]
		if x == "lock check" || x == "scripts audit" || x == "supply-chain scan" {
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
				return t, o, usageError{"после " + v + " нужно указать файл"}
			}
			o.Output = a[i+1]
			i++
		case strings.HasPrefix(v, "--output="):
			o.Output = strings.TrimPrefix(v, "--output=")
		case strings.HasPrefix(v, "-"):
			return t, o, usageError{"неизвестный флаг: " + v}
		default:
			t = v
		}
	}
	return t, o, nil
}

func Analyze(cmd, target string, o Options) (Report, error) {
	st := time.Now().UTC()
	ms, err := discover(target, o)
	if err != nil {
		return Report{}, err
	}
	fs := []Finding{}
	if len(ms) == 0 {
		fs = append(fs, info("dep.manifest.missing", "Манифесты зависимостей не найдены", "DepDoctor не нашёл package.json, composer.json, go.mod, requirements.txt, pyproject.toml, Cargo.toml, pom.xml или build.gradle.", target, "dependencies"))
	}
	for _, m := range ms {
		switch cmd {
		case "lock check", "check":
			fs = append(fs, lock(m)...)
		case "scripts audit":
			fs = append(fs, scripts(m)...)
		case "licenses":
			fs = append(fs, licenses(m)...)
		case "outdated":
			fs = append(fs, info("dep.outdated.offline", "Outdated-аудит требует online-режима", "Версия 1.0.1 выполняет offline-аудит. Проверка актуальности версий будет добавлена отдельным online-режимом.", m.Path, "outdated"))
		case "supply-chain scan", "scan":
			fs = append(fs, supply(m)...)
		default:
			fs = append(fs, lock(m)...)
			fs = append(fs, versions(m)...)
			fs = append(fs, scripts(m)...)
			fs = append(fs, licenses(m)...)
			fs = append(fs, supply(m)...)
		}
	}
	fs = dedupe(fs)
	sort.SliceStable(fs, func(i, j int) bool {
		if fs[i].Severity == fs[j].Severity {
			return fs[i].ID < fs[j].ID
		}
		return rank(fs[i].Severity) < rank(fs[j].Severity)
	})
	return Report{Tool: "DepDoctor", Version: version.Products["dep"].Version, Platform: version.PlatformName, Target: target, Command: cmd, StartedAt: st.Format(time.RFC3339), FinishedAt: time.Now().UTC().Format(time.RFC3339), Summary: sum(fs), Manifests: ms, Findings: fs, Artifacts: map[string]string{"engine": "DepDoctor offline analyzer", "engine_version": version.Products["dep"].Version, "online_mode": "disabled"}}, nil
}

func discover(target string, o Options) ([]Manifest, error) {
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}
	names := map[string]string{"package.json": "nodejs", "composer.json": "php", "go.mod": "go", "requirements.txt": "python-requirements", "pyproject.toml": "python-pyproject", "Cargo.toml": "rust", "pom.xml": "java-maven", "build.gradle": "java-gradle"}
	out := []Manifest{}
	add := func(p, k string) { out = append(out, Manifest{Path: p, Kind: k, LockFile: findLock(p, k)}) }
	if !info.IsDir() {
		if k, ok := names[filepath.Base(target)]; ok {
			add(target, k)
		}
		return out, nil
	}
	if !o.Recursive {
		for n, k := range names {
			p := filepath.Join(target, n)
			if exists(p) {
				add(p, k)
			}
		}
		return sorted(out), nil
	}
	count := 0
	filepath.WalkDir(target, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return nil
		}
		if d.IsDir() {
			b := d.Name()
			if b == ".git" || b == "node_modules" || b == "vendor" || b == "dist" || b == "build" || b == "target" {
				return filepath.SkipDir
			}
			return nil
		}
		if count >= o.MaxFiles {
			return nil
		}
		count++
		if k, ok := names[d.Name()]; ok {
			add(p, k)
		}
		return nil
	})
	return sorted(out), nil
}
func findLock(p, k string) string {
	d := filepath.Dir(p)
	c := map[string][]string{"nodejs": {"package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lockb"}, "php": {"composer.lock"}, "go": {"go.sum"}, "python-requirements": {"requirements.lock"}, "python-pyproject": {"poetry.lock", "uv.lock"}, "rust": {"Cargo.lock"}, "java-gradle": {"gradle.lockfile"}}
	for _, n := range c[k] {
		x := filepath.Join(d, n)
		if exists(x) {
			return x
		}
	}
	return ""
}
func lock(m Manifest) []Finding {
	if m.LockFile != "" {
		return []Finding{info("dep.lock.present", "Lock-файл найден", "Для "+m.Kind+" найден lock-файл: "+filepath.Base(m.LockFile)+".", m.LockFile, "lock", m.Kind)}
	}
	sev := core.SeverityWarn
	if m.Kind == "nodejs" || m.Kind == "php" || m.Kind == "rust" || m.Kind == "python-pyproject" {
		sev = core.SeverityDanger
	}
	return []Finding{{ID: "dep.lock.missing", Severity: sev, Title: "Lock-файл не найден", Message: "Зафиксируйте зависимости для воспроизводимой сборки и контролируемого релиза.", Path: m.Path, Tags: []string{"lock", m.Kind}}}
}
func versions(m Manifest) []Finding {
	txt := read(m.Path)
	fs := []Finding{}
	if m.Kind == "nodejs" || m.Kind == "php" {
		var doc map[string]any
		if json.Unmarshal([]byte(txt), &doc) != nil {
			return []Finding{warn("dep.manifest.invalid", "Манифест не разобран", "JSON-файл зависимостей содержит синтаксическую ошибку.", m.Path, m.Kind)}
		}
		sections := []string{"dependencies", "devDependencies", "peerDependencies", "optionalDependencies", "require", "require-dev"}
		for _, sec := range sections {
			if deps, ok := doc[sec].(map[string]any); ok {
				for name, raw := range deps {
					fs = append(fs, ver(m.Path, m.Kind, name, fmt.Sprint(raw))...)
				}
			}
		}
		if m.Kind == "nodejs" {
			if _, ok := doc["engines"]; !ok {
				fs = append(fs, info("dep.node.engine.missing", "Не указан Node.js engine", "Добавьте engines.node, чтобы зафиксировать ожидаемый runtime проекта.", m.Path, "nodejs", "runtime"))
			}
		}
	}
	if m.Kind == "python-requirements" {
		sc := bufio.NewScanner(strings.NewReader(txt))
		line := 0
		for sc.Scan() {
			line++
			x := strings.TrimSpace(sc.Text())
			if x == "" || strings.HasPrefix(x, "#") || strings.HasPrefix(x, "-") {
				continue
			}
			if strings.Contains(x, "git+") || strings.Contains(x, "http://") || strings.Contains(x, "https://") {
				fs = append(fs, warn("dep.python.external.source", "Python-зависимость из внешнего источника", "Git/HTTP зависимости требуют закреплённой ревизии и ручной проверки.", fmt.Sprintf("%s:%d", m.Path, line), "python", "supply-chain"))
			}
			if !strings.Contains(x, "==") && !strings.Contains(x, "===") {
				fs = append(fs, warn("dep.python.unpinned", "Python-зависимость не закреплена", "В requirements.txt используйте строгую фиксацию версий через == для production-сборок.", fmt.Sprintf("%s:%d", m.Path, line), "python", "version"))
			}
		}
	}
	lower := strings.ToLower(txt)
	if strings.Contains(lower, "replace ") && m.Kind == "go" {
		fs = append(fs, warn("dep.go.replace.local", "Go replace требует проверки", "Локальные replace-директивы могут сломать воспроизводимую сборку.", m.Path, "go", "replace"))
	}
	if strings.Contains(lower, "latest") || strings.Contains(lower, "snapshot") || strings.Contains(lower, "nightly") {
		fs = append(fs, warn("dep.source.moving_target", "Moving target версия", "latest/snapshot/nightly ухудшают воспроизводимость supply-chain.", m.Path, m.Kind, "supply-chain"))
	}
	return fs
}
func ver(path, eco, name, v string) []Finding {
	r := []Finding{}
	x := strings.TrimSpace(v)
	lx := strings.ToLower(x)
	if x == "" || x == "*" || lx == "latest" || x == "x" || x == "X" {
		r = append(r, warn("dep.version.dynamic", "Динамическая версия зависимости", fmt.Sprintf("%s использует версию %q. Закрепите версию явно.", name, x), path, eco, "version"))
	}
	if strings.Contains(lx, "git+") || strings.Contains(lx, "github.com") || strings.Contains(lx, "http://") || strings.Contains(lx, "https://") {
		r = append(r, warn("dep.source.external", "Зависимость из внешнего источника", name+" подключается из Git/HTTP источника. Проверьте закрепление ревизии.", path, eco, "supply-chain"))
	}
	if strings.HasPrefix(x, ">=") || strings.Contains(x, " || ") {
		r = append(r, info("dep.version.range.wide", "Широкий диапазон версии", name+" использует широкий диапазон. Для production-релиза проверьте lock-файл.", path, eco, "version"))
	}
	if strings.Contains(lx, "alpha") || strings.Contains(lx, "beta") || strings.Contains(lx, "rc") || strings.Contains(lx, "dev") {
		r = append(r, warn("dep.version.prerelease", "Pre-release версия зависимости", name+" использует pre-release/dev версию.", path, eco, "version"))
	}
	return r
}
func scripts(m Manifest) []Finding {
	txt := read(m.Path)
	low := strings.ToLower(txt)
	fs := []Finding{}
	if m.Kind == "nodejs" || m.Kind == "php" {
		var doc map[string]any
		if json.Unmarshal([]byte(txt), &doc) == nil {
			if ss, ok := doc["scripts"].(map[string]any); ok {
				for n, raw := range ss {
					v := strings.ToLower(fmt.Sprint(raw))
					if n == "postinstall" || n == "preinstall" || strings.Contains(v, "curl ") || strings.Contains(v, "wget ") || strings.Contains(v, "| sh") || strings.Contains(v, "powershell") {
						fs = append(fs, danger("dep.script.suspicious", "Подозрительный install/script hook", fmt.Sprintf("Скрипт %s требует ручной проверки.", n), m.Path, m.Kind, "scripts", "supply-chain"))
					}
				}
			}
		}
	}
	if strings.Contains(low, "curl ") || strings.Contains(low, "wget ") || strings.Contains(low, "| sh") || strings.Contains(low, "base64 -d") || strings.Contains(low, "chmod +x") {
		fs = append(fs, warn("dep.script.shell.pattern", "Shell-паттерн в файле зависимостей", "Обнаружены curl/wget/pipe/chmod/base64 паттерны, требующие проверки.", m.Path, m.Kind, "scripts"))
	}
	return fs
}
func licenses(m Manifest) []Finding {
	if m.Kind != "nodejs" && m.Kind != "php" && m.Kind != "rust" {
		return nil
	}
	txt := read(m.Path)
	if m.Kind == "rust" {
		if !strings.Contains(txt, "license") {
			return []Finding{info("dep.license.missing", "Лицензия проекта не указана", "В Cargo.toml не найдено поле license.", m.Path, "rust", "license")}
		}
		return nil
	}
	var doc map[string]any
	if json.Unmarshal([]byte(txt), &doc) == nil {
		if _, ok := doc["license"]; !ok {
			return []Finding{info("dep.license.missing", "Лицензия проекта не указана", "В манифесте не найдено поле license.", m.Path, m.Kind, "license")}
		}
	}
	return nil
}
func supply(m Manifest) []Finding {
	txt := strings.ToLower(read(m.Path))
	fs := []Finding{}
	if strings.Contains(txt, "http://") {
		fs = append(fs, danger("dep.source.insecure_http", "Небезопасный HTTP-источник", "Зависимости или скрипты используют http://.", m.Path, m.Kind, "supply-chain"))
	}
	if strings.Contains(txt, "github.com/") || strings.Contains(txt, "gitlab.com/") || strings.Contains(txt, "git+") {
		fs = append(fs, warn("dep.source.git", "Git-зависимость требует проверки", "Git-зависимости должны быть закреплены commit/tag и проверены вручную.", m.Path, m.Kind, "supply-chain"))
	}
	fs = append(fs, versions(m)...)
	fs = append(fs, scripts(m)...)
	return fs
}
func write(r Report, w io.Writer, o Options) error {
	if o.JSON || o.Output != "" {
		b, e := json.MarshalIndent(r, "", "  ")
		if e != nil {
			return e
		}
		b = append(b, '\n')
		if o.Output == "" {
			_, e = w.Write(b)
			return e
		}
		d := filepath.Dir(o.Output)
		if d != "." && d != "" {
			if e = os.MkdirAll(d, 0755); e != nil {
				return e
			}
		}
		if e = os.WriteFile(o.Output, b, 0644); e != nil {
			return e
		}
		fmt.Fprintf(w, "Отчёт сохранён: %s\n", o.Output)
		return nil
	}
	fmt.Fprintf(w, "%s %s: %s, цель %s\n", r.Tool, r.Version, r.Command, r.Target)
	fmt.Fprintf(w, "Итог: critical=%d, danger=%d, warn=%d, info=%d\n", r.Summary["critical"], r.Summary["danger"], r.Summary["warn"], r.Summary["info"])
	for _, m := range r.Manifests {
		lock := "lock-файл не найден"
		if m.LockFile != "" {
			lock = "lock-файл: " + filepath.Base(m.LockFile)
		}
		fmt.Fprintf(w, "  - %s (%s, %s)\n", m.Path, m.Kind, lock)
	}
	for _, f := range r.Findings {
		fmt.Fprintf(w, "[%s] %s — %s", f.Severity, f.Title, f.Message)
		if strings.TrimSpace(f.Path) != "" {
			fmt.Fprintf(w, " (%s)", f.Path)
		}
		fmt.Fprintln(w)
	}
	return nil
}
func exists(p string) bool { st, e := os.Stat(p); return e == nil && !st.IsDir() }
func read(p string) string { b, _ := os.ReadFile(p); return string(b) }
func sorted(x []Manifest) []Manifest {
	sort.Slice(x, func(i, j int) bool { return x[i].Path < x[j].Path })
	return x
}
func dedupe(x []Finding) []Finding {
	s := map[string]bool{}
	r := []Finding{}
	for _, f := range x {
		k := f.ID + "|" + f.Path + "|" + f.Message
		if !s[k] {
			s[k] = true
			r = append(r, f)
		}
	}
	return r
}
func sum(x []Finding) map[string]int {
	m := map[string]int{"critical": 0, "danger": 0, "warn": 0, "info": 0}
	for _, f := range x {
		m[string(f.Severity)]++
	}
	return m
}
func rank(s core.Severity) int {
	if s == core.SeverityCritical {
		return 0
	}
	if s == core.SeverityDanger {
		return 1
	}
	if s == core.SeverityWarn {
		return 2
	}
	return 3
}
func info(id, t, m, p string, tags ...string) Finding {
	return Finding{id, core.SeverityInfo, t, m, p, tags}
}
func warn(id, t, m, p string, tags ...string) Finding {
	return Finding{id, core.SeverityWarn, t, m, p, tags}
}
func danger(id, t, m, p string, tags ...string) Finding {
	return Finding{id, core.SeverityDanger, t, m, p, tags}
}
