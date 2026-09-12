package stackdoctor

import (
	"archive/zip"
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

type StackKind string

const (
	StackNode   StackKind = "nodejs"
	StackPHP    StackKind = "php"
	StackGo     StackKind = "go"
	StackPython StackKind = "python"
	StackRust   StackKind = "rust"
	StackJava   StackKind = "java"
	StackDocker StackKind = "docker"
)

type Component struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
	Role string `json:"role"`
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
	Components []Component       `json:"components"`
	Findings   []Finding         `json:"findings"`
	Artifacts  map[string]string `json:"artifacts"`
}

type snapshot struct {
	Root    string
	Files   []string
	Dirs    map[string]bool
	FileSet map[string]bool
	Text    map[string]string
}

type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		help(stdout)
		return nil
	}
	if args[0] == "version" || args[0] == "--version" || args[0] == "-v" {
		p := version.Products["stack"]
		fmt.Fprintf(stdout, "%s %s\nПлатформа: %s %s\nСтатус: %s\n", p.Name, p.Version, version.PlatformName, version.PlatformVersion, p.Status)
		return nil
	}
	cmd := norm(args)
	if !okcmd(cmd) {
		return usageError{"неизвестная команда StackDoctor: " + strings.Join(args, " ")}
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
	p := version.Products["stack"]
	fmt.Fprintf(w, `%s %s
Диагностика full-stack проектов: структура, frontend/backend, окружение, зависимости, API, Docker и production-readiness.

Использование:
  stackdoctor <команда> [путь] [--json] [--output файл] [--recursive]

Команды:
  scan
  structure check
  frontend scan
  backend scan
  env audit
  production check
  version
`, p.Name, p.Version)
}

func norm(a []string) string {
	if len(a) >= 2 {
		x := a[0] + " " + a[1]
		switch x {
		case "structure check", "frontend scan", "backend scan", "env audit", "production check":
			return x
		}
	}
	return a[0]
}
func okcmd(c string) bool {
	switch c {
	case "scan", "check", "audit", "structure check", "frontend scan", "backend scan", "env audit", "production check":
		return true
	}
	return false
}
func parse(a []string) (string, Options, error) {
	target := "."
	opts := Options{MaxFiles: 12000}
	skip := 1
	if len(a) >= 2 {
		x := a[0] + " " + a[1]
		if x == "structure check" || x == "frontend scan" || x == "backend scan" || x == "env audit" || x == "production check" {
			skip = 2
		}
	}
	for i := skip; i < len(a); i++ {
		v := a[i]
		switch {
		case v == "--json":
			opts.JSON = true
		case v == "--recursive" || v == "-r":
			opts.Recursive = true
		case v == "--output" || v == "-o":
			if i+1 >= len(a) {
				return target, opts, usageError{"после " + v + " нужно указать файл"}
			}
			opts.Output = a[i+1]
			i++
		case strings.HasPrefix(v, "--output="):
			opts.Output = strings.TrimPrefix(v, "--output=")
		case strings.HasPrefix(v, "-"):
			return target, opts, usageError{"неизвестный флаг: " + v}
		default:
			target = v
		}
	}
	return target, opts, nil
}

