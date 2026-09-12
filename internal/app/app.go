package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gitflic.ru/skif4er/doctortools/internal/analyzers/config"
	"gitflic.ru/skif4er/doctortools/internal/analyzers/craftadvanced"
	"gitflic.ru/skif4er/doctortools/internal/analyzers/logs"
	"gitflic.ru/skif4er/doctortools/internal/analyzers/performance"
	"gitflic.ru/skif4er/doctortools/internal/analyzers/plugins"
	"gitflic.ru/skif4er/doctortools/internal/analyzers/production"
	"gitflic.ru/skif4er/doctortools/internal/analyzers/proxy"
	"gitflic.ru/skif4er/doctortools/internal/analyzers/security"
	"gitflic.ru/skif4er/doctortools/internal/model"
	"gitflic.ru/skif4er/doctortools/internal/report"
	"gitflic.ru/skif4er/doctortools/internal/rules"
	"gitflic.ru/skif4er/doctortools/internal/scanner"
	"gitflic.ru/skif4er/doctortools/internal/schema"
	"gitflic.ru/skif4er/doctortools/internal/version"
)

type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

type qualityGateError struct{ msg string }

func (e qualityGateError) Error() string { return e.msg }

const (
	ExitOK          = 0
	ExitRuntime     = 1
	ExitQualityGate = 2
	ExitUsage       = 64
)

func ExitCode(err error) int {
	var u usageError
	if errors.As(err, &u) {
		return ExitUsage
	}
	var q qualityGateError
	if errors.As(err, &q) {
		return ExitQualityGate
	}
	return ExitRuntime
}

func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printHelp(stdout)
		return nil
	}
	switch args[0] {
	case "help", "--help", "-h":
		printHelp(stdout)
		return nil
	case "version", "--version", "-v":
		return runVersion(args[1:], stdout)
	case "schema":
		return runSchema(args[1:], stdout)
	case "ci":
		return runCI(args[1:], stdout, stderr)
	case "scan":
		return runScan(args[1:], stdout, stderr)
	case "rules":
		return runRules(args[1:], stdout)
	case "config":
		return runConfig(args[1:], stdout)
	case "plugins":
		return runPlugins(args[1:], stdout)
	case "logs":
		return runLogs(args[1:], stdout)
	case "performance":
		return runPerformance(args[1:], stdout)
	case "java":
		return runJava(args[1:], stdout)
	case "world":
		return runWorld(args[1:], stdout)
	case "proxy":
		return runProxy(args[1:], stdout)
	case "security":
		return runSecurity(args[1:], stdout)
	case "production":
		return runProduction(args[1:], stdout)
	default:
		return usageError{"неизвестная команда: " + args[0]}
	}
}

func runVersion(args []string, stdout io.Writer) error {
	format := "text"
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			format = "json"
		case arg == "--format" || arg == "-f":
			if i+1 >= len(args) {
				return usageError{"после " + arg + " нужно указать формат"}
			}
			format = args[i+1]
			i++
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
		default:
			return usageError{"неизвестный флаг version: " + arg}
		}
	}
	meta := struct {
		Name       string         `json:"name"`
		Version    string         `json:"version"`
		Author     string         `json:"author"`
		License    string         `json:"license"`
		ModulePath string         `json:"module_path"`
		Wiki       string         `json:"wiki"`
		Schema     string         `json:"report_schema"`
		ExitCodes  map[string]int `json:"exit_codes"`
	}{
		Name:       version.Name,
		Version:    version.Version,
		Author:     version.Author,
		License:    version.License,
		ModulePath: version.ModulePath,
		Wiki:       version.Wiki,
		Schema:     "schemas/report.schema.json",
		ExitCodes:  map[string]int{"ok": ExitOK, "runtime_error": ExitRuntime, "quality_gate_failed": ExitQualityGate, "usage_error": ExitUsage},
	}
	if strings.EqualFold(format, "json") {
		data, err := json.MarshalIndent(meta, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(data))
		return err
	}
	if !strings.EqualFold(format, "text") && format != "" {
		return usageError{"неподдерживаемый формат version: " + format}
	}
	fmt.Fprintf(stdout, "%s %s\nАвтор: %s\nЛицензия: %s\nМодуль: %s\nДокументация: %s\nJSON Schema: %s\nExit codes: 0=OK, 1=ошибка выполнения, 2=quality gate, 64=ошибка использования\n", version.Name, version.Version, version.Author, version.License, version.ModulePath, version.Wiki, "schemas/report.schema.json")
	return nil
}

func runSchema(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printSchemaHelp(stdout)
		return nil
	}
	if args[0] != "report" {
		return usageError{"неизвестная схема: " + args[0]}
	}
	output := ""
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--output" || arg == "-o":
			if i+1 >= len(args) {
				return usageError{"после " + arg + " нужно указать путь файла"}
			}
			output = args[i+1]
			i++
		case strings.HasPrefix(arg, "--output="):
			output = strings.TrimPrefix(arg, "--output=")
		default:
			return usageError{"неизвестный флаг schema report: " + arg}
		}
	}
	data := []byte(schema.ReportJSONSchema + "\n")
	if output == "" {
		_, err := stdout.Write(data)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(normalizeOutput(output)), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(output, data, 0644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "JSON Schema отчёта сохранена: %s\n", output)
	return nil
}

func runCI(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return usageError{"для команды ci нужно указать путь к Minecraft-серверу"}
	}
	target := args[0]
	output := "craftdoctor-ci-report.json"
	toStdout := false
	maxCritical, maxDanger, maxWarn := 0, -1, -1
	opts := scanner.Options{}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--stdout":
			toStdout = true
		case arg == "--output" || arg == "-o":
			if i+1 >= len(args) {
				return usageError{"после " + arg + " нужно указать путь файла"}
			}
			output = args[i+1]
			i++
		case strings.HasPrefix(arg, "--output="):
			output = strings.TrimPrefix(arg, "--output=")
		case arg == "--max-critical":
			if i+1 >= len(args) {
				return usageError{"после --max-critical нужно указать число"}
			}
			v, err := strconv.Atoi(args[i+1])
			if err != nil {
				return usageError{"--max-critical должен быть числом"}
			}
			maxCritical = v
			i++
		case strings.HasPrefix(arg, "--max-critical="):
			v, err := strconv.Atoi(strings.TrimPrefix(arg, "--max-critical="))
			if err != nil {
				return usageError{"--max-critical должен быть числом"}
			}
			maxCritical = v
		case arg == "--max-danger":
			if i+1 >= len(args) {
				return usageError{"после --max-danger нужно указать число"}
			}
			v, err := strconv.Atoi(args[i+1])
			if err != nil {
				return usageError{"--max-danger должен быть числом"}
			}
			maxDanger = v
			i++
		case strings.HasPrefix(arg, "--max-danger="):
			v, err := strconv.Atoi(strings.TrimPrefix(arg, "--max-danger="))
			if err != nil {
				return usageError{"--max-danger должен быть числом"}
			}
			maxDanger = v
		case arg == "--max-warn":
			if i+1 >= len(args) {
				return usageError{"после --max-warn нужно указать число"}
			}
			v, err := strconv.Atoi(args[i+1])
			if err != nil {
				return usageError{"--max-warn должен быть числом"}
			}
			maxWarn = v
			i++
		case strings.HasPrefix(arg, "--max-warn="):
			v, err := strconv.Atoi(strings.TrimPrefix(arg, "--max-warn="))
			if err != nil {
				return usageError{"--max-warn должен быть числом"}
			}
			maxWarn = v
		case arg == "--no-ignore":
			opts.NoIgnore = true
		case arg == "--players":
			if i+1 >= len(args) {
				return usageError{"после --players нужно указать число игроков"}
			}
			v, err := strconv.Atoi(args[i+1])
			if err != nil || v < 0 {
				return usageError{"--players должен быть неотрицательным числом"}
			}
			opts.PerformancePlayers = v
			i++
		case strings.HasPrefix(arg, "--players="):
			v, err := strconv.Atoi(strings.TrimPrefix(arg, "--players="))
			if err != nil || v < 0 {
				return usageError{"--players должен быть неотрицательным числом"}
			}
			opts.PerformancePlayers = v
		case arg == "--target":
			if i+1 >= len(args) {
				return usageError{"после --target нужно указать тип сервера"}
			}
			opts.PerformanceTarget = args[i+1]
			i++
		case strings.HasPrefix(arg, "--target="):
			opts.PerformanceTarget = strings.TrimPrefix(arg, "--target=")
		case arg == "--systemd-dir":
			if i+1 >= len(args) {
				return usageError{"после --systemd-dir нужно указать путь"}
			}
			opts.ProductionSystemdDir = args[i+1]
			i++
		case strings.HasPrefix(arg, "--systemd-dir="):
			opts.ProductionSystemdDir = strings.TrimPrefix(arg, "--systemd-dir=")
		case arg == "--logrotate-dir":
			if i+1 >= len(args) {
				return usageError{"после --logrotate-dir нужно указать путь"}
			}
			opts.ProductionLogrotateDir = args[i+1]
			i++
		case strings.HasPrefix(arg, "--logrotate-dir="):
			opts.ProductionLogrotateDir = strings.TrimPrefix(arg, "--logrotate-dir=")
		case arg == "--backup-max-age":
			if i+1 >= len(args) {
				return usageError{"после --backup-max-age нужно указать интервал"}
			}
			opts.ProductionBackupMaxAge = args[i+1]
			i++
		case strings.HasPrefix(arg, "--backup-max-age="):
			opts.ProductionBackupMaxAge = strings.TrimPrefix(arg, "--backup-max-age=")
		case arg == "--ignore-file":
			if i+1 >= len(args) {
				return usageError{"после --ignore-file нужно указать путь файла"}
			}
			opts.IgnoreFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--ignore-file="):
			opts.IgnoreFile = strings.TrimPrefix(arg, "--ignore-file=")
		case arg == "--rules-file":
			if i+1 >= len(args) {
				return usageError{"после --rules-file нужно указать путь JSON-файла правил"}
			}
			opts.RulesFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--rules-file="):
			opts.RulesFile = strings.TrimPrefix(arg, "--rules-file=")
		default:
			return usageError{"неизвестный флаг ci: " + arg}
		}
	}
	rep, err := scanner.ScanWithOptions(target, opts)
	if err != nil {
		return err
	}
	data, _, err := report.Render(rep, "json")
	if err != nil {
		return err
	}
	if toStdout {
		_, err = stdout.Write(data)
		if err == nil {
			_, _ = fmt.Fprintln(stdout)
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(normalizeOutput(output)), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(output, data, 0644); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "CI-отчёт создан: %s\n", output)
	}
	fmt.Fprintf(stderr, "CI: CRITICAL=%d/%d · DANGER=%d/%s · WARN=%d/%s · INFO=%d\n", rep.Summary.Critical, maxCritical, rep.Summary.Danger, limitText(maxDanger), rep.Summary.Warn, limitText(maxWarn), rep.Summary.Info)
	if maxCritical >= 0 && rep.Summary.Critical > maxCritical {
		return qualityGateError{fmt.Sprintf("quality gate не пройден: CRITICAL=%d больше лимита %d", rep.Summary.Critical, maxCritical)}
	}
	if maxDanger >= 0 && rep.Summary.Danger > maxDanger {
		return qualityGateError{fmt.Sprintf("quality gate не пройден: DANGER=%d больше лимита %d", rep.Summary.Danger, maxDanger)}
	}
	if maxWarn >= 0 && rep.Summary.Warn > maxWarn {
		return qualityGateError{fmt.Sprintf("quality gate не пройден: WARN=%d больше лимита %d", rep.Summary.Warn, maxWarn)}
	}
	return nil
}

