package logdoctor

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gitflic.ru/skif4er/doctortools/internal/core"
	"gitflic.ru/skif4er/doctortools/internal/version"
)

type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

type Options struct {
	JSON      bool
	Output    string
	Level     string
	MaxLines  int
	Recursive bool
	Since     string
}

type LogFileReport struct {
	Path          string            `json:"path"`
	LinesScanned  int               `json:"lines_scanned"`
	Errors        int               `json:"errors"`
	Warnings      int               `json:"warnings"`
	Fatal         int               `json:"fatal"`
	Panics        int               `json:"panics"`
	Exceptions    int               `json:"exceptions"`
	OOM           int               `json:"oom"`
	Permission    int               `json:"permission_denied"`
	Database      int               `json:"database_errors"`
	Network       int               `json:"network_errors"`
	StackTraces   int               `json:"stack_traces"`
	Samples       []LogSample       `json:"samples"`
	Groups        map[string]int    `json:"groups"`
	FirstSeenLine int               `json:"first_seen_line,omitempty"`
	LastSeenLine  int               `json:"last_seen_line,omitempty"`
	Meta          map[string]string `json:"meta,omitempty"`
}

type LogSample struct {
	Line    int    `json:"line"`
	Level   string `json:"level"`
	Pattern string `json:"pattern"`
	Text    string `json:"text"`
}

type FullReport struct {
	Tool       string            `json:"tool"`
	Version    string            `json:"tool_version"`
	Platform   string            `json:"platform"`
	Command    string            `json:"command"`
	Target     string            `json:"target"`
	StartedAt  string            `json:"started_at"`
	FinishedAt string            `json:"finished_at"`
	Summary    map[string]int    `json:"summary"`
	Files      []LogFileReport   `json:"files"`
	Findings   []core.Finding    `json:"findings"`
	Artifacts  map[string]string `json:"artifacts"`
}

type pattern struct {
	ID       string
	Level    string
	Severity core.Severity
	Re       *regexp.Regexp
	Title    string
	Advice   string
}

var patterns = []pattern{
	{"fatal", "fatal", core.SeverityDanger, regexp.MustCompile(`(?i)\b(fatal|critical|crit)\b`), "Обнаружены fatal/critical записи", "Проверьте причину аварийного завершения процесса."},
	{"panic", "panic", core.SeverityDanger, regexp.MustCompile(`(?i)\bpanic\b|thread .* panicked`), "Обнаружены panic-записи", "Проверьте stack trace и последний успешный этап перед падением."},
	{"exception", "exception", core.SeverityWarn, regexp.MustCompile(`(?i)(exception|traceback \(most recent call last\)|stack trace)`), "Обнаружены исключения или stack trace", "Сгруппируйте повторяющиеся stack trace и устраните первичный источник."},
	{"error", "error", core.SeverityWarn, regexp.MustCompile(`(?i)\b(error|err)\b|\[error\]|level=error`), "Обнаружены ошибки", "Проверьте повторяемость ошибок и влияние на работу сервиса."},
	{"warning", "warn", core.SeverityInfo, regexp.MustCompile(`(?i)\b(warn|warning)\b|\[warn\]|level=warn`), "Обнаружены предупреждения", "Проверьте предупреждения, которые повторяются часто."},
	{"oom", "error", core.SeverityDanger, regexp.MustCompile(`(?i)(out of memory|outofmemory|oom|cannot allocate memory|killed process)`), "Обнаружены признаки нехватки памяти", "Проверьте лимиты памяти, JVM/Node/PHP параметры и OOM killer."},
	{"permission", "error", core.SeverityWarn, regexp.MustCompile(`(?i)(permission denied|access denied|forbidden|eacces|operation not permitted)`), "Обнаружены ошибки доступа", "Проверьте владельца файлов, права директорий, SELinux/AppArmor и пользователя процесса."},
	{"database", "error", core.SeverityWarn, regexp.MustCompile(`(?i)(sqlstate|database|postgres|mysql|mariadb|sqlite|connection refused|too many connections|deadlock)`), "Обнаружены ошибки базы данных", "Проверьте подключение, миграции, лимиты соединений и блокировки."},
	{"network", "error", core.SeverityWarn, regexp.MustCompile(`(?i)(connection refused|connection reset|timeout|timed out|no route to host|dns|tls handshake|certificate)`), "Обнаружены сетевые ошибки", "Проверьте DNS, firewall, TLS, upstream и таймауты."},
}