func Analyze(cmd, target string, o Options) (Report, error) {
	started := time.Now().UTC()
	s, err := takeSnapshot(target, o)
	if err != nil {
		return Report{}, err
	}
	comps := detectComponents(s)
	fs := []Finding{{ID: "stack.profile.ready", Severity: core.SeverityInfo, Title: "Профиль StackDoctor готов", Message: "Выполнена диагностика full-stack проекта.", Path: target, Tags: []string{"stackdoctor", "fullstack"}}}
	switch cmd {
	case "structure check", "check":
		fs = append(fs, analyzeStructure(s, comps)...)
	case "frontend scan":
		fs = append(fs, analyzeFrontend(s, comps)...)
	case "backend scan":
		fs = append(fs, analyzeBackend(s, comps)...)
	case "env audit":
		fs = append(fs, analyzeEnv(s)...)
	case "production check":
		fs = append(fs, analyzeProduction(s, comps)...)
	default:
		fs = append(fs, analyzeStructure(s, comps)...)
		fs = append(fs, analyzeFrontend(s, comps)...)
		fs = append(fs, analyzeBackend(s, comps)...)
		fs = append(fs, analyzeEnv(s)...)
		fs = append(fs, analyzeDependencies(s, comps)...)
		fs = append(fs, analyzeAPI(s)...)
		fs = append(fs, analyzeDocker(s)...)
		fs = append(fs, analyzeProduction(s, comps)...)
	}
	fs = dedupe(fs)
	sort.SliceStable(fs, func(i, j int) bool {
		if fs[i].Severity == fs[j].Severity {
			return fs[i].ID < fs[j].ID
		}
		return rank(fs[i].Severity) < rank(fs[j].Severity)
	})
	return Report{Tool: "StackDoctor", Version: version.Products["stack"].Version, Platform: version.PlatformName, Target: target, Command: cmd, StartedAt: started.Format(time.RFC3339), FinishedAt: time.Now().UTC().Format(time.RFC3339), Summary: sum(fs), Components: comps, Findings: fs, Artifacts: map[string]string{"engine": "StackDoctor full-stack analyzer", "engine_version": version.Products["stack"].Version, "mode": "offline"}}, nil
}