func limitText(v int) string {
	if v < 0 {
		return "∞"
	}
	return strconv.Itoa(v)
}

func runScan(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return usageError{"для команды scan нужно указать путь к Minecraft-серверу"}
	}
	target := args[0]
	format := "html"
	output := ""
	toStdout := false
	failOnCritical := false
	opts := scanner.Options{}

	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--stdout":
			toStdout = true
		case arg == "--fail-on-critical":
			failOnCritical = true
		case arg == "--no-ignore":
			opts.NoIgnore = true
		case arg == "--players":
			if i+1 >= len(args) {
				return usageError{"после --players нужно указать число игроков"}
			}
			v, err := strconv.Atoi(args[i+1])
			if err != nil || v < 0 {
				return usageError{"--players должен быть неотрицательным числом"}
			}
			opts.PerformancePlayers = v
			i++
		case strings.HasPrefix(arg, "--players="):
			v, err := strconv.Atoi(strings.TrimPrefix(arg, "--players="))
			if err != nil || v < 0 {
				return usageError{"--players должен быть неотрицательным числом"}
			}
			opts.PerformancePlayers = v
		case arg == "--target":
			if i+1 >= len(args) {
				return usageError{"после --target нужно указать тип сервера"}
			}
			opts.PerformanceTarget = args[i+1]
			i++
		case strings.HasPrefix(arg, "--target="):
			opts.PerformanceTarget = strings.TrimPrefix(arg, "--target=")
		case arg == "--systemd-dir":
			if i+1 >= len(args) {
				return usageError{"после --systemd-dir нужно указать путь"}
			}
			opts.ProductionSystemdDir = args[i+1]
			i++
		case strings.HasPrefix(arg, "--systemd-dir="):
			opts.ProductionSystemdDir = strings.TrimPrefix(arg, "--systemd-dir=")
		case arg == "--logrotate-dir":
			if i+1 >= len(args) {
				return usageError{"после --logrotate-dir нужно указать путь"}
			}
			opts.ProductionLogrotateDir = args[i+1]
			i++
		case strings.HasPrefix(arg, "--logrotate-dir="):
			opts.ProductionLogrotateDir = strings.TrimPrefix(arg, "--logrotate-dir=")
		case arg == "--backup-max-age":
			if i+1 >= len(args) {
				return usageError{"после --backup-max-age нужно указать интервал"}
			}
			opts.ProductionBackupMaxAge = args[i+1]
			i++
		case strings.HasPrefix(arg, "--backup-max-age="):
			opts.ProductionBackupMaxAge = strings.TrimPrefix(arg, "--backup-max-age=")
		case arg == "--format" || arg == "-f":
			if i+1 >= len(args) {
				return usageError{"после " + arg + " нужно указать формат"}
			}
			format = args[i+1]
			i++
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
		case arg == "--output" || arg == "-o":
			if i+1 >= len(args) {
				return usageError{"после " + arg + " нужно указать путь файла"}
			}
			output = args[i+1]
			i++
		case strings.HasPrefix(arg, "--output="):
			output = strings.TrimPrefix(arg, "--output=")
		case arg == "--ignore-file":
			if i+1 >= len(args) {
				return usageError{"после --ignore-file нужно указать путь файла"}
			}
			opts.IgnoreFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--ignore-file="):
			opts.IgnoreFile = strings.TrimPrefix(arg, "--ignore-file=")
		case arg == "--rules-file":
			if i+1 >= len(args) {
				return usageError{"после --rules-file нужно указать путь JSON-файла правил"}
			}
			opts.RulesFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--rules-file="):
			opts.RulesFile = strings.TrimPrefix(arg, "--rules-file=")
		default:
			return usageError{"неизвестный флаг scan: " + arg}
		}
	}

	rep, err := scanner.ScanWithOptions(target, opts)
	if err != nil {
		return err
	}
	data, ext, err := report.Render(rep, format)
	if err != nil {
		return err
	}

	if toStdout {
		_, err = stdout.Write(data)
		if err == nil {
			_, _ = fmt.Fprintln(stdout)
		}
	} else {
		if output == "" {
			output = "craftdoctor-report." + ext
		}
		if err := os.MkdirAll(filepath.Dir(normalizeOutput(output)), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(output, data, 0644); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Отчёт создан: %s\n", output)
	}
	fmt.Fprintf(stderr, "Статус: %s · CRITICAL: %d · DANGER: %d · WARN: %d · INFO: %d · подавлено: %d\n", rep.Status, rep.Summary.Critical, rep.Summary.Danger, rep.Summary.Warn, rep.Summary.Info, rep.Rules.SuppressedFindings)
	if failOnCritical && rep.Summary.Critical > 0 {
		return qualityGateError{"обнаружены CRITICAL-проблемы"}
	}
	return nil
}

func runConfig(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printConfigHelp(stdout)
		return nil
	}
	switch args[0] {
	case "audit":
		return runConfigAudit(args[1:], stdout)
	default:
		return usageError{"неизвестная подкоманда config: " + args[0]}
	}
}

