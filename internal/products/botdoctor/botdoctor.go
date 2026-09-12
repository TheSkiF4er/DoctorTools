package botdoctor

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
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
}

type BotFile struct {
	Path     string   `json:"path"`
	Kind     string   `json:"kind"`
	Platform []string `json:"platform,omitempty"`
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
	Files      []BotFile         `json:"files"`
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
		p := version.Products["bot"]
		fmt.Fprintf(stdout, "%s %s\nПлатформа: %s %s\nСтатус: %s\n", p.Name, p.Version, version.PlatformName, version.PlatformVersion, p.Status)
		return nil
	}
	cmd := normalize(args)
	if !supported(cmd) {
		return usageError{"неизвестная команда BotDoctor: " + strings.Join(args, " ")}
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
	p := version.Products["bot"]
	fmt.Fprintf(w, `%s %s
Offline-диагностика Telegram/VK/Discord-ботов: токены, env, webhook/polling, logging, rate limit, systemd, Docker и BotHost-ready состояние.

Использование:
  botdoctor <команда> [путь] [--json] [--output файл] [--recursive]

Команды:
  scan
  telegram check
  discord check
  vk check
  env audit
  webhook check
  bothost check
  security audit
  version
`, p.Name, p.Version)
}

func normalize(args []string) string {
	if len(args) >= 2 {
		pair := args[0] + " " + args[1]
		switch pair {
		case "telegram check", "discord check", "vk check", "env audit", "webhook check", "bothost check", "security audit":
			return pair
		}
	}
	return args[0]
}

func supported(cmd string) bool {
	switch cmd {
	case "scan", "check", "audit", "telegram check", "discord check", "vk check", "env audit", "webhook check", "bothost check", "security audit":
		return true
	}
	return false
}

func parse(args []string) (string, Options, error) {
	target := "."
	opts := Options{MaxFiles: 12000}
	skip := 1
	if len(args) >= 2 {
		pair := args[0] + " " + args[1]
		if pair == "telegram check" || pair == "discord check" || pair == "vk check" || pair == "env audit" || pair == "webhook check" || pair == "bothost check" || pair == "security audit" {
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
	s, err := takeSnapshot(target, opts)
	if err != nil {
		return Report{}, err
	}
	files := detectBotFiles(s)
	findings := []Finding{{ID: "bot.profile.ready", Severity: core.SeverityInfo, Title: "Профиль BotDoctor готов", Message: "Выполнена диагностика bot-проекта.", Path: target, Tags: []string{"botdoctor", "bot"}}}
	switch cmd {
	case "telegram check":
		findings = append(findings, analyzePlatform(s, files, "telegram")...)
	case "discord check":
		findings = append(findings, analyzePlatform(s, files, "discord")...)
	case "vk check":
		findings = append(findings, analyzePlatform(s, files, "vk")...)
	case "env audit":
		findings = append(findings, analyzeEnv(s)...)
	case "webhook check":
		findings = append(findings, analyzeWebhook(s)...)
	case "bothost check":
		findings = append(findings, analyzeBotHost(s)...)
	case "security audit", "audit":
		findings = append(findings, analyzeSecrets(s)...)
		findings = append(findings, analyzeEnv(s)...)
	default:
		findings = append(findings, analyzeProjectShape(s, files)...)
		findings = append(findings, analyzePlatform(s, files, "telegram")...)
		findings = append(findings, analyzePlatform(s, files, "discord")...)
		findings = append(findings, analyzePlatform(s, files, "vk")...)
		findings = append(findings, analyzeEnv(s)...)
		findings = append(findings, analyzeSecrets(s)...)
		findings = append(findings, analyzeWebhook(s)...)
		findings = append(findings, analyzeRuntime(s)...)
		findings = append(findings, analyzeBotHost(s)...)
	}
	findings = dedupe(findings)
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Severity == findings[j].Severity {
			return findings[i].ID < findings[j].ID
		}
		return rank(findings[i].Severity) < rank(findings[j].Severity)
	})
	return Report{Tool: "BotDoctor", Version: version.Products["bot"].Version, Platform: version.PlatformName, Target: target, Command: cmd, StartedAt: started.Format(time.RFC3339), FinishedAt: time.Now().UTC().Format(time.RFC3339), Summary: sum(findings), Files: files, Findings: findings, Artifacts: map[string]string{"engine": "BotDoctor offline analyzer", "engine_version": version.Products["bot"].Version, "mode": "offline"}}, nil
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
	err = filepath.WalkDir(target, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".git" || name == "node_modules" || name == "vendor" || name == "dist" || name == "build" || name == ".next" || name == "target" || name == "bin" {
				return filepath.SkipDir
			}
			return nil
		}
		if len(out.Files) >= opts.MaxFiles {
			return filepath.SkipAll
		}
		add(path)
		return nil
	})
	return out, err
}