func takeSnapshot(target string, o Options) (snapshot, error) {
	if o.MaxFiles <= 0 {
		o.MaxFiles = 12000
	}
	info, err := os.Stat(target)
	if err != nil {
		return snapshot{}, err
	}
	if !info.IsDir() {
		if strings.HasSuffix(strings.ToLower(target), ".zip") {
			return snapshotZip(target, o)
		}
		return snapshot{}, fmt.Errorf("StackDoctor ожидает директорию проекта или ZIP-архив")
	}
	s := snapshot{Root: target, Dirs: map[string]bool{}, FileSet: map[string]bool{}, Text: map[string]string{}}
	count := 0
	err = filepath.WalkDir(target, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return nil
		}
		rel, _ := filepath.Rel(target, p)
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if shouldSkipDir(name) {
				return filepath.SkipDir
			}
			s.Dirs[rel] = true
			return nil
		}
		if count >= o.MaxFiles {
			return nil
		}
		count++
		s.Files = append(s.Files, rel)
		s.FileSet[rel] = true
		if shouldRead(rel) {
			if b, err := os.ReadFile(p); err == nil {
				s.Text[rel] = limit(string(b), 65536)
			}
		}
		return nil
	})
	sort.Strings(s.Files)
	return s, err
}
func snapshotZip(target string, o Options) (snapshot, error) {
	zr, err := zip.OpenReader(target)
	if err != nil {
		return snapshot{}, err
	}
	defer zr.Close()
	s := snapshot{Root: target, Dirs: map[string]bool{}, FileSet: map[string]bool{}, Text: map[string]string{}}
	for i, f := range zr.File {
		if i >= o.MaxFiles {
			break
		}
		rel := strings.TrimPrefix(filepath.ToSlash(f.Name), "./")
		if rel == "" {
			continue
		}
		if f.FileInfo().IsDir() {
			s.Dirs[strings.TrimSuffix(rel, "/")] = true
			continue
		}
		if shouldSkipPath(rel) {
			continue
		}
		s.Files = append(s.Files, rel)
		s.FileSet[rel] = true
		d := filepath.ToSlash(filepath.Dir(rel))
		if d != "." {
			s.Dirs[d] = true
		}
		if shouldRead(rel) && f.UncompressedSize64 <= 65536 {
			rc, err := f.Open()
			if err == nil {
				b, _ := io.ReadAll(rc)
				rc.Close()
				s.Text[rel] = string(b)
			}
		}
	}
	sort.Strings(s.Files)
	return s, nil
}
func shouldSkipDir(n string) bool {
	switch n {
	case ".git", "node_modules", "vendor", "dist", "build", "target", ".next", ".nuxt", ".cache", "coverage":
		return true
	}
	return false
}
func shouldSkipPath(p string) bool {
	parts := strings.Split(p, "/")
	for _, x := range parts {
		if shouldSkipDir(x) {
			return true
		}
	}
	return false
}
func shouldRead(p string) bool {
	b := filepath.Base(p)
	switch b {
	case "package.json", "composer.json", "go.mod", "requirements.txt", "pyproject.toml", "Cargo.toml", "pom.xml", "build.gradle", "Dockerfile", "docker-compose.yml", "compose.yml", ".env", ".env.example", "README.md", "CHANGELOG.md", "Makefile", ".gitflic-ci.yml", ".gitlab-ci.yml", "openapi.yaml", "openapi.yml", "openapi.json":
		return true
	}
	return strings.HasSuffix(p, ".service") || strings.HasSuffix(p, ".conf")
}
func limit(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func detectComponents(s snapshot) []Component {
	out := []Component{}
	add := func(kind StackKind, path, role string) {
		out = append(out, Component{Kind: string(kind), Path: path, Role: role})
	}
	for _, f := range s.Files {
		b := filepath.Base(f)
		switch b {
		case "package.json":
			add(StackNode, f, detectNodeRole(f, s.Text[f]))
		case "composer.json":
			add(StackPHP, f, "backend")
		case "go.mod":
			add(StackGo, f, "backend")
		case "requirements.txt", "pyproject.toml":
			add(StackPython, f, "backend")
		case "Cargo.toml":
			add(StackRust, f, "backend")
		case "pom.xml", "build.gradle":
			add(StackJava, f, "backend")
		case "Dockerfile", "docker-compose.yml", "compose.yml":
			add(StackDocker, f, "infrastructure")
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
func detectNodeRole(path, txt string) string {
	l := strings.ToLower(path + "\n" + txt)
	switch {
	case strings.Contains(l, "next") || strings.Contains(l, "nuxt") || strings.Contains(l, "vite") || strings.Contains(l, "react") || strings.Contains(l, "vue") || strings.Contains(l, "svelte") || strings.Contains(l, "frontend") || strings.Contains(l, "client"):
		if strings.Contains(l, "express") || strings.Contains(l, "nestjs") || strings.Contains(l, "fastify") {
			return "fullstack"
		}
		return "frontend"
	case strings.Contains(l, "express") || strings.Contains(l, "nestjs") || strings.Contains(l, "fastify") || strings.Contains(l, "koa") || strings.Contains(l, "backend") || strings.Contains(l, "server") || strings.Contains(l, "api"):
		return "backend"
	default:
		return "nodejs"
	}
}

func analyzeStructure(s snapshot, comps []Component) []Finding {
	fs := []Finding{}
	if !s.hasFile("README.md") {
		fs = append(fs, warn("stack.readme.missing", "Нет README.md", "Full-stack проекту нужен README с запуском, env-переменными, сборкой и production-режимом.", s.Root, "documentation"))
	}
	if len(comps) == 0 {
		fs = append(fs, warn("stack.components.missing", "Компоненты приложения не найдены", "Не обнаружены package.json, composer.json, go.mod, pyproject.toml, Cargo.toml, pom.xml, build.gradle или Dockerfile.", s.Root, "structure"))
	}
	if !s.anyDir("frontend", "client", "web", "app") {
		fs = append(fs, info("stack.frontend.dir.unknown", "Frontend-каталог не выделен", "Если проект содержит frontend, выделите его в frontend/client/web/app или зафиксируйте структуру в README.", s.Root, "frontend"))
	}
	if !s.anyDir("backend", "server", "api", "cmd", "internal", "src") {
		fs = append(fs, info("stack.backend.dir.unknown", "Backend-каталог не выделен", "Если проект содержит backend, выделите его в backend/server/api/cmd/internal/src или опишите структуру в README.", s.Root, "backend"))
	}
	if s.hasFile("README.md") {
		fs = append(fs, info("stack.readme.present", "README.md найден", "Документация проекта присутствует.", s.path("README.md"), "documentation"))
	}
	return fs
}
func analyzeFrontend(s snapshot, comps []Component) []Finding {
	fs := []Finding{}
	found := false
	for _, c := range comps {
		if c.Role == "frontend" || c.Role == "fullstack" {
			found = true
			p := c.Path
			txt := s.Text[p]
			if !containsAny(txt, "\"build\"", "'build'") {
				fs = append(fs, warn("stack.frontend.build.missing", "Не найден frontend build script", "В package.json frontend/fullstack компонента нужен script build для production-сборки.", s.path(p), "frontend", "build"))
			}
			if !containsAny(txt, "\"start\"", "'start'", "\"preview\"") {
				fs = append(fs, info("stack.frontend.start.missing", "Не найден start/preview script", "Для проверки production-сборки желательно иметь start или preview script.", s.path(p), "frontend"))
			}
		}
	}
	if !found {
		fs = append(fs, info("stack.frontend.not.detected", "Frontend-компонент не обнаружен", "StackDoctor не нашёл явный frontend Node.js компонент.", s.Root, "frontend"))
	}
	return fs
}
func analyzeBackend(s snapshot, comps []Component) []Finding {
	fs := []Finding{}
	found := false
	for _, c := range comps {
		if c.Role == "backend" || c.Role == "fullstack" {
			found = true
		}
	}
	if !found {
		fs = append(fs, info("stack.backend.not.detected", "Backend-компонент не обнаружен", "StackDoctor не нашёл явный backend компонент.", s.Root, "backend"))
	}
	if !s.anyDir("migrations", "database/migrations", "db/migrations") {
		fs = append(fs, info("stack.backend.migrations.missing", "Миграции БД не найдены", "Если проект использует БД, храните миграции в репозитории и описывайте порядок применения.", s.Root, "database"))
	}
	if !s.hasAnyFile("Makefile", ".gitflic-ci.yml", ".gitlab-ci.yml") {
		fs = append(fs, warn("stack.backend.automation.missing", "Автоматизация тестов/сборки не найдена", "Добавьте Makefile или CI с smoke/test/build командами.", s.Root, "ci"))
	}
	return fs
}
func analyzeEnv(s snapshot) []Finding {
	fs := []Finding{}
	if s.hasFile(".env") {
		fs = append(fs, danger("stack.env.real.present", "Обнаружен реальный .env", "Реальный .env не должен попадать в репозиторий или релизный архив.", s.path(".env"), "env", "secrets"))
	}
	if !s.hasFile(".env.example") && !s.hasFile(".env.sample") {
		fs = append(fs, warn("stack.env.example.missing", "Нет .env.example", "Добавьте шаблон переменных окружения без секретов.", s.Root, "env"))
	}
	if txt, ok := s.Text[".env.example"]; ok && looksSecret(txt) {
		fs = append(fs, danger("stack.env.example.secret", "В .env.example похожие на секреты значения", "Шаблон окружения должен содержать placeholders, а не реальные токены/пароли.", s.path(".env.example"), "env", "secrets"))
	}
	return fs
}
func analyzeDependencies(s snapshot, comps []Component) []Finding {
	fs := []Finding{}
	for _, c := range comps {
		dir := filepath.Dir(c.Path)
		if dir == "." {
			dir = ""
		}
		switch c.Kind {
		case string(StackNode):
			if !s.hasIn(dir, "package-lock.json") && !s.hasIn(dir, "pnpm-lock.yaml") && !s.hasIn(dir, "yarn.lock") && !s.hasIn(dir, "bun.lockb") {
				fs = append(fs, danger("stack.node.lock.missing", "Node.js lock-файл не найден", "Зафиксируйте зависимости через package-lock.json, pnpm-lock.yaml, yarn.lock или bun.lockb.", s.path(c.Path), "dependencies", "nodejs"))
			}
		case string(StackPHP):
			if !s.hasIn(dir, "composer.lock") {
				fs = append(fs, danger("stack.php.lock.missing", "composer.lock не найден", "Для production-релиза PHP-проекта нужен composer.lock.", s.path(c.Path), "dependencies", "php"))
			}
		case string(StackGo):
			if !s.hasIn(dir, "go.sum") {
				fs = append(fs, warn("stack.go.sum.missing", "go.sum не найден", "Для воспроизводимой Go-сборки нужен go.sum.", s.path(c.Path), "dependencies", "go"))
			}
		case string(StackRust):
			if !s.hasIn(dir, "Cargo.lock") {
				fs = append(fs, warn("stack.rust.lock.missing", "Cargo.lock не найден", "Для бинарного Rust-приложения обычно нужен Cargo.lock.", s.path(c.Path), "dependencies", "rust"))
			}
		}
	}
	return fs
}
func analyzeAPI(s snapshot) []Finding {
	fs := []Finding{}
	if !s.hasAnyFile("openapi.yaml", "openapi.yml", "openapi.json", "swagger.yaml", "swagger.json") {
		fs = append(fs, info("stack.api.openapi.missing", "OpenAPI/Swagger-схема не найдена", "Для backend/API части желательно хранить актуальную OpenAPI/Swagger-схему.", s.Root, "api"))
	}
	if !s.anyDir("routes", "controllers", "handlers", "api", "internal") {
		fs = append(fs, info("stack.api.routes.unknown", "Структура API-маршрутов не определена", "Не найден явный каталог routes/controllers/handlers/api/internal.", s.Root, "api"))
	}
	return fs
}
func analyzeDocker(s snapshot) []Finding {
	fs := []Finding{}
	if !s.hasFile("Dockerfile") {
		fs = append(fs, info("stack.dockerfile.missing", "Dockerfile не найден", "Для повторяемого деплоя full-stack проекта желательно иметь Dockerfile или описанную альтернативу.", s.Root, "docker"))
		return fs
	}
	txt := s.Text["Dockerfile"]
	if !s.hasFile(".dockerignore") {
		fs = append(fs, warn("stack.dockerignore.missing", "Нет .dockerignore", "Docker-сборка может включить .env, node_modules, .git и временные файлы.", s.path("Dockerfile"), "docker"))
	}
	if !containsAny(strings.ToLower(txt), "healthcheck") {
		fs = append(fs, warn("stack.docker.healthcheck.missing", "В Dockerfile нет HEALTHCHECK", "Production-образу полезен healthcheck для контроля состояния.", s.path("Dockerfile"), "docker"))
	}
	if !containsAny(strings.ToLower(txt), "user ") {
		fs = append(fs, info("stack.docker.user.missing", "В Dockerfile не задан USER", "Контейнер может запускаться от root, если USER не задан явно.", s.path("Dockerfile"), "docker", "security"))
	}
	if strings.Contains(strings.ToLower(txt), ":latest") {
		fs = append(fs, warn("stack.docker.latest", "Используется latest image tag", "Зафиксируйте базовый образ Docker по версии.", s.path("Dockerfile"), "docker"))
	}
	return fs
}
func analyzeProduction(s snapshot, comps []Component) []Finding {
	fs := []Finding{}
	if !s.hasFile("CHANGELOG.md") {
		fs = append(fs, warn("stack.changelog.missing", "Нет CHANGELOG.md", "Для product-релизов нужен журнал изменений.", s.Root, "release"))
	}
	if !s.hasFile("SECURITY.md") {
		fs = append(fs, info("stack.security.missing", "Нет SECURITY.md", "Добавьте правила сообщения об уязвимостях и безопасности проекта.", s.Root, "security"))
	}
	if !s.hasFile("LICENSE") && !s.hasFile("LICENSE.md") {
		fs = append(fs, warn("stack.license.missing", "Лицензия не найдена", "Добавьте LICENSE для понятного режима использования проекта.", s.Root, "license"))
	}
	if !s.hasFile("RELEASE_CHECKLIST.md") {
		fs = append(fs, info("stack.release.checklist.missing", "Нет RELEASE_CHECKLIST.md", "Release checklist снижает риск отдать неполный архив или сломанный build.", s.Root, "release"))
	}
	if !hasTestSignal(s) {
		fs = append(fs, info("stack.tests.unknown", "Тестовый сценарий не найден", "StackDoctor не нашёл явный test/smoke сценарий в Makefile, CI или package.json.", s.Root, "test"))
	}
	return fs
}

func (s snapshot) hasFile(name string) bool { return s.FileSet[name] }
func (s snapshot) hasAnyFile(names ...string) bool {
	for _, n := range names {
		if s.hasFile(n) {
			return true
		}
	}
	return false
}
func (s snapshot) anyDir(names ...string) bool {
	for _, d := range names {
		if s.Dirs[d] {
			return true
		}
		for x := range s.Dirs {
			if strings.HasSuffix(x, "/"+d) {
				return true
			}
		}
	}
	return false
}
func (s snapshot) hasIn(dir, name string) bool {
	if dir == "" || dir == "." {
		return s.hasFile(name)
	}
	return s.hasFile(filepath.ToSlash(filepath.Join(dir, name)))
}
func (s snapshot) path(rel string) string {
	if s.Root == "" {
		return rel
	}
	if strings.HasSuffix(strings.ToLower(s.Root), ".zip") {
		return s.Root + ":" + rel
	}
	return filepath.Join(s.Root, filepath.FromSlash(rel))
}
func containsAny(s string, vals ...string) bool {
	for _, v := range vals {
		if strings.Contains(s, v) {
			return true
		}
	}
	return false
}
func looksSecret(txt string) bool {
	l := strings.ToLower(txt)
	if containsAny(l, "changeme", "example", "placeholder", "your_", "xxx") {
		return false
	}
	return containsAny(l, "password=", "token=", "secret=", "api_key=", "private_key=")
}
func hasTestSignal(s snapshot) bool {
	for _, txt := range s.Text {
		l := strings.ToLower(txt)
		if containsAny(l, " test", "\"test\"", "go test", "pytest", "phpunit", "smoke") {
			return true
		}
	}
	return false
}

func info(id, t, m, p string, tags ...string) Finding {
	return Finding{ID: id, Severity: core.SeverityInfo, Title: t, Message: m, Path: p, Tags: tags}
}
func warn(id, t, m, p string, tags ...string) Finding {
	return Finding{ID: id, Severity: core.SeverityWarn, Title: t, Message: m, Path: p, Tags: tags}
}
func danger(id, t, m, p string, tags ...string) Finding {
	return Finding{ID: id, Severity: core.SeverityDanger, Title: t, Message: m, Path: p, Tags: tags}
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
func sum(fs []Finding) map[string]int {
	m := map[string]int{"critical": 0, "danger": 0, "warn": 0, "info": 0}
	for _, f := range fs {
		switch f.Severity {
		case core.SeverityCritical:
			m["critical"]++
		case core.SeverityDanger:
			m["danger"]++
		case core.SeverityWarn:
			m["warn"]++
		default:
			m["info"]++
		}
	}
	return m
}
func dedupe(in []Finding) []Finding {
	seen := map[string]bool{}
	out := []Finding{}
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
func write(r Report, w io.Writer, o Options) error {
	if o.JSON || o.Output != "" {
		b, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			return err
		}
		b = append(b, '\n')
		if o.Output == "" {
			_, err = w.Write(b)
			return err
		}
		if err := os.MkdirAll(filepath.Dir(o.Output), 0755); err != nil && filepath.Dir(o.Output) != "." {
			return err
		}
		if err := os.WriteFile(o.Output, b, 0644); err != nil {
			return err
		}
		fmt.Fprintf(w, "Отчёт сохранён: %s\n", o.Output)
		return nil
	}
	fmt.Fprintf(w, "%s %s: full-stack аудит %s\n", r.Tool, r.Version, r.Target)
	fmt.Fprintf(w, "Компоненты: %d\n", len(r.Components))
	fmt.Fprintf(w, "Итог: critical=%d, danger=%d, warn=%d, info=%d\n", r.Summary["critical"], r.Summary["danger"], r.Summary["warn"], r.Summary["info"])
	for _, f := range r.Findings {
		fmt.Fprintf(w, "[%s] %s — %s (%s)\n", f.Severity, f.Title, f.Message, f.Path)
	}
	return nil
}