func runConfigAudit(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError{"для config audit нужно указать путь к Minecraft-серверу"}
	}
	target := args[0]
	format := "text"
	opts := scanner.Options{}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--format" || arg == "-f":
			if i+1 >= len(args) {
				return usageError{"после " + arg + " нужно указать формат"}
			}
			format = args[i+1]
			i++
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
		case arg == "--rules-file":
			if i+1 >= len(args) {
				return usageError{"после --rules-file нужно указать путь JSON-файла правил"}
			}
			opts.RulesFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--rules-file="):
			opts.RulesFile = strings.TrimPrefix(arg, "--rules-file=")
		case arg == "--ignore-file":
			if i+1 >= len(args) {
				return usageError{"после --ignore-file нужно указать путь файла"}
			}
			opts.IgnoreFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--ignore-file="):
			opts.IgnoreFile = strings.TrimPrefix(arg, "--ignore-file=")
		case arg == "--no-ignore":
			opts.NoIgnore = true
		case arg == "--systemd-dir":
			if i+1 >= len(args) {
				return usageError{"после --systemd-dir нужно указать путь"}
			}
			opts.ProductionSystemdDir = args[i+1]
			i++
		case strings.HasPrefix(arg, "--systemd-dir="):
			opts.ProductionSystemdDir = strings.TrimPrefix(arg, "--systemd-dir=")
		case arg == "--logrotate-dir":
			if i+1 >= len(args) {
				return usageError{"после --logrotate-dir нужно указать путь"}
			}
			opts.ProductionLogrotateDir = args[i+1]
			i++
		case strings.HasPrefix(arg, "--logrotate-dir="):
			opts.ProductionLogrotateDir = strings.TrimPrefix(arg, "--logrotate-dir=")
		case arg == "--backup-max-age":
			if i+1 >= len(args) {
				return usageError{"после --backup-max-age нужно указать интервал"}
			}
			opts.ProductionBackupMaxAge = args[i+1]
			i++
		case strings.HasPrefix(arg, "--backup-max-age="):
			opts.ProductionBackupMaxAge = strings.TrimPrefix(arg, "--backup-max-age=")

		default:
			return usageError{"неизвестный флаг config audit: " + arg}
		}
	}
	rep, err := scanner.ScanWithOptions(target, opts)
	if err != nil {
		return err
	}
	configFindings := make([]model.Finding, 0)
	for _, f := range rep.Findings {
		if f.Category == "конфигурация" || strings.HasPrefix(f.ID, "config.") || strings.HasPrefix(f.ID, "minecraft.server_properties") || strings.HasPrefix(f.ID, "minecraft.eula") {
			configFindings = append(configFindings, f)
		}
	}
	if strings.EqualFold(format, "json") {
		data, err := json.MarshalIndent(struct {
			ToolVersion string           `json:"tool_version"`
			TargetPath  string           `json:"target_path"`
			Config      model.ConfigInfo `json:"config"`
			Findings    []model.Finding  `json:"findings"`
		}{ToolVersion: version.Version, TargetPath: rep.TargetPath, Config: rep.Config, Findings: configFindings}, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(data))
		return err
	}
	if !strings.EqualFold(format, "text") && format != "" {
		return usageError{"неподдерживаемый формат config audit: " + format}
	}
	_, err = fmt.Fprint(stdout, config.FormatAnalysisText(rep.Config, configFindings))
	return err
}

func runPlugins(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printPluginsHelp(stdout)
		return nil
	}
	switch args[0] {
	case "audit":
		return runPluginsAudit(args[1:], stdout)
	case "graph":
		return runPluginsGraph(args[1:], stdout)
	case "explain":
		return runPluginsExplain(args[1:], stdout)
	default:
		return usageError{"неизвестная подкоманда plugins: " + args[0]}
	}
}

func runPluginsAudit(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError{"для plugins audit нужно указать путь к Minecraft-серверу"}
	}
	target := args[0]
	format := "text"
	rulesFile := ""
	ignoreFile := ""
	noIgnore := false
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--format" || arg == "-f":
			if i+1 >= len(args) {
				return usageError{"после " + arg + " нужно указать формат"}
			}
			format = args[i+1]
			i++
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
		case arg == "--rules-file":
			if i+1 >= len(args) {
				return usageError{"после --rules-file нужно указать путь JSON-файла правил"}
			}
			rulesFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--rules-file="):
			rulesFile = strings.TrimPrefix(arg, "--rules-file=")
		case arg == "--ignore-file":
			if i+1 >= len(args) {
				return usageError{"после --ignore-file нужно указать путь файла"}
			}
			ignoreFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--ignore-file="):
			ignoreFile = strings.TrimPrefix(arg, "--ignore-file=")
		case arg == "--no-ignore":
			noIgnore = true
		default:
			return usageError{"неизвестный флаг plugins audit: " + arg}
		}
	}
	rep, err := scanner.ScanWithOptions(target, scanner.Options{RulesFile: rulesFile, IgnoreFile: ignoreFile, NoIgnore: noIgnore})
	if err != nil {
		return err
	}
	pluginFindings := make([]model.Finding, 0)
	for _, f := range rep.Findings {
		if f.Category == "плагины" || strings.HasPrefix(f.ID, "plugins.") {
			pluginFindings = append(pluginFindings, f)
		}
	}
	result := struct {
		ToolVersion string                `json:"tool_version"`
		TargetPath  string                `json:"target_path"`
		Audit       model.PluginAuditInfo `json:"audit"`
		Plugins     []model.PluginInfo    `json:"plugins"`
		Findings    []model.Finding       `json:"findings"`
	}{
		ToolVersion: version.Version,
		TargetPath:  rep.TargetPath,
		Audit:       rep.PluginAudit,
		Plugins:     rep.Plugins,
		Findings:    pluginFindings,
	}
	if strings.EqualFold(format, "json") {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(data))
		return err
	}
	if !strings.EqualFold(format, "text") && format != "" {
		return usageError{"неподдерживаемый формат plugins audit: " + format}
	}
	_, err = fmt.Fprint(stdout, plugins.FormatAuditText(rep.PluginAudit, rep.Plugins, rep.Findings))
	return err
}

func runPluginsGraph(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError{"для plugins graph нужно указать путь к Minecraft-серверу"}
	}
	target := args[0]
	format := "text"
	rulesFile := ""
	ignoreFile := ""
	noIgnore := false
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--format" || arg == "-f":
			if i+1 >= len(args) {
				return usageError{"после " + arg + " нужно указать формат"}
			}
			format = args[i+1]
			i++
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
		case arg == "--rules-file":
			if i+1 >= len(args) {
				return usageError{"после --rules-file нужно указать путь JSON-файла правил"}
			}
			rulesFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--rules-file="):
			rulesFile = strings.TrimPrefix(arg, "--rules-file=")
		case arg == "--ignore-file":
			if i+1 >= len(args) {
				return usageError{"после --ignore-file нужно указать путь файла"}
			}
			ignoreFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--ignore-file="):
			ignoreFile = strings.TrimPrefix(arg, "--ignore-file=")
		case arg == "--no-ignore":
			noIgnore = true
		default:
			return usageError{"неизвестный флаг plugins graph: " + arg}
		}
	}
	rep, err := scanner.ScanWithOptions(target, scanner.Options{RulesFile: rulesFile, IgnoreFile: ignoreFile, NoIgnore: noIgnore})
	if err != nil {
		return err
	}
	switch strings.ToLower(format) {
	case "json":
		data, err := json.MarshalIndent(struct {
			ToolVersion string                `json:"tool_version"`
			TargetPath  string                `json:"target_path"`
			Graph       model.PluginGraphInfo `json:"graph"`
		}{ToolVersion: version.Version, TargetPath: rep.TargetPath, Graph: rep.PluginAudit.Graph}, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(data))
		return err
	case "text", "mermaid", "dot", "":
		_, err := fmt.Fprint(stdout, plugins.FormatGraphText(rep.PluginAudit, format))
		return err
	default:
		return usageError{"неподдерживаемый формат plugins graph: " + format}
	}
}

