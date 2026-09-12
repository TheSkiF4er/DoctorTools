package apidoctor

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
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
	Timeout   time.Duration
}

type APIFile struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

type Endpoint struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Source string `json:"source"`
	Line   int    `json:"line"`
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
	Files      []APIFile         `json:"files"`
	Endpoints  []Endpoint        `json:"endpoints"`
	Findings   []Finding         `json:"findings"`
	Artifacts  map[string]string `json:"artifacts"`
}

type snapshot struct {
	Root    string
	Files   []string
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
		p := version.Products["api"]
		fmt.Fprintf(stdout, "%s %s\nПлатформа: %s %s\nСтатус: %s\n", p.Name, p.Version, version.PlatformName, version.PlatformVersion, p.Status)
		return nil
	}
	cmd := normalize(args)
	if !supported(cmd) {
		return usageError{"неизвестная команда APIDoctor: " + strings.Join(args, " ")}
	}
	target, opts, err := parse(args)
	if err != nil {
		return err
	}
	report, err := Analyze(cmd, target, opts)
	if err != nil {
		return err
	}
	return write(report, stdout, opts)
}

func help(w io.Writer) {
	p := version.Products["api"]
	fmt.Fprintf(w, `%s %s
Offline-диагностика backend/API проектов: OpenAPI, маршруты, security middleware, CORS, rate limit, validation и health endpoints.

Использование:
  apidoctor <команда> [путь|url] [--json] [--output файл] [--recursive] [--timeout 5s]

Команды:
  scan
  openapi validate
  routes check
  security audit
  health check
  probe
  version
`, p.Name, p.Version)
}

func normalize(args []string) string {
	if len(args) >= 2 {
		pair := args[0] + " " + args[1]
		switch pair {
		case "openapi validate", "routes check", "security audit", "health check":
			return pair
		}
	}
	return args[0]
}

func supported(cmd string) bool {
	switch cmd {
	case "scan", "check", "audit", "openapi validate", "routes check", "security audit", "health check", "probe", "validate":
		return true
	}
	return false
}

func parse(args []string) (string, Options, error) {
	target := "."
	opts := Options{MaxFiles: 12000, Timeout: 5 * time.Second}
	skip := 1
	if len(args) >= 2 {
		pair := args[0] + " " + args[1]
		if pair == "openapi validate" || pair == "routes check" || pair == "security audit" || pair == "health check" {
			skip = 2
		}
	}
	for i := skip; i < len(args); i++ {
		v := args[i]
		switch {
		case v == "--json":
			opts.JSON = true
		case v == "--recursive" || v == "-r":
			opts.Recursive = true
		case v == "--output" || v == "-o":
			if i+1 >= len(args) {
				return target, opts, usageError{"после " + v + " нужно указать файл"}
			}
			opts.Output = args[i+1]
			i++
		case strings.HasPrefix(v, "--output="):
			opts.Output = strings.TrimPrefix(v, "--output=")
		case v == "--timeout":
			if i+1 >= len(args) {
				return target, opts, usageError{"после --timeout нужно указать длительность, например 5s"}
			}
			d, err := time.ParseDuration(args[i+1])
			if err != nil {
				return target, opts, err
			}
			opts.Timeout = d
			i++
		case strings.HasPrefix(v, "--timeout="):
			d, err := time.ParseDuration(strings.TrimPrefix(v, "--timeout="))
			if err != nil {
				return target, opts, err
			}
			opts.Timeout = d
		case strings.HasPrefix(v, "-"):
			return target, opts, usageError{"неизвестный флаг: " + v}
		default:
			target = v
		}
	}
	return target, opts, nil
}