func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || isHelp(args[0]) {
		printHelp(stdout)
		return nil
	}
	switch args[0] {
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "LogDoctor %s\nПлатформа: %s %s\nСтатус: %s\n", version.Products["log"].Version, version.PlatformName, version.PlatformVersion, version.Products["log"].Status)
		return nil
	case "analyze", "scan", "summarize", "root-cause", "grep":
		return run(args[0], args[1:], stdout)
	default:
		return usageError{"неизвестная команда LogDoctor: " + args[0]}
	}
}

func printHelp(w io.Writer) {
	fmt.Fprintf(w, `LogDoctor %s
Универсальный анализ логов для DoctorTools.

Использование:
  logdoctor analyze <путь> [--json] [--output файл]
  logdoctor summarize <путь> [--json]
  logdoctor root-cause <путь> [--json]
  logdoctor grep <путь> --level error|warn|fatal [--json]

Поддерживаемые файлы:
  .log, .txt, .out, .err

Флаги:
  --json              вывести JSON-отчёт
  --output, -o файл   сохранить JSON-отчёт в файл
  --max-lines N       ограничить чтение каждого файла, по умолчанию 50000
  --level LEVEL       фильтр для grep: error, warn, fatal, panic, exception
`, version.Products["log"].Version)
}

func run(command string, args []string, stdout io.Writer) error {
	target, opts, err := parseArgs(args)
	if err != nil {
		return err
	}
	report, err := Analyze(command, target, opts)
	if err != nil {
		return err
	}
	if opts.JSON || opts.Output != "" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		data = append(data, '\n')
		if opts.Output != "" {
			if err := os.MkdirAll(filepath.Dir(opts.Output), 0755); err != nil && filepath.Dir(opts.Output) != "." {
				return err
			}
			if err := os.WriteFile(opts.Output, data, 0644); err != nil {
				return err
			}
			fmt.Fprintf(stdout, "Отчёт LogDoctor сохранён: %s\n", opts.Output)
			return nil
		}
		_, err = stdout.Write(data)
		return err
	}
	writeText(report, stdout)
	return nil
}

func parseArgs(args []string) (string, Options, error) {
	opts := Options{MaxLines: 50000, Recursive: true}
	target := "."
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			opts.JSON = true
		case arg == "--output" || arg == "-o":
			if i+1 >= len(args) {
				return target, opts, usageError{"после " + arg + " нужно указать файл"}
			}
			opts.Output = args[i+1]
			i++
		case strings.HasPrefix(arg, "--output="):
			opts.Output = strings.TrimPrefix(arg, "--output=")
		case arg == "--level":
			if i+1 >= len(args) {
				return target, opts, usageError{"после --level нужно указать уровень"}
			}
			opts.Level = strings.ToLower(args[i+1])
			i++
		case strings.HasPrefix(arg, "--level="):
			opts.Level = strings.ToLower(strings.TrimPrefix(arg, "--level="))
		case arg == "--max-lines":
			if i+1 >= len(args) {
				return target, opts, usageError{"после --max-lines нужно указать число"}
			}
			v, err := strconv.Atoi(args[i+1])
			if err != nil || v <= 0 {
				return target, opts, usageError{"--max-lines должен быть положительным числом"}
			}
			opts.MaxLines = v
			i++
		case strings.HasPrefix(arg, "--max-lines="):
			v, err := strconv.Atoi(strings.TrimPrefix(arg, "--max-lines="))
			if err != nil || v <= 0 {
				return target, opts, usageError{"--max-lines должен быть положительным числом"}
			}
			opts.MaxLines = v
		case strings.HasPrefix(arg, "-"):
			return target, opts, usageError{"неизвестный флаг: " + arg}
		default:
			target = arg
		}
	}
	return target, opts, nil
}