func runPluginsExplain(args []string, stdout io.Writer) error {
	if len(args) < 2 {
		return usageError{"для plugins explain нужно указать путь к Minecraft-серверу и имя плагина"}
	}
	target := args[0]
	pluginName := args[1]
	format := "text"
	rulesFile := ""
	ignoreFile := ""
	noIgnore := false
	for i := 2; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--format" || arg == "-f":
			if i+1 >= len(args) {
				return usageError{"после " + arg + " нужно указать формат"}
			}
			format = args[i+1]
			i++
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
		case arg == "--rules-file":
			if i+1 >= len(args) {
				return usageError{"после --rules-file нужно указать путь JSON-файла правил"}
			}
			rulesFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--rules-file="):
			rulesFile = strings.TrimPrefix(arg, "--rules-file=")
		case arg == "--ignore-file":
			if i+1 >= len(args) {
				return usageError{"после --ignore-file нужно указать путь файла"}
			}
			ignoreFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--ignore-file="):
			ignoreFile = strings.TrimPrefix(arg, "--ignore-file=")
		case arg == "--no-ignore":
			noIgnore = true
		default:
			return usageError{"неизвестный флаг plugins explain: " + arg}
		}
	}
	rep, err := scanner.ScanWithOptions(target, scanner.Options{RulesFile: rulesFile, IgnoreFile: ignoreFile, NoIgnore: noIgnore})
	if err != nil {
		return err
	}
	var selected model.PluginInfo
	found := false
	for _, p := range rep.Plugins {
		if strings.EqualFold(p.Name, pluginName) || strings.EqualFold(strings.TrimSuffix(filepath.Base(p.JarFile), filepath.Ext(p.JarFile)), pluginName) {
			selected = p
			found = true
			break
		}
	}
	if !found {
		return usageError{"плагин не найден: " + pluginName}
	}
	pluginFindings := make([]model.Finding, 0)
	for _, f := range rep.Findings {
		if f.Category == "плагины" || strings.HasPrefix(f.ID, "plugins.") {
			pluginFindings = append(pluginFindings, f)
		}
	}
	if strings.EqualFold(format, "json") {
		data, err := json.MarshalIndent(struct {
			ToolVersion string                `json:"tool_version"`
			TargetPath  string                `json:"target_path"`
			Plugin      model.PluginInfo      `json:"plugin"`
			Audit       model.PluginAuditInfo `json:"audit"`
			Findings    []model.Finding       `json:"findings"`
		}{ToolVersion: version.Version, TargetPath: rep.TargetPath, Plugin: selected, Audit: rep.PluginAudit, Findings: pluginFindings}, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(data))
		return err
	}
	if !strings.EqualFold(format, "text") && format != "" {
		return usageError{"неподдерживаемый формат plugins explain: " + format}
	}
	_, err = fmt.Fprint(stdout, plugins.FormatExplainText(selected, rep.PluginAudit, pluginFindings))
	return err
}

func runLogs(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printLogsHelp(stdout)
		return nil
	}
	switch args[0] {
	case "analyze":
		return runLogsAnalyze(args[1:], stdout)
	default:
		return usageError{"неизвестная подкоманда logs: " + args[0]}
	}
}

func runLogsAnalyze(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError{"для logs analyze нужно указать путь к Minecraft-серверу или файлу latest.log"}
	}
	target := args[0]
	format := "text"
	opts := logs.Options{}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--last-run":
			opts.LastRun = true
		case arg == "--since":
			if i+1 >= len(args) {
				return usageError{"после --since нужно указать интервал, например 24h или 7d"}
			}
			d, err := logs.ParseSinceDuration(args[i+1])
			if err != nil {
				return usageError{err.Error()}
			}
			opts.Since = d
			i++
		case strings.HasPrefix(arg, "--since="):
			d, err := logs.ParseSinceDuration(strings.TrimPrefix(arg, "--since="))
			if err != nil {
				return usageError{err.Error()}
			}
			opts.Since = d
		case arg == "--format" || arg == "-f":
			if i+1 >= len(args) {
				return usageError{"после " + arg + " нужно указать формат"}
			}
			format = args[i+1]
			i++
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
		default:
			return usageError{"неизвестный флаг logs analyze: " + arg}
		}
	}
	info, findings, err := logs.AnalyzeTargetWithOptions(target, opts)
	if err != nil {
		return err
	}
	if strings.EqualFold(format, "json") {
		data, err := json.MarshalIndent(struct {
			ToolVersion string          `json:"tool_version"`
			TargetPath  string          `json:"target_path"`
			Logs        model.LogInfo   `json:"logs"`
			Findings    []model.Finding `json:"findings"`
		}{ToolVersion: version.Version, TargetPath: target, Logs: info, Findings: findings}, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(data))
		return err
	}
	if !strings.EqualFold(format, "text") && format != "" {
		return usageError{"неподдерживаемый формат logs analyze: " + format}
	}
	_, err = fmt.Fprint(stdout, logs.FormatAnalysisText(info, findings))
	return err
}

func runPerformance(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printPerformanceHelp(stdout)
		return nil
	}
	switch args[0] {
	case "scan":
		return runPerformanceScan(args[1:], stdout)
	default:
		return usageError{"неизвестная подкоманда performance: " + args[0]}
	}
}

func runPerformanceScan(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError{"для performance scan нужно указать путь к Minecraft-серверу"}
	}
	target := args[0]
	format := "text"
	rulesFile := ""
	ignoreFile := ""
	noIgnore := false
	players := 0
	targetProfile := ""
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--players":
			if i+1 >= len(args) {
				return usageError{"после --players нужно указать число игроков"}
			}
			v, err := strconv.Atoi(args[i+1])
			if err != nil || v < 0 {
				return usageError{"--players должен быть неотрицательным числом"}
			}
			players = v
			i++
		case strings.HasPrefix(arg, "--players="):
			v, err := strconv.Atoi(strings.TrimPrefix(arg, "--players="))
			if err != nil || v < 0 {
				return usageError{"--players должен быть неотрицательным числом"}
			}
			players = v
		case arg == "--target":
			if i+1 >= len(args) {
				return usageError{"после --target нужно указать тип сервера"}
			}
			targetProfile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--target="):
			targetProfile = strings.TrimPrefix(arg, "--target=")
		case arg == "--format" || arg == "-f":
			if i+1 >= len(args) {
				return usageError{"после " + arg + " нужно указать формат"}
			}
			format = args[i+1]
			i++
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
		case arg == "--rules-file":
			if i+1 >= len(args) {
				return usageError{"после --rules-file нужно указать путь JSON-файла правил"}
			}
			rulesFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--rules-file="):
			rulesFile = strings.TrimPrefix(arg, "--rules-file=")
		case arg == "--ignore-file":
			if i+1 >= len(args) {
				return usageError{"после --ignore-file нужно указать путь файла"}
			}
			ignoreFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--ignore-file="):
			ignoreFile = strings.TrimPrefix(arg, "--ignore-file=")
		case arg == "--no-ignore":
			noIgnore = true
		default:
			return usageError{"неизвестный флаг performance scan: " + arg}
		}
	}
	rep, err := scanner.ScanWithOptions(target, scanner.Options{RulesFile: rulesFile, IgnoreFile: ignoreFile, NoIgnore: noIgnore, PerformancePlayers: players, PerformanceTarget: targetProfile})
	if err != nil {
		return err
	}
	perfFindings := make([]model.Finding, 0)
	for _, f := range rep.Findings {
		if f.Category == "производительность" || strings.HasPrefix(f.ID, "performance.") {
			perfFindings = append(perfFindings, f)
		}
	}
	if strings.EqualFold(format, "json") {
		data, err := json.MarshalIndent(struct {
			ToolVersion string                `json:"tool_version"`
			TargetPath  string                `json:"target_path"`
			Performance model.PerformanceInfo `json:"performance"`
			Findings    []model.Finding       `json:"findings"`
		}{ToolVersion: version.Version, TargetPath: rep.TargetPath, Performance: rep.Performance, Findings: perfFindings}, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(data))
		return err
	}
	if !strings.EqualFold(format, "text") && format != "" {
		return usageError{"неподдерживаемый формат performance scan: " + format}
	}
	_, err = fmt.Fprint(stdout, performance.FormatAnalysisText(rep.Performance, perfFindings))
	return err
}

func runSecurity(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printSecurityHelp(stdout)
		return nil
	}
	switch args[0] {
	case "audit":
		return runSecurityAudit(args[1:], stdout)
	default:
		return usageError{"неизвестная подкоманда security: " + args[0]}
	}
}

func runSecurityAudit(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError{"для security audit нужно указать путь к Minecraft-серверу"}
	}
	target := args[0]
	format := "text"
	rulesFile := ""
	ignoreFile := ""
	noIgnore := false
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--format" || arg == "-f":
			if i+1 >= len(args) {
				return usageError{"после " + arg + " нужно указать формат"}
			}
			format = args[i+1]
			i++
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
		case arg == "--rules-file":
			if i+1 >= len(args) {
				return usageError{"после --rules-file нужно указать путь JSON-файла правил"}
			}
			rulesFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--rules-file="):
			rulesFile = strings.TrimPrefix(arg, "--rules-file=")
		case arg == "--ignore-file":
			if i+1 >= len(args) {
				return usageError{"после --ignore-file нужно указать путь файла"}
			}
			ignoreFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--ignore-file="):
			ignoreFile = strings.TrimPrefix(arg, "--ignore-file=")
		case arg == "--no-ignore":
			noIgnore = true
		default:
			return usageError{"неизвестный флаг security audit: " + arg}
		}
	}
	rep, err := scanner.ScanWithOptions(target, scanner.Options{RulesFile: rulesFile, IgnoreFile: ignoreFile, NoIgnore: noIgnore})
	if err != nil {
		return err
	}
	securityFindings := make([]model.Finding, 0)
	for _, f := range rep.Findings {
		if f.Category == "безопасность" || strings.HasPrefix(f.ID, "security.") {
			securityFindings = append(securityFindings, f)
		}
	}
	if strings.EqualFold(format, "json") {
		data, err := json.MarshalIndent(struct {
			ToolVersion string             `json:"tool_version"`
			TargetPath  string             `json:"target_path"`
			Security    model.SecurityInfo `json:"security"`
			Findings    []model.Finding    `json:"findings"`
		}{ToolVersion: version.Version, TargetPath: rep.TargetPath, Security: rep.Security, Findings: securityFindings}, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(data))
		return err
	}
	if !strings.EqualFold(format, "text") && format != "" {
		return usageError{"неподдерживаемый формат security audit: " + format}
	}
	_, err = fmt.Fprint(stdout, security.FormatAnalysisText(rep.Security, securityFindings))
	return err
}