func Analyze(cmd, target string, opts Options) (Report, error) {
	started := time.Now().UTC()
	files := []APIFile{}
	endpoints := []Endpoint{}
	findings := []Finding{{ID: "api.profile.ready", Severity: core.SeverityInfo, Title: "Профиль APIDoctor готов", Message: "Выполнена диагностика backend/API проекта.", Path: target, Tags: []string{"apidoctor", "api"}}}

	if isURL(target) && (cmd == "probe" || cmd == "health check") {
		findings = append(findings, probeURL(target, opts)...)
	} else {
		s, err := takeSnapshot(target, opts)
		if err != nil {
			return Report{}, err
		}
		files = detectAPIFiles(s)
		endpoints = detectEndpoints(s)
		switch cmd {
		case "openapi validate", "validate":
			findings = append(findings, analyzeOpenAPI(s, files)...)
		case "routes check":
			findings = append(findings, analyzeRoutes(s, endpoints)...)
		case "security audit", "audit":
			findings = append(findings, analyzeSecurity(s)...)
		case "health check", "probe":
			findings = append(findings, analyzeHealth(s, endpoints)...)
		default:
			findings = append(findings, analyzeOpenAPI(s, files)...)
			findings = append(findings, analyzeRoutes(s, endpoints)...)
			findings = append(findings, analyzeSecurity(s)...)
			findings = append(findings, analyzeValidation(s)...)
			findings = append(findings, analyzeHealth(s, endpoints)...)
			findings = append(findings, analyzeConsistency(s, endpoints)...)
		}
	}
	findings = dedupe(findings)
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Severity == findings[j].Severity {
			return findings[i].ID < findings[j].ID
		}
		return rank(findings[i].Severity) < rank(findings[j].Severity)
	})
	return Report{Tool: "APIDoctor", Version: version.Products["api"].Version, Platform: version.PlatformName, Target: target, Command: cmd, StartedAt: started.Format(time.RFC3339), FinishedAt: time.Now().UTC().Format(time.RFC3339), Summary: sum(findings), Files: files, Endpoints: endpoints, Findings: findings, Artifacts: map[string]string{"engine": "APIDoctor offline analyzer", "engine_version": version.Products["api"].Version, "mode": "offline-first"}}, nil
}

func takeSnapshot(target string, opts Options) (snapshot, error) {
	if opts.MaxFiles <= 0 {
		opts.MaxFiles = 12000
	}
	info, err := os.Stat(target)
	if err != nil {
		return snapshot{}, err
	}
	root := target
	out := snapshot{Root: root, FileSet: map[string]bool{}, Text: map[string]string{}}
	add := func(path string) {
		rel := path
		if info.IsDir() {
			if r, err := filepath.Rel(root, path); err == nil {
				rel = filepath.ToSlash(r)
			}
		}
		out.Files = append(out.Files, rel)
		out.FileSet[filepath.ToSlash(rel)] = true
		if isTextCandidate(path) {
			if b, err := os.ReadFile(path); err == nil && len(b) <= 2*1024*1024 {
				out.Text[filepath.ToSlash(rel)] = string(b)
			}
		}
	}
	if !info.IsDir() {
		out.Root = filepath.Dir(target)
		add(target)
		return out, nil
	}
	if !opts.Recursive {
		entries, err := os.ReadDir(target)
		if err != nil {
			return snapshot{}, err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			add(filepath.Join(target, e.Name()))
		}
		return out, nil
	}
	count := 0
	filepath.WalkDir(target, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == "dist" || name == "build" || name == "target" || name == ".next" || name == "coverage" {
				return filepath.SkipDir
			}
			return nil
		}
		count++
		if count > opts.MaxFiles {
			return filepath.SkipDir
		}
		add(path)
		return nil
	})
	return out, nil
}

func isTextCandidate(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(path))
	known := map[string]bool{"openapi.yaml": true, "openapi.yml": true, "openapi.json": true, "swagger.yaml": true, "swagger.json": true, "package.json": true, "composer.json": true, "go.mod": true, "pyproject.toml": true, ".env": true, ".env.example": true, "readme.md": true}
	if known[base] {
		return true
	}
	switch ext {
	case ".go", ".js", ".ts", ".php", ".py", ".java", ".cs", ".kt", ".rb", ".rs", ".yaml", ".yml", ".json", ".toml", ".md", ".env":
		return true
	}
	return false
}