func Analyze(command, target string, opts Options) (FullReport, error) {
	started := time.Now().UTC()
	files, err := discoverLogFiles(target)
	if err != nil {
		return FullReport{}, err
	}
	reports := make([]LogFileReport, 0, len(files))
	findings := []core.Finding{}
	if len(files) == 0 {
		findings = append(findings, core.Finding{ID: "log.files.missing", Severity: core.SeverityInfo, Title: "Логи не найдены", Message: "Не обнаружены .log, .txt, .out или .err файлы.", Path: target, Tags: []string{"logs"}})
	}
	for _, file := range files {
		r, err := analyzeFile(file, opts)
		if err != nil {
			findings = append(findings, core.Finding{ID: "log.file.unreadable", Severity: core.SeverityWarn, Title: "Лог не удалось прочитать", Message: err.Error(), Path: file, Tags: []string{"logs", "io"}})
			continue
		}
		reports = append(reports, r)
		findings = append(findings, findingsForFile(r)...)
	}
	findings = filterFindings(command, opts.Level, findings)
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Severity == findings[j].Severity {
			return findings[i].ID < findings[j].ID
		}
		return severityRank(findings[i].Severity) < severityRank(findings[j].Severity)
	})
	return FullReport{
		Tool:       "LogDoctor",
		Version:    version.Products["log"].Version,
		Platform:   version.PlatformName,
		Command:    command,
		Target:     target,
		StartedAt:  started.Format(time.RFC3339),
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
		Summary:    core.Summarize(findings),
		Files:      reports,
		Findings:   findings,
		Artifacts: map[string]string{
			"engine":             "LogDoctor analyzer",
			"engine_version":     version.Products["log"].Version,
			"supported_ext":      ".log,.txt,.out,.err",
			"max_lines_per_file": strconv.Itoa(opts.MaxLines),
		},
	}, nil
}

func discoverLogFiles(target string) ([]string, error) {
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if isLogFile(target) {
			return []string{target}, nil
		}
		return nil, nil
	}
	files := []string{}
	err = filepath.WalkDir(target, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := strings.ToLower(d.Name())
			if base == ".git" || base == "node_modules" || base == "vendor" || base == "dist" || base == "bin" {
				return filepath.SkipDir
			}
			return nil
		}
		if isLogFile(path) {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func isLogFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".log", ".txt", ".out", ".err":
		return true
	default:
		return false
	}
}

func analyzeFile(path string, opts Options) (LogFileReport, error) {
	file, err := os.Open(path)
	if err != nil {
		return LogFileReport{}, err
	}
	defer file.Close()
	report := LogFileReport{Path: path, Samples: []LogSample{}, Groups: map[string]int{}, Meta: map[string]string{}}
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		text := scanner.Text()
		matched := false
		for _, p := range patterns {
			if opts.Level != "" && p.Level != opts.Level && p.ID != opts.Level {
				continue
			}
			if p.Re.MatchString(text) {
				matched = true
				report.Groups[p.ID]++
				countPattern(&report, p.ID)
				if len(report.Samples) < 30 {
					report.Samples = append(report.Samples, LogSample{Line: lineNo, Level: p.Level, Pattern: p.ID, Text: trimSample(text)})
				}
			}
		}
		if matched {
			if report.FirstSeenLine == 0 {
				report.FirstSeenLine = lineNo
			}
			report.LastSeenLine = lineNo
		}
		report.LinesScanned = lineNo
		if opts.MaxLines > 0 && lineNo >= opts.MaxLines {
			report.Meta["truncated"] = "true"
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return report, err
	}
	return report, nil
}

func countPattern(r *LogFileReport, id string) {
	switch id {
	case "fatal":
		r.Fatal++
	case "panic":
		r.Panics++
	case "exception":
		r.Exceptions++
		r.StackTraces++
	case "error":
		r.Errors++
	case "warning":
		r.Warnings++
	case "oom":
		r.OOM++
		r.Errors++
	case "permission":
		r.Permission++
		r.Errors++
	case "database":
		r.Database++
		r.Errors++
	case "network":
		r.Network++
		r.Errors++
	}
}