func runProduction(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printProductionHelp(stdout)
		return nil
	}
	switch args[0] {
	case "check":
		return runProductionCheck(args[1:], stdout)
	case "gate":
		return runProductionGate(args[1:], stdout)
	default:
		return usageError{"неизвестная подкоманда production: " + args[0]}
	}
}

func runProductionCheck(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError{"для production check нужно указать путь к Minecraft-серверу"}
	}
	target := args[0]
	format := "text"
	opts := scanner.Options{}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--format" || arg == "-f":
			if i+1 >= len(args) {
				return usageError{"после " + arg + " нужно указать формат"}
			}
			format = args[i+1]
			i++
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
		case arg == "--rules-file":
			if i+1 >= len(args) {
				return usageError{"после --rules-file нужно указать путь JSON-файла правил"}
			}
			opts.RulesFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--rules-file="):
			opts.RulesFile = strings.TrimPrefix(arg, "--rules-file=")
		case arg == "--ignore-file":
			if i+1 >= len(args) {
				return usageError{"после --ignore-file нужно указать путь файла"}
			}
			opts.IgnoreFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--ignore-file="):
			opts.IgnoreFile = strings.TrimPrefix(arg, "--ignore-file=")
		case arg == "--no-ignore":
			opts.NoIgnore = true
		case arg == "--systemd-dir":
			if i+1 >= len(args) {
				return usageError{"после --systemd-dir нужно указать путь"}
			}
			opts.ProductionSystemdDir = args[i+1]
			i++
		case strings.HasPrefix(arg, "--systemd-dir="):
			opts.ProductionSystemdDir = strings.TrimPrefix(arg, "--systemd-dir=")
		case arg == "--logrotate-dir":
			if i+1 >= len(args) {
				return usageError{"после --logrotate-dir нужно указать путь"}
			}
			opts.ProductionLogrotateDir = args[i+1]
			i++
		case strings.HasPrefix(arg, "--logrotate-dir="):
			opts.ProductionLogrotateDir = strings.TrimPrefix(arg, "--logrotate-dir=")
		case arg == "--backup-max-age":
			if i+1 >= len(args) {
				return usageError{"после --backup-max-age нужно указать интервал"}
			}
			opts.ProductionBackupMaxAge = args[i+1]
			i++
		case strings.HasPrefix(arg, "--backup-max-age="):
			opts.ProductionBackupMaxAge = strings.TrimPrefix(arg, "--backup-max-age=")
		default:
			return usageError{"неизвестный флаг production check: " + arg}
		}
	}
	rep, err := scanner.ScanWithOptions(target, opts)
	if err != nil {
		return err
	}
	productionFindings := make([]model.Finding, 0)
	for _, f := range rep.Findings {
		if f.Category == "production-ready" || strings.HasPrefix(f.ID, "production.") {
			productionFindings = append(productionFindings, f)
		}
	}
	if strings.EqualFold(format, "json") {
		data, err := json.MarshalIndent(struct {
			ToolVersion string               `json:"tool_version"`
			TargetPath  string               `json:"target_path"`
			Production  model.ProductionInfo `json:"production"`
			Findings    []model.Finding      `json:"findings"`
		}{ToolVersion: version.Version, TargetPath: rep.TargetPath, Production: rep.Production, Findings: productionFindings}, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(data))
		return err
	}
	if !strings.EqualFold(format, "text") && format != "" {
		return usageError{"неподдерживаемый формат production check: " + format}
	}
	_, err = fmt.Fprint(stdout, production.FormatAnalysisText(rep.Production, productionFindings))
	return err
}

func runProductionGate(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError{"для production gate нужно указать путь к Minecraft-серверу"}
	}
	target := args[0]
	format := "text"
	opts := scanner.Options{}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--format" || arg == "-f":
			if i+1 >= len(args) {
				return usageError{"после " + arg + " нужно указать формат"}
			}
			format = args[i+1]
			i++
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
		case arg == "--rules-file":
			if i+1 >= len(args) {
				return usageError{"после --rules-file нужно указать путь JSON-файла правил"}
			}
			opts.RulesFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--rules-file="):
			opts.RulesFile = strings.TrimPrefix(arg, "--rules-file=")
		case arg == "--ignore-file":
			if i+1 >= len(args) {
				return usageError{"после --ignore-file нужно указать путь файла"}
			}
			opts.IgnoreFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--ignore-file="):
			opts.IgnoreFile = strings.TrimPrefix(arg, "--ignore-file=")
		case arg == "--no-ignore":
			opts.NoIgnore = true
		case arg == "--systemd-dir":
			if i+1 >= len(args) {
				return usageError{"после --systemd-dir нужно указать путь"}
			}
			opts.ProductionSystemdDir = args[i+1]
			i++
		case strings.HasPrefix(arg, "--systemd-dir="):
			opts.ProductionSystemdDir = strings.TrimPrefix(arg, "--systemd-dir=")
		case arg == "--logrotate-dir":
			if i+1 >= len(args) {
				return usageError{"после --logrotate-dir нужно указать путь"}
			}
			opts.ProductionLogrotateDir = args[i+1]
			i++
		case strings.HasPrefix(arg, "--logrotate-dir="):
			opts.ProductionLogrotateDir = strings.TrimPrefix(arg, "--logrotate-dir=")
		case arg == "--backup-max-age":
			if i+1 >= len(args) {
				return usageError{"после --backup-max-age нужно указать интервал"}
			}
			opts.ProductionBackupMaxAge = args[i+1]
			i++
		case strings.HasPrefix(arg, "--backup-max-age="):
			opts.ProductionBackupMaxAge = strings.TrimPrefix(arg, "--backup-max-age=")
		default:
			return usageError{"неизвестный флаг production gate: " + arg}
		}
	}
	rep, err := scanner.ScanWithOptions(target, opts)
	if err != nil {
		return err
	}
	gateFindings := make([]model.Finding, 0)
	for _, f := range rep.Findings {
		if strings.HasPrefix(f.ID, "craft.gate.") {
			gateFindings = append(gateFindings, f)
		}
	}
	if strings.EqualFold(format, "json") {
		data, err := json.MarshalIndent(struct {
			ToolVersion string                  `json:"tool_version"`
			TargetPath  string                  `json:"target_path"`
			Gate        model.CraftAdvancedInfo `json:"gate"`
			Findings    []model.Finding         `json:"findings"`
		}{ToolVersion: version.Version, TargetPath: rep.TargetPath, Gate: rep.CraftAdvanced, Findings: gateFindings}, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(data))
		return err
	}
	if !strings.EqualFold(format, "text") && format != "" {
		return usageError{"неподдерживаемый формат production gate: " + format}
	}
	_, err = fmt.Fprint(stdout, craftadvanced.FormatGateText(rep.CraftAdvanced))
	return err
}

func runJava(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printJavaHelp(stdout)
		return nil
	}
	if args[0] != "flags" {
		return usageError{"неизвестная подкоманда java: " + args[0]}
	}
	return runJavaFlags(args[1:], stdout)
}