func detectAPIFiles(s snapshot) []APIFile {
	out := []APIFile{}
	for _, f := range s.Files {
		b := strings.ToLower(filepath.Base(f))
		d := strings.ToLower(filepath.Dir(f))
		switch {
		case b == "openapi.yaml" || b == "openapi.yml" || b == "openapi.json":
			out = append(out, APIFile{f, "openapi"})
		case b == "swagger.yaml" || b == "swagger.yml" || b == "swagger.json":
			out = append(out, APIFile{f, "swagger"})
		case strings.Contains(d, "route") || strings.Contains(d, "controller") || strings.Contains(d, "handler") || strings.Contains(d, "middleware") || strings.Contains(d, "api"):
			if isSource(f) {
				out = append(out, APIFile{f, "api-source"})
			}
		case strings.Contains(b, "route") || strings.Contains(b, "controller") || strings.Contains(b, "handler") || strings.Contains(b, "middleware"):
			if isSource(f) {
				out = append(out, APIFile{f, "api-source"})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func isSource(f string) bool {
	switch strings.ToLower(filepath.Ext(f)) {
	case ".go", ".js", ".ts", ".php", ".py", ".java", ".cs", ".kt", ".rb", ".rs":
		return true
	}
	return false
}

var routePatterns = []*regexp.Regexp{
	regexp.MustCompile("(?i)\\b(GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)\\s+[\"'`]([^\"'`]+)[\"'`]"),
	regexp.MustCompile("(?i)\\.(get|post|put|patch|delete|options|head)\\s*\\(\\s*[\"'`]([^\"'`]+)[\"'`]"),
	regexp.MustCompile("(?i)@(Get|Post|Put|Patch|Delete|RequestMapping)\\w*\\s*\\(\\s*[\"'`]([^\"'`]+)[\"'`]"),
	regexp.MustCompile("(?i)(router|r|app)\\.(GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)\\s*\\(\\s*[\"'`]([^\"'`]+)[\"'`]"),
}

func detectEndpoints(s snapshot) []Endpoint {
	out := []Endpoint{}
	for path, text := range s.Text {
		if !isSource(path) && !strings.Contains(strings.ToLower(filepath.Base(path)), "openapi") && !strings.Contains(strings.ToLower(filepath.Base(path)), "swagger") {
			continue
		}
		scanner := bufio.NewScanner(strings.NewReader(text))
		line := 0
		for scanner.Scan() {
			line++
			row := scanner.Text()
			for _, re := range routePatterns {
				m := re.FindStringSubmatch(row)
				if len(m) >= 3 {
					method := strings.ToUpper(m[1])
					p := m[2]
					if len(m) >= 4 {
						method = strings.ToUpper(m[2])
						p = m[3]
					}
					out = append(out, Endpoint{Method: method, Path: p, Source: path, Line: line})
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Source == out[j].Source {
			return out[i].Line < out[j].Line
		}
		return out[i].Source < out[j].Source
	})
	return out
}

func analyzeOpenAPI(s snapshot, files []APIFile) []Finding {
	fs := []Finding{}
	openapi := []APIFile{}
	for _, f := range files {
		if f.Kind == "openapi" || f.Kind == "swagger" {
			openapi = append(openapi, f)
		}
	}
	if len(openapi) == 0 {
		return []Finding{warn("api.openapi.missing", "OpenAPI/Swagger-схема не найдена", "Для backend/API проекта нужна актуальная спецификация openapi.yaml/json или swagger.yaml/json.", s.Root, "openapi")}
	}
	for _, f := range openapi {
		text := strings.ToLower(s.Text[f.Path])
		if !strings.Contains(text, "openapi:") && !strings.Contains(text, "\"openapi\"") && !strings.Contains(text, "swagger:") && !strings.Contains(text, "\"swagger\"") {
			fs = append(fs, danger("api.openapi.version.missing", "В спецификации не найдена версия OpenAPI/Swagger", "Файл похож на API-спецификацию, но не содержит явного openapi/swagger поля.", f.Path, "openapi"))
		}
		if !strings.Contains(text, "paths:") && !strings.Contains(text, "\"paths\"") {
			fs = append(fs, danger("api.openapi.paths.missing", "В спецификации не найден раздел paths", "Без paths спецификация не описывает маршруты API.", f.Path, "openapi"))
		}
		if !strings.Contains(text, "info:") && !strings.Contains(text, "\"info\"") {
			fs = append(fs, warn("api.openapi.info.missing", "В спецификации не найден раздел info", "Добавьте title/version/description в OpenAPI info.", f.Path, "openapi"))
		}
		if !strings.Contains(text, "securityschemes") && !strings.Contains(text, "securityschemes:") && !strings.Contains(text, "security:") {
			fs = append(fs, info("api.openapi.security.missing", "В OpenAPI не описана безопасность", "Опишите securitySchemes/security для токенов, cookie, OAuth или другого механизма авторизации.", f.Path, "security"))
		}
	}
	return fs
}

func analyzeRoutes(s snapshot, endpoints []Endpoint) []Finding {
	fs := []Finding{}
	if len(endpoints) == 0 {
		fs = append(fs, warn("api.routes.missing", "Маршруты API не обнаружены", "APIDoctor не нашёл HTTP route declarations в типовых source-файлах.", s.Root, "routes"))
	} else {
		fs = append(fs, info("api.routes.detected", "Маршруты API обнаружены", fmt.Sprintf("Найдено маршрутов: %d.", len(endpoints)), s.Root, "routes"))
	}
	methods := map[string]bool{}
	for _, e := range endpoints {
		methods[e.Method] = true
	}
	if len(endpoints) > 0 && !methods["GET"] {
		fs = append(fs, info("api.routes.get.missing", "GET-маршруты не найдены", "Проверьте, не пропущены ли read-endpoints или health endpoints.", s.Root, "routes"))
	}
	if len(endpoints) > 0 && !methods["POST"] && !methods["PUT"] && !methods["PATCH"] {
		fs = append(fs, info("api.routes.write.missing", "Write-маршруты не найдены", "Если API изменяет состояние, должны быть явно описаны POST/PUT/PATCH endpoints.", s.Root, "routes"))
	}
	return fs
}

func analyzeSecurity(s snapshot) []Finding {
	all := strings.ToLower(joinText(s))
	fs := []Finding{}
	if s.FileSet[".env"] {
		fs = append(fs, danger("api.env.present", "Обнаружен .env", "Реальный .env не должен попадать в репозиторий или релизный архив API.", filepath.Join(s.Root, ".env"), "secrets"))
	}
	if !s.FileSet[".env.example"] {
		fs = append(fs, warn("api.env.example.missing", "Нет .env.example", "Для API нужен шаблон переменных окружения без секретов.", s.Root, "env"))
	}
	if !containsAny(all, "auth", "authorize", "authentication", "bearer", "jwt", "session", "passport", "sanctum", "middleware") {
		fs = append(fs, warn("api.auth.signal.missing", "Не найдены признаки авторизации", "Проверьте наличие auth middleware и защиту приватных маршрутов.", s.Root, "auth"))
	}
	if containsAny(all, "access-control-allow-origin: *", "origin: '*'", "origin: \"*\"", "allow_origins=[\"*\"]") {
		fs = append(fs, danger("api.cors.wildcard", "Обнаружен wildcard CORS", "Не используйте Access-Control-Allow-Origin: * для приватного API.", s.Root, "cors"))
	}
	if !containsAny(all, "cors", "access-control-allow-origin", "allow_origins") {
		fs = append(fs, info("api.cors.signal.missing", "CORS-настройки не найдены", "Для browser-facing API явно настройте разрешённые origins, методы и headers.", s.Root, "cors"))
	}
	if !containsAny(all, "rate", "limiter", "throttle", "slowapi", "express-rate-limit") {
		fs = append(fs, warn("api.rate_limit.missing", "Не найдены признаки rate limit", "Для публичного API нужен throttling/rate limit на чувствительных маршрутах.", s.Root, "rate-limit"))
	}
	return fs
}

func analyzeValidation(s snapshot) []Finding {
	all := strings.ToLower(joinText(s))
	fs := []Finding{}
	if !containsAny(all, "validate", "validator", "schema", "zod", "joi", "pydantic", "request", "formrequest", "bindingresult", "serde") {
		fs = append(fs, warn("api.validation.missing", "Не найдены признаки валидации запросов", "Добавьте request validation через schema/DTO/validator/pydantic/zod/joi/FormRequest или аналог.", s.Root, "validation"))
	}
	if !containsAny(all, "error", "errors", "exception", "handler", "problem+json") {
		fs = append(fs, info("api.error_format.unknown", "Формат ошибок не определён", "Желательно иметь единый JSON-формат ошибок и централизованный exception handler.", s.Root, "errors"))
	}
	return fs
}

func analyzeHealth(s snapshot, endpoints []Endpoint) []Finding {
	all := strings.ToLower(joinText(s))
	fs := []Finding{}
	for _, e := range endpoints {
		if strings.Contains(strings.ToLower(e.Path), "health") || strings.Contains(strings.ToLower(e.Path), "ready") || strings.Contains(strings.ToLower(e.Path), "live") {
			return []Finding{info("api.health.detected", "Health endpoint найден", "Обнаружен endpoint для health/readiness/liveness проверки.", e.Source, "health")}
		}
	}
	if containsAny(all, "/health", "healthcheck", "readiness", "liveness", "/ready", "/live") {
		return []Finding{info("api.health.detected", "Health endpoint найден", "Обнаружены признаки health/readiness/liveness проверки.", s.Root, "health")}
	}
	fs = append(fs, warn("api.health.missing", "Health endpoint не найден", "Для production API нужен /health, /ready или аналогичный endpoint для мониторинга и деплоя.", s.Root, "health"))
	return fs
}

func analyzeConsistency(s snapshot, endpoints []Endpoint) []Finding {
	fs := []Finding{}
	if len(endpoints) >= 10 {
		byPath := map[string]int{}
		for _, e := range endpoints {
			byPath[e.Path]++
		}
		for p, n := range byPath {
			if n >= 4 {
				fs = append(fs, info("api.rest.resource.detected", "Обнаружен REST-like ресурс", fmt.Sprintf("Путь %s используется в %d методах.", p, n), s.Root, "rest"))
				break
			}
		}
	}
	all := strings.ToLower(joinText(s))
	if !containsAny(all, "pagination", "paginate", "page=", "limit", "offset", "cursor") {
		fs = append(fs, info("api.pagination.unknown", "Пагинация не обнаружена", "Для списочных endpoints желательно иметь page/limit, offset или cursor pagination.", s.Root, "pagination"))
	}
	return fs
}

func probeURL(target string, opts Options) []Finding {
	client := http.Client{Timeout: opts.Timeout}
	candidates := []string{target}
	trim := strings.TrimRight(target, "/")
	if !strings.HasSuffix(trim, "/health") {
		candidates = append(candidates, trim+"/health", trim+"/ready")
	}
	for _, u := range candidates {
		resp, err := client.Get(u)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 500 {
			sev := core.SeverityInfo
			title := "HTTP endpoint доступен"
			if resp.StatusCode >= 400 {
				sev = core.SeverityWarn
				title = "HTTP endpoint отвечает ошибкой"
			}
			return []Finding{{ID: "api.probe.http", Severity: sev, Title: title, Message: fmt.Sprintf("%s вернул HTTP %d.", u, resp.StatusCode), Path: u, Tags: []string{"probe", "http"}}}
		}
	}
	return []Finding{{ID: "api.probe.unavailable", Severity: core.SeverityWarn, Title: "HTTP probe не получил ответа", Message: "APIDoctor не смог получить успешный HTTP-ответ от URL или health endpoints.", Path: target, Tags: []string{"probe", "http"}}}
}

func joinText(s snapshot) string {
	var b strings.Builder
	for _, t := range s.Text {
		b.WriteString("\n")
		b.WriteString(t)
	}
	return b.String()
}
func containsAny(s string, xs ...string) bool {
	for _, x := range xs {
		if strings.Contains(s, strings.ToLower(x)) {
			return true
		}
	}
	return false
}
func isURL(s string) bool {
	return strings.HasPrefix(strings.ToLower(s), "http://") || strings.HasPrefix(strings.ToLower(s), "https://")
}

func write(r Report, w io.Writer, opts Options) error {
	if opts.JSON || opts.Output != "" {
		b, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			return err
		}
		b = append(b, '\n')
		if opts.Output == "" {
			_, err = w.Write(b)
			return err
		}
		if err := os.MkdirAll(filepath.Dir(opts.Output), 0755); err != nil && filepath.Dir(opts.Output) != "." {
			return err
		}
		if err := os.WriteFile(opts.Output, b, 0644); err != nil {
			return err
		}
		fmt.Fprintf(w, "Отчёт сохранён: %s\n", opts.Output)
		return nil
	}
	fmt.Fprintf(w, "%s %s: команда %s, цель %s\n", r.Tool, r.Version, r.Command, r.Target)
	fmt.Fprintf(w, "Итог: critical=%d, danger=%d, warn=%d, info=%d\n", r.Summary["critical"], r.Summary["danger"], r.Summary["warn"], r.Summary["info"])
	fmt.Fprintf(w, "API-файлов: %d, endpoints: %d\n", len(r.Files), len(r.Endpoints))
	for _, f := range r.Findings {
		fmt.Fprintf(w, "[%s] %s — %s (%s)\n", f.Severity, f.Title, f.Message, f.Path)
	}
	return nil
}

func info(id, title, msg, path string, tags ...string) Finding {
	return Finding{ID: id, Severity: core.SeverityInfo, Title: title, Message: msg, Path: path, Tags: tags}
}
func warn(id, title, msg, path string, tags ...string) Finding {
	return Finding{ID: id, Severity: core.SeverityWarn, Title: title, Message: msg, Path: path, Tags: tags}
}
func danger(id, title, msg, path string, tags ...string) Finding {
	return Finding{ID: id, Severity: core.SeverityDanger, Title: title, Message: msg, Path: path, Tags: tags}
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
		k := f.ID + "|" + f.Path + "|" + f.Message
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, f)
	}
	return out
}