func findingsForFile(r LogFileReport) []core.Finding {
	findings := []core.Finding{}
	add := func(id string, sev core.Severity, title, message string, tags ...string) {
		findings = append(findings, core.Finding{ID: id, Severity: sev, Title: title, Message: message, Path: r.Path, Tags: tags})
	}
	if r.Fatal > 0 {
		add("log.fatal.detected", core.SeverityDanger, "Обнаружены fatal/critical записи", fmt.Sprintf("Найдено fatal/critical строк: %d.", r.Fatal), "logs", "fatal")
	}
	if r.Panics > 0 {
		add("log.panic.detected", core.SeverityDanger, "Обнаружены panic-записи", fmt.Sprintf("Найдено panic строк: %d.", r.Panics), "logs", "panic")
	}
	if r.OOM > 0 {
		add("log.oom.detected", core.SeverityDanger, "Обнаружены признаки нехватки памяти", fmt.Sprintf("Найдено OOM/memory строк: %d.", r.OOM), "logs", "memory")
	}
	if r.Exceptions > 0 || r.StackTraces > 0 {
		add("log.exception.detected", core.SeverityWarn, "Обнаружены exception/stack trace признаки", fmt.Sprintf("Найдено exception/stack trace строк: %d.", r.Exceptions+r.StackTraces), "logs", "stacktrace")
	}
	if r.Errors > 0 {
		add("log.error.detected", core.SeverityWarn, "Обнаружены ошибки в логах", fmt.Sprintf("Найдено error/permission/database/network строк: %d.", r.Errors), "logs", "error")
	}
	if r.Warnings > 50 {
		add("log.warning.noisy", core.SeverityInfo, "Много предупреждений", fmt.Sprintf("Найдено warning/warn строк: %d.", r.Warnings), "logs", "warn")
	}
	if r.LinesScanned == 0 {
		add("log.file.empty", core.SeverityInfo, "Файл лога пуст", "Лог-файл не содержит строк.", "logs")
	}
	return findings
}

func filterFindings(command, level string, findings []core.Finding) []core.Finding {
	if command != "root-cause" && command != "grep" {
		return findings
	}
	filtered := []core.Finding{}
	for _, f := range findings {
		if command == "root-cause" && (f.Severity == core.SeverityDanger || strings.Contains(strings.Join(f.Tags, ","), "error")) {
			filtered = append(filtered, f)
			continue
		}
		if command == "grep" {
			tags := strings.Join(f.Tags, ",")
			if level == "" || strings.Contains(tags, level) || strings.Contains(f.ID, level) {
				filtered = append(filtered, f)
			}
		}
	}
	return filtered
}

func writeText(report FullReport, w io.Writer) {
	fmt.Fprintf(w, "%s %s: %s %s\n", report.Tool, report.Version, report.Command, report.Target)
	fmt.Fprintf(w, "Файлов: %d. Итог: critical=%d, danger=%d, warn=%d, info=%d\n", len(report.Files), report.Summary["critical"], report.Summary["danger"], report.Summary["warn"], report.Summary["info"])
	for _, f := range report.Files {
		fmt.Fprintf(w, "\n%s\n", f.Path)
		fmt.Fprintf(w, "  строки=%d error=%d warn=%d fatal=%d panic=%d exception=%d oom=%d permission=%d database=%d network=%d\n", f.LinesScanned, f.Errors, f.Warnings, f.Fatal, f.Panics, f.Exceptions, f.OOM, f.Permission, f.Database, f.Network)
		for _, s := range f.Samples {
			fmt.Fprintf(w, "  L%d [%s/%s] %s\n", s.Line, s.Level, s.Pattern, s.Text)
		}
	}
	if len(report.Findings) > 0 {
		fmt.Fprintln(w, "\nНаходки:")
		for _, f := range report.Findings {
			fmt.Fprintf(w, "[%s] %s — %s (%s)\n", f.Severity, f.Title, f.Message, f.Path)
		}
	}
}

func trimSample(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 220 {
		return s
	}
	return s[:217] + "..."
}

func severityRank(s core.Severity) int {
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

func isHelp(s string) bool { return s == "help" || s == "--help" || s == "-h" }