func runJavaFlags(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError{"для java flags нужно указать путь к Minecraft-серверу"}
	}
	target := args[0]
	format := "text"
	opts := scanner.Options{}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--format" || arg == "-f":
			if i+1 >= len(args) {
				return usageError{"после " + arg + " нужно указать формат"}
			}
			format = args[i+1]
			i++
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
		case arg == "--no-ignore":
			opts.NoIgnore = true
		case arg == "--ignore-file":
			if i+1 >= len(args) {
				return usageError{"после --ignore-file нужно указать путь файла"}
			}
			opts.IgnoreFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--ignore-file="):
			opts.IgnoreFile = strings.TrimPrefix(arg, "--ignore-file=")
		case arg == "--rules-file":
			if i+1 >= len(args) {
				return usageError{"после --rules-file нужно указать путь JSON-файла правил"}
			}
			opts.RulesFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--rules-file="):
			opts.RulesFile = strings.TrimPrefix(arg, "--rules-file=")
		default:
			return usageError{"неизвестный флаг java flags: " + arg}
		}
	}
	rep, err := scanner.ScanWithOptions(target, opts)
	if err != nil {
		return err
	}
	javaFindings := make([]model.Finding, 0)
	for _, f := range rep.Findings {
		if strings.HasPrefix(f.ID, "craft.java.") || strings.HasPrefix(f.ID, "performance.jvm.") || strings.HasPrefix(f.ID, "performance.java.") {
			javaFindings = append(javaFindings, f)
		}
	}
	if strings.EqualFold(format, "json") {
		data, err := json.MarshalIndent(struct {
			ToolVersion string                  `json:"tool_version"`
			TargetPath  string                  `json:"target_path"`
			Java        model.JavaInfo          `json:"java"`
			Performance model.PerformanceInfo   `json:"performance"`
			Advanced    model.CraftAdvancedInfo `json:"advanced"`
			Findings    []model.Finding         `json:"findings"`
		}{ToolVersion: version.Version, TargetPath: rep.TargetPath, Java: rep.Java, Performance: rep.Performance, Advanced: rep.CraftAdvanced, Findings: javaFindings}, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(data))
		return err
	}
	if !strings.EqualFold(format, "text") && format != "" {
		return usageError{"неподдерживаемый формат java flags: " + format}
	}
	_, err = fmt.Fprint(stdout, craftadvanced.FormatJavaText(rep.CraftAdvanced))
	return err
}

func runWorld(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printWorldHelp(stdout)
		return nil
	}
	if args[0] != "audit" {
		return usageError{"неизвестная подкоманда world: " + args[0]}
	}
	return runWorldAudit(args[1:], stdout)
}

func runWorldAudit(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError{"для world audit нужно указать путь к Minecraft-серверу"}
	}
	target := args[0]
	format := "text"
	opts := scanner.Options{}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--format" || arg == "-f":
			if i+1 >= len(args) {
				return usageError{"после " + arg + " нужно указать формат"}
			}
			format = args[i+1]
			i++
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
		case arg == "--no-ignore":
			opts.NoIgnore = true
		case arg == "--ignore-file":
			if i+1 >= len(args) {
				return usageError{"после --ignore-file нужно указать путь файла"}
			}
			opts.IgnoreFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--ignore-file="):
			opts.IgnoreFile = strings.TrimPrefix(arg, "--ignore-file=")
		case arg == "--rules-file":
			if i+1 >= len(args) {
				return usageError{"после --rules-file нужно указать путь JSON-файла правил"}
			}
			opts.RulesFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--rules-file="):
			opts.RulesFile = strings.TrimPrefix(arg, "--rules-file=")
		default:
			return usageError{"неизвестный флаг world audit: " + arg}
		}
	}
	rep, err := scanner.ScanWithOptions(target, opts)
	if err != nil {
		return err
	}
	worldFindings := make([]model.Finding, 0)
	for _, f := range rep.Findings {
		if strings.HasPrefix(f.ID, "craft.world.") {
			worldFindings = append(worldFindings, f)
		}
	}
	if strings.EqualFold(format, "json") {
		data, err := json.MarshalIndent(struct {
			ToolVersion string                  `json:"tool_version"`
			TargetPath  string                  `json:"target_path"`
			Worlds      []model.CraftWorldInfo  `json:"worlds"`
			Advanced    model.CraftAdvancedInfo `json:"advanced"`
			Findings    []model.Finding         `json:"findings"`
		}{ToolVersion: version.Version, TargetPath: rep.TargetPath, Worlds: rep.CraftAdvanced.Worlds, Advanced: rep.CraftAdvanced, Findings: worldFindings}, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(data))
		return err
	}
	if !strings.EqualFold(format, "text") && format != "" {
		return usageError{"неподдерживаемый формат world audit: " + format}
	}
	_, err = fmt.Fprint(stdout, craftadvanced.FormatWorldText(rep.CraftAdvanced))
	return err
}

func runProxy(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printProxyHelp(stdout)
		return nil
	}
	switch args[0] {
	case "scan", "audit":
		return runProxyScan(args[1:], stdout)
	default:
		return usageError{"неизвестная подкоманда proxy: " + args[0]}
	}
}

func runProxyScan(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError{"для proxy scan нужно указать путь к Minecraft-серверу или корню сети"}
	}
	target := args[0]
	format := "text"
	rulesFile := ""
	ignoreFile := ""
	noIgnore := false
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--format" || arg == "-f":
			if i+1 >= len(args) {
				return usageError{"после " + arg + " нужно указать формат"}
			}
			format = args[i+1]
			i++
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
		case arg == "--rules-file":
			if i+1 >= len(args) {
				return usageError{"после --rules-file нужно указать путь JSON-файла правил"}
			}
			rulesFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--rules-file="):
			rulesFile = strings.TrimPrefix(arg, "--rules-file=")
		case arg == "--ignore-file":
			if i+1 >= len(args) {
				return usageError{"после --ignore-file нужно указать путь файла"}
			}
			ignoreFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--ignore-file="):
			ignoreFile = strings.TrimPrefix(arg, "--ignore-file=")
		case arg == "--no-ignore":
			noIgnore = true
		default:
			return usageError{"неизвестный флаг proxy scan: " + arg}
		}
	}
	rep, err := scanner.ScanWithOptions(target, scanner.Options{RulesFile: rulesFile, IgnoreFile: ignoreFile, NoIgnore: noIgnore})
	if err != nil {
		return err
	}
	proxyFindings := make([]model.Finding, 0)
	for _, f := range rep.Findings {
		if f.Category == "proxy/network" || strings.HasPrefix(f.ID, "proxy.") {
			proxyFindings = append(proxyFindings, f)
		}
	}
	if strings.EqualFold(format, "json") {
		data, err := proxy.ToJSON(rep.Proxy, proxyFindings, version.Version, rep.TargetPath)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(data))
		return err
	}
	if !strings.EqualFold(format, "text") && format != "" {
		return usageError{"неподдерживаемый формат proxy scan: " + format}
	}
	_, err = fmt.Fprint(stdout, proxy.FormatAnalysisText(rep.Proxy, proxyFindings))
	return err
}

func runRules(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printRulesHelp(stdout)
		return nil
	}
	switch args[0] {
	case "list":
		return runRulesList(args[1:], stdout)
	case "explain":
		return runRulesExplain(args[1:], stdout)
	case "validate":
		return runRulesValidate(args[1:], stdout)
	default:
		return usageError{"неизвестная подкоманда rules: " + args[0]}
	}
}

func runRulesList(args []string, stdout io.Writer) error {
	format := "text"
	category := ""
	severity := ""
	rulesFile := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--format" || arg == "-f":
			if i+1 >= len(args) {
				return usageError{"после " + arg + " нужно указать формат"}
			}
			format = args[i+1]
			i++
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
		case arg == "--category":
			if i+1 >= len(args) {
				return usageError{"после --category нужно указать категорию"}
			}
			category = args[i+1]
			i++
		case strings.HasPrefix(arg, "--category="):
			category = strings.TrimPrefix(arg, "--category=")
		case arg == "--severity":
			if i+1 >= len(args) {
				return usageError{"после --severity нужно указать уровень"}
			}
			severity = args[i+1]
			i++
		case strings.HasPrefix(arg, "--severity="):
			severity = strings.TrimPrefix(arg, "--severity=")
		case arg == "--rules-file":
			if i+1 >= len(args) {
				return usageError{"после --rules-file нужно указать путь JSON-файла правил"}
			}
			rulesFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--rules-file="):
			rulesFile = strings.TrimPrefix(arg, "--rules-file=")
		default:
			return usageError{"неизвестный флаг rules list: " + arg}
		}
	}
	catalog, err := rules.LoadCatalog(rulesFile)
	if err != nil {
		return err
	}
	list := filterRules(catalog.SortedRules(), category, severity)
	if strings.EqualFold(format, "json") {
		data, err := json.MarshalIndent(list, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(data))
		return err
	}
	if !strings.EqualFold(format, "text") && format != "" {
		return usageError{"неподдерживаемый формат rules list: " + format}
	}
	fmt.Fprintf(stdout, "Каталог правил CraftDoctor %s · правил: %d\n\n", catalog.Version, len(list))
	for _, rule := range list {
		fmt.Fprintf(stdout, "[%s] %s · %s · %s\n", rule.Severity, rule.ID, rule.Category, rule.Title)
	}
	return nil
}