func isTextCandidate(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(path))
	if strings.HasPrefix(base, ".env") || base == "dockerfile" || strings.HasSuffix(base, ".service") {
		return true
	}
	switch ext {
	case ".go", ".js", ".ts", ".mjs", ".cjs", ".py", ".php", ".rb", ".java", ".kt", ".cs", ".json", ".yaml", ".yml", ".toml", ".ini", ".conf", ".md", ".sh", ".service", ".env", ".txt":
		return true
	}
	return false
}

func detectBotFiles(s snapshot) []BotFile {
	items := []BotFile{}
	for _, f := range s.Files {
		lf := strings.ToLower(f)
		kind := ""
		switch {
		case filepath.Base(lf) == "package.json":
			kind = "node-manifest"
		case filepath.Base(lf) == "requirements.txt" || filepath.Base(lf) == "pyproject.toml":
			kind = "python-manifest"
		case filepath.Base(lf) == "go.mod":
			kind = "go-manifest"
		case strings.Contains(lf, "dockerfile") || strings.HasSuffix(lf, "docker-compose.yml") || strings.HasSuffix(lf, "compose.yml"):
			kind = "container"
		case strings.HasSuffix(lf, ".service"):
			kind = "systemd"
		case strings.HasPrefix(filepath.Base(lf), ".env"):
			kind = "env"
		case strings.Contains(lf, "bot") || strings.Contains(lf, "telegram") || strings.Contains(lf, "discord") || strings.Contains(lf, "vk"):
			kind = "bot-source"
		}
		platform := platformsFromText(lf + "\n" + strings.ToLower(s.Text[f]))
		if kind != "" || len(platform) > 0 {
			if kind == "" {
				kind = "bot-related"
			}
			items = append(items, BotFile{Path: f, Kind: kind, Platform: platform})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	return items
}

func analyzeProjectShape(s snapshot, files []BotFile) []Finding {
	out := []Finding{}
	if len(files) == 0 {
		out = append(out, Finding{ID: "bot.project.no_bot_files", Severity: core.SeverityWarn, Title: "Не найдены bot-артефакты", Message: "BotDoctor не нашёл явных файлов Telegram/VK/Discord-бота. Укажите корень проекта или включите релевантные исходники.", Path: s.Root, Tags: []string{"bot", "structure"}})
	}
	if !hasAny(s, "README.md", "readme.md") {
		out = append(out, Finding{ID: "bot.docs.no_readme", Severity: core.SeverityWarn, Title: "Нет README", Message: "Для bot-проекта нужен README с запуском, env-переменными, режимом webhook/polling и обслуживанием.", Path: s.Root, Tags: []string{"docs"}})
	}
	if !hasAny(s, "Makefile", ".gitflic-ci.yml", ".github/workflows/ci.yml", ".gitlab-ci.yml") {
		out = append(out, Finding{ID: "bot.ci.no_automation", Severity: core.SeverityWarn, Title: "Нет явной автоматизации проверки", Message: "Не найден Makefile или CI-конфигурация для тестов/сборки bot-проекта.", Path: s.Root, Tags: []string{"ci", "release"}})
	}
	return out
}

func analyzePlatform(s snapshot, files []BotFile, platform string) []Finding {
	out := []Finding{}
	if !mentionsPlatform(s, files, platform) {
		return []Finding{{ID: "bot." + platform + ".not_detected", Severity: core.SeverityInfo, Title: "Платформа не обнаружена", Message: "В проекте не найдены явные признаки платформы " + platform + ".", Path: s.Root, Tags: []string{platform}}}
	}
	out = append(out, Finding{ID: "bot." + platform + ".detected", Severity: core.SeverityInfo, Title: "Платформа обнаружена", Message: "BotDoctor нашёл признаки bot-интеграции: " + platform + ".", Path: s.Root, Tags: []string{platform}})
	tokenKey := map[string]string{"telegram": "TELEGRAM_BOT_TOKEN", "discord": "DISCORD_BOT_TOKEN", "vk": "VK_TOKEN"}[platform]
	if tokenKey != "" && !envExampleHas(s, tokenKey) {
		out = append(out, Finding{ID: "bot." + platform + ".token_missing_in_env_example", Severity: core.SeverityWarn, Title: "Токен не описан в .env.example", Message: "Для " + platform + " рекомендуется явно описать " + tokenKey + " в .env.example без реального значения.", Path: ".env.example", Tags: []string{platform, "env"}})
	}
	return out
}

func analyzeEnv(s snapshot) []Finding {
	out := []Finding{}
	if !hasAny(s, ".env.example", "env.example", ".env.sample") {
		out = append(out, Finding{ID: "bot.env.no_example", Severity: core.SeverityDanger, Title: "Нет .env.example", Message: "Bot-проект должен поставляться с примером env-переменных без секретов.", Path: s.Root, Tags: []string{"env", "security"}})
	}
	if hasAny(s, ".env") {
		out = append(out, Finding{ID: "bot.env.real_env_present", Severity: core.SeverityDanger, Title: "В проекте найден .env", Message: "Реальный .env не должен попадать в репозиторий или релизный архив.", Path: ".env", Tags: []string{"env", "secret"}})
	}
	for path, text := range s.Text {
		if strings.HasPrefix(strings.ToLower(filepath.Base(path)), ".env") && !strings.Contains(strings.ToLower(filepath.Base(path)), "example") && !strings.Contains(strings.ToLower(filepath.Base(path)), "sample") {
			if looksLikeSecret(text) {
				out = append(out, Finding{ID: "bot.env.possible_secret." + safeID(path), Severity: core.SeverityDanger, Title: "Похожий на секрет env-файл", Message: "Файл содержит строки, похожие на реальные bot-токены или секреты.", Path: path, Tags: []string{"env", "secret"}})
			}
		}
	}
	return out
}

func analyzeSecrets(s snapshot) []Finding {
	out := []Finding{}
	patterns := []struct {
		id, title string
		re        *regexp.Regexp
	}{
		{"telegram_token", "Похожий на Telegram Bot Token", regexp.MustCompile(`\b\d{6,12}:[A-Za-z0-9_-]{30,}\b`)},
		{"discord_token", "Похожий на Discord Token", regexp.MustCompile(`\b[MNO][A-Za-z\d_-]{20,}\.[A-Za-z\d_-]{6,}\.[A-Za-z\d_-]{20,}\b`)},
		{"vk_token", "Похожий на VK access token", regexp.MustCompile(`vk1\.[A-Za-z0-9_\-.]{40,}`)},
	}
	for path, text := range s.Text {
		if isAllowedExample(path) {
			continue
		}
		for _, p := range patterns {
			if p.re.FindString(text) != "" {
				out = append(out, Finding{ID: "bot.secret." + p.id + "." + safeID(path), Severity: core.SeverityCritical, Title: p.title, Message: "В файле найдено значение, похожее на реальный bot-токен. Его нужно удалить из истории и перевыпустить токен.", Path: path, Tags: []string{"secret", "token"}})
			}
		}
	}
	return out
}

func analyzeWebhook(s snapshot) []Finding {
	all := strings.ToLower(joinText(s))
	out := []Finding{}
	hasWebhook := strings.Contains(all, "webhook") || strings.Contains(all, "setwebhook") || strings.Contains(all, "interactions_endpoint_url")
	hasPolling := strings.Contains(all, "polling") || strings.Contains(all, "longpoll") || strings.Contains(all, "long_poll") || strings.Contains(all, "getupdates")
	if !hasWebhook && !hasPolling {
		out = append(out, Finding{ID: "bot.runtime.mode_not_documented", Severity: core.SeverityWarn, Title: "Не определён режим получения событий", Message: "Не найдено явных признаков webhook, polling или long poll. Рекомендуется задокументировать режим запуска.", Path: s.Root, Tags: []string{"runtime", "webhook", "polling"}})
	}
	if hasWebhook && !strings.Contains(all, "https://") && !strings.Contains(all, "tls") {
		out = append(out, Finding{ID: "bot.webhook.no_tls_hint", Severity: core.SeverityWarn, Title: "Webhook без явного TLS-контекста", Message: "Для webhook-режима нужен HTTPS/TLS и корректный reverse proxy.", Path: s.Root, Tags: []string{"webhook", "tls"}})
	}
	return out
}

func analyzeRuntime(s snapshot) []Finding {
	all := strings.ToLower(joinText(s))
	out := []Finding{}
	if !strings.Contains(all, "rate") && !strings.Contains(all, "throttle") && !strings.Contains(all, "limiter") {
		out = append(out, Finding{ID: "bot.runtime.no_rate_limit", Severity: core.SeverityWarn, Title: "Не найден rate limit/backoff", Message: "Для ботов нужен контроль частоты запросов и обработка ограничений API платформы.", Path: s.Root, Tags: []string{"runtime", "rate-limit"}})
	}
	if !strings.Contains(all, "log") && !strings.Contains(all, "logger") && !strings.Contains(all, "zap") && !strings.Contains(all, "slog") {
		out = append(out, Finding{ID: "bot.runtime.no_logging", Severity: core.SeverityWarn, Title: "Не найден logging-контур", Message: "Bot-проект должен иметь явное логирование ошибок, входящих событий и lifecycle-состояний.", Path: s.Root, Tags: []string{"runtime", "logs"}})
	}
	if !strings.Contains(all, "signal") && !strings.Contains(all, "shutdown") && !strings.Contains(all, "graceful") {
		out = append(out, Finding{ID: "bot.runtime.no_graceful_shutdown", Severity: core.SeverityWarn, Title: "Не найден graceful shutdown", Message: "Для production-бота нужна корректная остановка по SIGTERM/SIGINT.", Path: s.Root, Tags: []string{"runtime", "shutdown"}})
	}
	return out
}

func analyzeBotHost(s snapshot) []Finding {
	out := []Finding{}
	if !hasDocker(s) && !hasSystemd(s) && !hasAny(s, "Procfile", "bot.json", "app.json") {
		out = append(out, Finding{ID: "bot.deploy.no_runtime_descriptor", Severity: core.SeverityWarn, Title: "Нет descriptor-файла запуска", Message: "Не найден Dockerfile, systemd unit, Procfile или похожее описание запуска. Для BotHost/VDS-деплоя нужен воспроизводимый runtime.", Path: s.Root, Tags: []string{"deploy", "bothost"}})
	}
	if hasDocker(s) && !fileContains(s, "Dockerfile", "HEALTHCHECK") {
		out = append(out, Finding{ID: "bot.docker.no_healthcheck", Severity: core.SeverityWarn, Title: "Dockerfile без HEALTHCHECK", Message: "Для контейнерного bot-деплоя полезен HEALTHCHECK или внешний health endpoint.", Path: "Dockerfile", Tags: []string{"docker", "health"}})
	}
	if hasSystemd(s) && !systemdHasRestart(s) {
		out = append(out, Finding{ID: "bot.systemd.no_restart", Severity: core.SeverityWarn, Title: "systemd unit без Restart", Message: "Для bot-сервиса рекомендуется Restart=on-failure или аналогичная политика перезапуска.", Path: s.Root, Tags: []string{"systemd", "runtime"}})
	}
	return out
}

func platformsFromText(text string) []string {
	items := []string{}
	if strings.Contains(text, "telegram") || strings.Contains(text, "tgbot") || strings.Contains(text, "telegraf") || strings.Contains(text, "aiogram") || strings.Contains(text, "telebot") {
		items = append(items, "telegram")
	}
	if strings.Contains(text, "discord") || strings.Contains(text, "discord.js") || strings.Contains(text, "discordgo") || strings.Contains(text, "discord.py") || strings.Contains(text, "dsharp") {
		items = append(items, "discord")
	}
	if strings.Contains(text, "vk_api") || strings.Contains(text, "vkontakte") || strings.Contains(text, "vk-io") || strings.Contains(text, "vkbot") || strings.Contains(text, "longpoll") {
		items = append(items, "vk")
	}
	return unique(items)
}

func mentionsPlatform(s snapshot, files []BotFile, platform string) bool {
	for _, f := range files {
		for _, p := range f.Platform {
			if p == platform {
				return true
			}
		}
	}
	return strings.Contains(strings.ToLower(joinText(s)), platform)
}

func envExampleHas(s snapshot, key string) bool {
	for path, text := range s.Text {
		base := strings.ToLower(filepath.Base(path))
		if strings.Contains(base, "env") && (strings.Contains(base, "example") || strings.Contains(base, "sample")) && strings.Contains(text, key) {
			return true
		}
	}
	return false
}

func hasAny(s snapshot, names ...string) bool {
	for _, n := range names {
		if s.FileSet[filepath.ToSlash(n)] {
			return true
		}
		for f := range s.FileSet {
			if strings.EqualFold(filepath.Base(f), n) {
				return true
			}
		}
	}
	return false
}

func hasDocker(s snapshot) bool { return hasAny(s, "Dockerfile", "docker-compose.yml", "compose.yml") }
func hasSystemd(s snapshot) bool {
	for f := range s.FileSet {
		if strings.HasSuffix(strings.ToLower(f), ".service") {
			return true
		}
	}
	return false
}

func fileContains(s snapshot, base, needle string) bool {
	for path, text := range s.Text {
		if strings.EqualFold(filepath.Base(path), base) && strings.Contains(strings.ToLower(text), strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func systemdHasRestart(s snapshot) bool {
	for path, text := range s.Text {
		if strings.HasSuffix(strings.ToLower(path), ".service") && strings.Contains(strings.ToLower(text), "restart=") {
			return true
		}
	}
	return false
}

func joinText(s snapshot) string {
	var b strings.Builder
	for path, text := range s.Text {
		b.WriteString(path)
		b.WriteByte('\n')
		b.WriteString(text)
		b.WriteByte('\n')
	}
	return b.String()
}

func looksLikeSecret(text string) bool {
	low := strings.ToLower(text)
	return strings.Contains(low, "token=") || strings.Contains(low, "bot_token=") || strings.Contains(low, "secret=") || regexp.MustCompile(`\b\d{6,12}:[A-Za-z0-9_-]{30,}\b`).FindString(text) != ""
}

func isAllowedExample(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	return strings.Contains(base, "example") || strings.Contains(base, "sample") || strings.Contains(base, "template")
}

func safeID(path string) string {
	v := strings.ToLower(path)
	v = regexp.MustCompile(`[^a-z0-9а-яё]+`).ReplaceAllString(v, ".")
	v = strings.Trim(v, ".")
	if v == "" {
		return "file"
	}
	return v
}

func unique(items []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

func dedupe(in []Finding) []Finding {
	seen := map[string]bool{}
	out := []Finding{}
	for _, f := range in {
		key := f.ID + "|" + f.Path
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, f)
	}
	return out
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

func sum(findings []Finding) map[string]int {
	out := map[string]int{"critical": 0, "danger": 0, "warn": 0, "info": 0}
	for _, f := range findings {
		out[string(f.Severity)]++
	}
	return out
}

func write(report Report, stdout io.Writer, opts Options) error {
	if opts.JSON || opts.Output != "" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		data = append(data, '\n')
		if opts.Output == "" {
			_, err = stdout.Write(data)
			return err
		}
		if err := os.MkdirAll(filepath.Dir(opts.Output), 0755); err != nil && filepath.Dir(opts.Output) != "." {
			return err
		}
		if err := os.WriteFile(opts.Output, data, 0644); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Отчёт сохранён: %s\n", opts.Output)
		return nil
	}
	fmt.Fprintf(stdout, "%s %s: команда %s, цель %s\n", report.Tool, report.Version, report.Command, report.Target)
	fmt.Fprintf(stdout, "Итог: critical=%d, danger=%d, warn=%d, info=%d\n", report.Summary["critical"], report.Summary["danger"], report.Summary["warn"], report.Summary["info"])
	for _, f := range report.Findings {
		fmt.Fprintf(stdout, "[%s] %s — %s", f.Severity, f.Title, f.Message)
		if strings.TrimSpace(f.Path) != "" {
			fmt.Fprintf(stdout, " (%s)", f.Path)
		}
		fmt.Fprintln(stdout)
	}
	return nil
}

func scanLines(text string, fn func(int, string)) {
	s := bufio.NewScanner(strings.NewReader(text))
	line := 1
	for s.Scan() {
		fn(line, s.Text())
		line++
	}
}