func runRulesExplain(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError{"для rules explain нужно указать ID правила"}
	}
	id := args[0]
	format := "text"
	rulesFile := ""
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--format" || arg == "-f":
			if i+1 >= len(args) {
				return usageError{"после " + arg + " нужно указать формат"}
			}
			format = args[i+1]
			i++
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
		case arg == "--rules-file":
			if i+1 >= len(args) {
				return usageError{"после --rules-file нужно указать путь JSON-файла правил"}
			}
			rulesFile = args[i+1]
			i++
		case strings.HasPrefix(arg, "--rules-file="):
			rulesFile = strings.TrimPrefix(arg, "--rules-file=")
		default:
			return usageError{"неизвестный флаг rules explain: " + arg}
		}
	}
	catalog, err := rules.LoadCatalog(rulesFile)
	if err != nil {
		return err
	}
	rule, ok := catalog.Find(id)
	if !ok {
		return fmt.Errorf("правило не найдено: %s", id)
	}
	if strings.EqualFold(format, "json") {
		data, err := json.MarshalIndent(rule, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(data))
		return err
	}
	if !strings.EqualFold(format, "text") && format != "" {
		return usageError{"неподдерживаемый формат rules explain: " + format}
	}
	fmt.Fprintf(stdout, "ID: %s\n", rule.ID)
	fmt.Fprintf(stdout, "Уровень: %s\n", rule.Severity)
	fmt.Fprintf(stdout, "Категория: %s\n", rule.Category)
	fmt.Fprintf(stdout, "Название: %s\n", rule.Title)
	fmt.Fprintf(stdout, "Описание: %s\n", rule.Description)
	if rule.Recommendation != "" {
		fmt.Fprintf(stdout, "Рекомендация: %s\n", rule.Recommendation)
	}
	return nil
}

func runRulesValidate(args []string, stdout io.Writer) error {
	format := "text"
	path := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--format" || arg == "-f":
			if i+1 >= len(args) {
				return usageError{"после " + arg + " нужно указать формат"}
			}
			format = args[i+1]
			i++
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
		case arg == "--path":
			if i+1 >= len(args) {
				return usageError{"после --path нужно указать путь к каталогу или JSON-файлу правил"}
			}
			path = args[i+1]
			i++
		case strings.HasPrefix(arg, "--path="):
			path = strings.TrimPrefix(arg, "--path=")
		default:
			if strings.HasPrefix(arg, "-") {
				return usageError{"неизвестный флаг rules validate: " + arg}
			}
			if path != "" {
				return usageError{"для rules validate можно указать только один путь"}
			}
			path = arg
		}
	}
	report, err := rules.ValidateCatalogPath(path)
	if err != nil {
		return err
	}
	if strings.EqualFold(format, "json") {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(data))
		if err != nil {
			return err
		}
	} else {
		if !strings.EqualFold(format, "text") && format != "" {
			return usageError{"неподдерживаемый формат rules validate: " + format}
		}
		target := "встроенный каталог"
		if strings.TrimSpace(path) != "" {
			target = path
		}
		fmt.Fprintf(stdout, "Проверка каталога правил CraftDoctor: %s\n", target)
		fmt.Fprintf(stdout, "Версия: %s\n", report.Version)
		fmt.Fprintf(stdout, "Правил: %d\n", report.Rules)
		if len(report.Modules) > 0 {
			fmt.Fprintln(stdout, "Модули:")
			keys := make([]string, 0, len(report.Modules))
			for key := range report.Modules {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				fmt.Fprintf(stdout, "- %s: %d\n", key, report.Modules[key])
			}
		}
		if len(report.Issues) == 0 {
			fmt.Fprintln(stdout, "Статус: OK")
		} else {
			fmt.Fprintln(stdout, "Проблемы:")
			for _, issue := range report.Issues {
				if issue.RuleID != "" {
					fmt.Fprintf(stdout, "- [%s] %s: %s\n", issue.Severity, issue.RuleID, issue.Message)
				} else {
					fmt.Fprintf(stdout, "- [%s] %s\n", issue.Severity, issue.Message)
				}
			}
		}
	}
	if !report.Valid {
		return qualityGateError{"каталог правил не прошёл проверку"}
	}
	return nil
}

func filterRules(list []rules.Rule, category, severity string) []rules.Rule {
	out := make([]rules.Rule, 0, len(list))
	for _, rule := range list {
		if category != "" && !strings.EqualFold(rule.Category, category) {
			continue
		}
		if severity != "" && !strings.EqualFold(string(rule.Severity), severity) {
			continue
		}
		out = append(out, rule)
	}
	return out
}

func normalizeOutput(path string) string {
	if filepath.Dir(path) == "." {
		return "."
	}
	return path
}

func printHelp(w io.Writer) {
	fmt.Fprintf(w, `%s %s

Назначение:
  CraftDoctor — read-only CLI-инструмент для диагностики Minecraft-серверных проектов.

Использование:
  craftdoctor scan <путь> [--format html|markdown|json] [--output файл]
  craftdoctor ci <путь> [--output report.json] [--max-critical 0]
  craftdoctor schema report [--output report.schema.json]
  craftdoctor scan <путь> --stdout --format markdown
  craftdoctor rules list [--format text|json]
  craftdoctor rules explain <id> [--format text|json]
  craftdoctor rules validate [путь] [--format text|json]
  craftdoctor config audit <путь> [--format text|json]
  craftdoctor plugins audit <путь> [--format text|json]
  craftdoctor plugins graph <путь> [--format text|json|mermaid|dot]
  craftdoctor plugins explain <путь> <плагин> [--format text|json]
  craftdoctor logs analyze <путь> [--format text|json] [--last-run] [--since 24h]
  craftdoctor performance scan <путь> [--format text|json] [--players N] [--target тип]
  craftdoctor java flags <путь> [--format text|json]
  craftdoctor world audit <путь> [--format text|json]
  craftdoctor proxy scan <путь> [--format text|json]
  craftdoctor proxy audit <путь> [--format text|json]
  craftdoctor security audit <путь> [--format text|json]
  craftdoctor production check <путь> [--format text|json] [--systemd-dir путь] [--logrotate-dir путь] [--backup-max-age 24h]
  craftdoctor production gate <путь> [--format text|json]
  craftdoctor version [--json]
  craftdoctor help

Примеры:
  craftdoctor scan /srv/minecraft
  craftdoctor scan /srv/minecraft --format markdown --output report.md
  craftdoctor scan /srv/minecraft --format json --stdout
  craftdoctor ci /srv/minecraft --output report.json
  craftdoctor ci /srv/minecraft --max-critical 0 --max-danger 0
  craftdoctor schema report --output schemas/report.schema.json
  craftdoctor version --json
  craftdoctor scan /srv/minecraft --ignore-file .craftdoctorignore
  craftdoctor rules list --severity CRITICAL
  craftdoctor rules explain security.online_mode.false_without_proxy
  craftdoctor rules validate
  craftdoctor config audit /srv/minecraft
  craftdoctor plugins audit /srv/minecraft
  craftdoctor plugins graph /srv/minecraft --format mermaid
  craftdoctor plugins explain /srv/minecraft LuckPerms
  craftdoctor logs analyze /srv/minecraft
  craftdoctor logs analyze /srv/minecraft --last-run
  craftdoctor logs analyze /srv/minecraft --since 24h
  craftdoctor logs analyze /srv/minecraft/logs/latest.log --format json
  craftdoctor performance scan /srv/minecraft
  craftdoctor performance scan /srv/minecraft --players 80 --target rpg
  craftdoctor java flags /srv/minecraft
  craftdoctor world audit /srv/minecraft
  craftdoctor proxy scan /srv/minecraft
  craftdoctor security audit /srv/minecraft
  craftdoctor production check /srv/minecraft
  craftdoctor production gate /srv/minecraft
  craftdoctor production check /srv/minecraft --systemd-dir /etc/systemd/system --logrotate-dir /etc/logrotate.d --backup-max-age 24h

Флаги scan:
  -f, --format             Формат отчёта: html, markdown, json. По умолчанию: html.
  -o, --output             Путь для сохранения отчёта. По умолчанию: craftdoctor-report.<format>.
      --stdout             Вывести отчёт в stdout вместо записи в файл.
      --fail-on-critical   Завершить команду с ошибкой, если найдены CRITICAL-проблемы.
      --ignore-file        Путь к файлу игнорирования. По умолчанию: <сервер>/.craftdoctorignore.
      --no-ignore          Отключить чтение .craftdoctorignore.
      --rules-file         Подключить дополнительный JSON-файл правил.
      --players            Целевой онлайн для Performance Doctor 2.0.
      --target             Тип сервера: online, survival, rpg, minigames, custom.
      --systemd-dir        Дополнительная директория systemd service-файлов для Production Doctor.
      --logrotate-dir      Дополнительная директория logrotate-конфигов для Production Doctor.
      --backup-max-age     Максимальный возраст свежего бэкапа: 24h, 7d, 168h.

CI exit codes:
  0   Проверка пройдена.
  1   Ошибка выполнения.
  2   Quality gate не пройден.
  64  Ошибка использования CLI.

`, version.Name, version.Version)
}

func printSchemaHelp(w io.Writer) {
	fmt.Fprintf(w, `Команды JSON Schema

Использование:
  craftdoctor schema report [--output файл]

Примеры:
  craftdoctor schema report
  craftdoctor schema report --output schemas/report.schema.json

Назначение:
  Команда выводит стабильную JSON-схему отчёта CraftDoctor 2.x для CI/CD, валидаторов и внешней автоматизации.

`)
}

func printConfigHelp(w io.Writer) {
	fmt.Fprintf(w, `Команды Config Doctor

Использование:
  craftdoctor config audit <путь-к-серверу> [--format text|json]

Примеры:
  craftdoctor config audit /srv/minecraft
  craftdoctor config audit /srv/minecraft --format json
  craftdoctor config audit /srv/minecraft --no-ignore

Флаги:
  -f, --format       Формат вывода: text или json.
      --ignore-file  Путь к файлу игнорирования. По умолчанию: <сервер>/.craftdoctorignore.
      --no-ignore    Отключить чтение .craftdoctorignore.
      --rules-file   Подключить дополнительный JSON-файл правил.

`)
}

func printPluginsHelp(w io.Writer) {
	fmt.Fprintf(w, `Команды Plugin Doctor

Использование:
  craftdoctor plugins audit <путь> [--format text|json]
  craftdoctor plugins graph <путь> [--format text|json|mermaid|dot]
  craftdoctor plugins explain <путь> <плагин> [--format text|json]

Примеры:
  craftdoctor plugins audit /srv/minecraft
  craftdoctor plugins audit /srv/minecraft --format json
  craftdoctor plugins graph /srv/minecraft --format mermaid
  craftdoctor plugins graph /srv/minecraft --format dot
  craftdoctor plugins explain /srv/minecraft LuckPerms
  craftdoctor plugins audit /srv/minecraft --no-ignore

Флаги:
  -f, --format       Формат вывода: text/json, для graph также mermaid/dot.
      --ignore-file  Путь к файлу игнорирования. По умолчанию: <сервер>/.craftdoctorignore.
      --no-ignore    Отключить чтение .craftdoctorignore.
      --rules-file   Подключить дополнительный JSON-файл правил.

`)
}

func printLogsHelp(w io.Writer) {
	fmt.Fprintf(w, `Команды Log Doctor 2.0

Использование:
  craftdoctor logs analyze <путь-к-серверу-или-log-файлу> [--format text|json] [--last-run] [--since 24h]

Примеры:
  craftdoctor logs analyze /srv/minecraft
  craftdoctor logs analyze /srv/minecraft/logs/latest.log
  craftdoctor logs analyze /srv/minecraft --format json
  craftdoctor logs analyze /srv/minecraft --last-run
  craftdoctor logs analyze /srv/minecraft --since 24h

Флаги:
  -f, --format  Формат вывода: text или json.
      --last-run Анализировать только последнюю обнаруженную сессию запуска сервера.
      --since    Анализировать строки не старше интервала, если в логе есть абсолютные даты. Примеры: 30m, 24h, 7d.

`)
}

func printPerformanceHelp(w io.Writer) {
	fmt.Fprintf(w, `Команды Performance Doctor 2.0

Использование:
  craftdoctor performance scan <путь-к-серверу> [--format text|json] [--players N] [--target тип]

Примеры:
  craftdoctor performance scan /srv/minecraft
  craftdoctor performance scan /srv/minecraft --players 80 --target rpg
  craftdoctor performance scan /srv/minecraft --format json
  craftdoctor performance scan /srv/minecraft --no-ignore

Флаги:
  -f, --format       Формат вывода: text или json.
      --players      Целевой онлайн для capacity-ориентиров.
      --target       Тип сервера: online, survival, rpg, minigames, custom.
      --ignore-file  Путь к файлу игнорирования. По умолчанию: <сервер>/.craftdoctorignore.
      --no-ignore    Отключить чтение .craftdoctorignore.
      --rules-file   Подключить дополнительный JSON-файл правил.

`)
}

func printJavaHelp(w io.Writer) {
	fmt.Fprintf(w, `Команды Java Doctor для CraftDoctor

Использование:
  craftdoctor java flags <путь-к-серверу> [--format text|json]

Примеры:
  craftdoctor java flags /srv/minecraft
  craftdoctor java flags /srv/minecraft --format json

Назначение:
  Команда ревизует JVM-флаги запуска Minecraft-сервера: -Xmx/-Xms, GC-профиль,
  устаревшие параметры и полноту типового G1GC-профиля.

`)
}

func printWorldHelp(w io.Writer) {
	fmt.Fprintf(w, `Команды World Doctor для CraftDoctor

Использование:
  craftdoctor world audit <путь-к-серверу> [--format text|json]

Примеры:
  craftdoctor world audit /srv/minecraft
  craftdoctor world audit /srv/minecraft --format json

Назначение:
  Команда проверяет директории миров: level.dat, session.lock, uid.dat,
  region/entities/playerdata/datapacks и признаки крупных миров для backup/restore.

`)
}

func printProxyHelp(w io.Writer) {
	fmt.Fprintf(w, `Команды Proxy / Network Doctor

Использование:
  craftdoctor proxy scan <путь-к-серверу-или-корню-сети> [--format text|json]
  craftdoctor proxy audit <путь-к-серверу-или-корню-сети> [--format text|json]

Примеры:
  craftdoctor proxy scan /srv/minecraft
  craftdoctor proxy scan /srv/minecraft-network --format json
  craftdoctor proxy scan /srv/minecraft --no-ignore

Флаги:
  -f, --format       Формат вывода: text или json.
      --ignore-file  Путь к файлу игнорирования. По умолчанию: <сервер>/.craftdoctorignore.
      --no-ignore    Отключить чтение .craftdoctorignore.
      --rules-file   Подключить дополнительный JSON-файл правил.

`)
}

func printSecurityHelp(w io.Writer) {
	fmt.Fprintf(w, `Команды Security Doctor

Использование:
  craftdoctor security audit <путь-к-серверу> [--format text|json]

Примеры:
  craftdoctor security audit /srv/minecraft
  craftdoctor production check /srv/minecraft
  craftdoctor security audit /srv/minecraft --format json
  craftdoctor security audit /srv/minecraft --no-ignore

Флаги:
  -f, --format       Формат вывода: text или json.
      --ignore-file  Путь к файлу игнорирования. По умолчанию: <сервер>/.craftdoctorignore.
      --no-ignore    Отключить чтение .craftdoctorignore.
      --rules-file   Подключить дополнительный JSON-файл правил.

`)
}

func printProductionHelp(w io.Writer) {
	fmt.Fprintf(w, `Команды Production Doctor 2.0

Использование:
  craftdoctor production check <путь-к-серверу> [--format text|json] [--systemd-dir путь] [--logrotate-dir путь] [--backup-max-age 24h]
  craftdoctor production gate <путь-к-серверу> [--format text|json]

Примеры:
  craftdoctor production check /srv/minecraft
  craftdoctor production check /srv/minecraft --format json
  craftdoctor production check /srv/minecraft --systemd-dir /etc/systemd/system --logrotate-dir /etc/logrotate.d
  craftdoctor production check /srv/minecraft --backup-max-age 24h
  craftdoctor production gate /srv/minecraft --format json
  craftdoctor production check /srv/minecraft --no-ignore

Флаги:
  -f, --format        Формат вывода: text или json.
      --ignore-file   Путь к файлу игнорирования. По умолчанию: <сервер>/.craftdoctorignore.
      --no-ignore     Отключить чтение .craftdoctorignore.
      --rules-file    Подключить дополнительный JSON-файл правил.
      --systemd-dir   Дополнительная директория systemd service-файлов.
      --logrotate-dir Дополнительная директория logrotate-конфигов.
      --backup-max-age Максимальный возраст свежего бэкапа: 24h, 7d, 168h.

`)
}

func printRulesHelp(w io.Writer) {
	fmt.Fprintf(w, `Команды правил CraftDoctor

Использование:
  craftdoctor rules list [--format text|json] [--category категория] [--severity уровень]
  craftdoctor rules explain <id> [--format text|json]
  craftdoctor rules validate [путь] [--format text|json]

Примеры:
  craftdoctor rules list
  craftdoctor rules list --severity CRITICAL
  craftdoctor rules list --category безопасность
  craftdoctor rules explain plugins.dependency.missing.*
  craftdoctor rules explain security.rcon.enabled --format json
  craftdoctor rules validate
  craftdoctor rules validate --format json

Флаги:
  -f, --format       Формат вывода: text или json.
      --category     Фильтр по категории.
      --severity     Фильтр по уровню: INFO, WARN, DANGER, CRITICAL.
      --rules-file   Подключить дополнительный JSON-файл правил.

`)
}
