package logs

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
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

	"gitflic.ru/skif4er/doctortools/internal/model"
)

const maxLogBytes int64 = 10 * 1024 * 1024

var (
	couldNotLoadPluginRe = regexp.MustCompile(`(?i)could not load ['\"]?plugins[\\/]([^'\"\\/]+\.jar)['\"]?`)
	enablePluginRe       = regexp.MustCompile(`(?i)(?:error occurred while enabling|error enabling|failed to enable)\s+([A-Za-z0-9_.-]+)`)
	disablePluginRe      = regexp.MustCompile(`(?i)(?:error occurred while disabling|error disabling|failed to disable)\s+([A-Za-z0-9_.-]+)`)
	pluginBracketRe      = regexp.MustCompile(`\[([A-Za-z0-9_.-]+)]`)
	missingPluginRe      = regexp.MustCompile(`(?i)(?:unknown dependency|missing dependency|depend(?:ency)? .* not found|could not find dependency)[: ]+([A-Za-z0-9_.-]+)?`)
	logTimestampRe       = regexp.MustCompile(`^\[(\d{2}:\d{2}:\d{2})(?:\s+[^\]]*)?]`)
	isoTimestampRe       = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})[ T](\d{2}:\d{2}:\d{2})`)
	atLineRe             = regexp.MustCompile(`\bat\s+[\w.$]+\([^)]*\)`)
	causedByRe           = regexp.MustCompile(`(?i)^\s*Caused by:\s*(.+)$`)
	exceptionNameRe      = regexp.MustCompile(`(?i)([A-Za-z0-9_.$]+(?:Exception|Error))(?::\s*(.*))?`)
)

// Options задаёт дополнительные режимы Log Doctor 2.0.
type Options struct {
	LastRun bool
	Since   time.Duration
}

// Analyze выполняет анализ logs/latest.log внутри директории Minecraft-сервера.
func Analyze(root string) (model.LogInfo, []model.Finding) {
	latest := filepath.Join(root, "logs", "latest.log")
	return AnalyzeFile(latest)
}

// AnalyzeTarget принимает либо директорию сервера, либо прямой путь к .log-файлу.
func AnalyzeTarget(target string) (model.LogInfo, []model.Finding, error) {
	return AnalyzeTargetWithOptions(target, Options{})
}

// AnalyzeTargetWithOptions принимает либо директорию сервера, либо прямой путь к .log-файлу с режимами --last-run/--since.
func AnalyzeTargetWithOptions(target string, opts Options) (model.LogInfo, []model.Finding, error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return model.LogInfo{}, nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return model.LogInfo{}, nil, fmt.Errorf("не удалось открыть путь %s: %w", abs, err)
	}
	if st.IsDir() {
		info, findings := AnalyzeFileWithOptions(filepath.Join(abs, "logs", "latest.log"), opts)
		return info, findings, nil
	}
	info, findings := AnalyzeFileWithOptions(abs, opts)
	return info, findings, nil
}

// AnalyzeFile анализирует конкретный файл лога.
func AnalyzeFile(path string) (model.LogInfo, []model.Finding) {
	return AnalyzeFileWithOptions(path, Options{})
}

// AnalyzeFileWithOptions анализирует конкретный файл лога с поддержкой Log Doctor 2.0.
func AnalyzeFileWithOptions(path string, opts Options) (model.LogInfo, []model.Finding) {
	info := model.LogInfo{AnalyzedFile: path, Options: model.LogAnalysisOptions{LastRun: opts.LastRun}}
	if opts.Since > 0 {
		info.Options.Since = opts.Since.String()
	}
	data, err := readTail(path, maxLogBytes)
	if err != nil {
		findings := []model.Finding{{
			ID:             "logs.latest.not_found",
			Severity:       model.SeverityWarn,
			Category:       "логи",
			Title:          "Лог-файл не найден",
			Message:        "CraftDoctor не смог прочитать файл лога: " + path + ".",
			Recommendation: "Если сервер уже запускался, проверить наличие директории logs и права доступа.",
			File:           path,
		}}
		return info, findings
	}
	info.LatestLogFound = true

	lines := splitLines(data)
	allLines := make([]logLine, 0, len(lines))
	for i, line := range lines {
		allLines = append(allLines, logLine{No: i + 1, Text: line, Time: parseLineTime(line)})
	}
	info.Sessions = detectSessions(allLines)
	selected := selectLines(allLines, info.Sessions, opts)
	if opts.LastRun && len(info.Sessions) > 0 {
		s := info.Sessions[len(info.Sessions)-1]
		info.SelectedSession = &s
	}

	state := newLogState()
	for _, item := range selected {
		analyzeLine(item.Text, item.No, &info, state)
	}
	info.LinesAnalyzed = len(selected)
	info.Issues = state.issues()
	info.Components = state.componentSummaries()
	info.RootCauses = state.rootCauses()
	info.IssueTypes = state.issueTypes()
	info.StackTraces = extractStackTraces(selected)
	if len(info.RootCauses) > 0 {
		main := info.RootCauses[0]
		info.MainRootCause = &main
	}

	findings := buildFindings(path, info)
	return info, findings
}

type logLine struct {
	No   int
	Text string
	Time string
}

type logState struct {
	issuesByKey     map[string]*model.LogIssue
	components      map[string]*model.LogComponentSummary
	rootByKey       map[string]*model.LogRootCause
	issueTypesByKey map[string]*model.LogIssueTypeSummary
}

func newLogState() *logState {
	return &logState{
		issuesByKey:     map[string]*model.LogIssue{},
		components:      map[string]*model.LogComponentSummary{},
		rootByKey:       map[string]*model.LogRootCause{},
		issueTypesByKey: map[string]*model.LogIssueTypeSummary{},
	}
}

func analyzeLine(line string, lineNo int, info *model.LogInfo, state *logState) {
	lower := strings.ToLower(line)
	component := detectComponent(line)
	severity := model.SeverityInfo

	if containsAny(line, lower, []string{" WARN]", "[WARN]", " warn "}) {
		info.WarnCount++
		severity = model.SeverityWarn
	}
	if containsAny(line, lower, []string{" ERROR]", "[ERROR]", " error "}) {
		info.ErrorCount++
		severity = model.SeverityDanger
		addSample(info, line)
	}
	if strings.Contains(line, "Exception") || strings.Contains(line, "Caused by:") || strings.Contains(line, "Error:") || strings.Contains(lower, "stacktrace") {
		info.ExceptionCount++
		if severity.Rank() < model.SeverityDanger.Rank() {
			severity = model.SeverityDanger
		}
		addSample(info, line)
	}
	if strings.Contains(lower, "can't keep up") || strings.Contains(lower, "server overloaded") || strings.Contains(lower, "running behind") {
		info.LongTickCount++
		addSample(info, line)
		state.addIssue("long_tick", model.SeverityWarn, component, lineNo, line, "Сервер не успевает обрабатывать tick loop", "Проверить view-distance, simulation-distance, сущности, чанки и тяжёлые плагины через spark.")
		state.addRoot("performance.long_tick", model.SeverityWarn, component, line, "Признаки перегрузки tick loop", "Нужен profiling через spark/timings и проверка настроек дистанций, сущностей и плагинов.")
	}

	if isMissingDependencyLine(lower) {
		dep := captureMissingDependency(line)
		message := "Плагин или компонент не смог найти обязательную зависимость"
		if dep != "" {
			message += ": " + dep
		}
		state.addIssue("missing_dependency", model.SeverityCritical, component, lineNo, line, message, "Установить отсутствующую зависимость или отключить зависящий от неё плагин.")
		state.addRoot("plugins.missing_dependency", model.SeverityCritical, component, line, "Отсутствующая зависимость плагина", "Проверить depend/softdepend в plugin.yml и список установленных плагинов.")
		addSample(info, line)
	}
	if strings.Contains(lower, "could not load") || strings.Contains(lower, "failed to load") || strings.Contains(lower, "invalid plugin.yml") || strings.Contains(lower, "invalidpluginexception") {
		state.addIssue("plugin_load_failed", model.SeverityCritical, component, lineNo, line, "Плагин не загрузился", "Проверить jar-файл, plugin.yml/paper-plugin.yml, зависимости и совместимость с версией ядра.")
		state.addRoot("plugins.load_failed", model.SeverityCritical, component, line, "Ошибка загрузки плагина", "Проверить конкретный плагин, jar-файл, зависимости и stack trace ниже по логу.")
		addSample(info, line)
	}
	if strings.Contains(lower, "error occurred while enabling") || strings.Contains(lower, "failed to enable") || strings.Contains(lower, "error enabling") {
		state.addIssue("plugin_enable_failed", model.SeverityDanger, component, lineNo, line, "Плагин не включился после загрузки", "Проверить stack trace, конфигурацию плагина, зависимости и версию API.")
		state.addRoot("plugins.enable_failed", model.SeverityDanger, component, line, "Ошибка включения плагина", "Проверить блок stack trace для этого плагина и его конфигурацию.")
		addSample(info, line)
	}
	if strings.Contains(lower, "sqlexception") || strings.Contains(lower, "communications link failure") || strings.Contains(lower, "hikaripool") || strings.Contains(lower, "access denied for user") || strings.Contains(lower, "database") && strings.Contains(lower, "connection") {
		state.addIssue("database_error", model.SeverityDanger, component, lineNo, line, "В логе найдена ошибка базы данных", "Проверить доступность БД, логин/пароль, host/port, пул соединений и миграции схемы.")
		state.addRoot("database.connection", model.SeverityDanger, component, line, "Проблема подключения или работы с базой данных", "Проверить конфиги плагинов, права пользователя БД, сеть и состояние сервера БД.")
		addSample(info, line)
	}
	if strings.Contains(lower, "accessdeniedexception") || strings.Contains(lower, "permission denied") || strings.Contains(lower, "read-only file system") {
		state.addIssue("filesystem_permission", model.SeverityDanger, component, lineNo, line, "В логе найдена ошибка прав файловой системы", "Проверить владельца файлов, chmod/chown, пользователя systemd-сервиса и права на директории сервера.")
		state.addRoot("filesystem.permissions", model.SeverityDanger, component, line, "Недостаточно прав на файлы или директории", "Проверить права на plugins, worlds, logs, configs и пользователя запуска сервера.")
		addSample(info, line)
	}
	if strings.Contains(lower, "address already in use") || strings.Contains(lower, "bindexception") || strings.Contains(lower, "failed to bind") {
		state.addIssue("network_bind", model.SeverityCritical, component, lineNo, line, "Порт уже занят или сервер не смог привязаться к адресу", "Проверить server-port, bind-address, занятые процессы и systemd unit.")
		state.addRoot("network.bind", model.SeverityCritical, component, line, "Ошибка привязки к порту", "Освободить порт или изменить server-port/bind-address.")
		addSample(info, line)
	}
	if strings.Contains(lower, "classnotfoundexception") || strings.Contains(lower, "noclassdeffounderror") || strings.Contains(lower, "nosuchmethoderror") || strings.Contains(lower, "unsupportedclassversionerror") {
		state.addIssue("java_class_api", model.SeverityDanger, component, lineNo, line, "Java/API несовместимость или отсутствующий класс", "Проверить версию Java, версию ядра, зависимости и совместимость плагина.")
		state.addRoot("java.compatibility", model.SeverityDanger, component, line, "Несовместимость Java/API или отсутствующий класс", "Проверить Java major, api-version плагина и зависимости.")
		addSample(info, line)
	}
	if (strings.Contains(lower, "chunk") || strings.Contains(lower, "region")) && (strings.Contains(lower, "error") || strings.Contains(lower, "corrupt") || strings.Contains(lower, "failed")) {
		state.addIssue("world_chunk", model.SeverityDanger, component, lineNo, line, "Возможная проблема мира, региона или чанка", "Сделать бэкап, проверить crash reports, region-файлы и последние изменения мира.")
		state.addRoot("world.chunk_region", model.SeverityDanger, component, line, "Проблема мира/чанка/region-файла", "Перед любыми действиями сделать бэкап мира и проверить повреждённые region-файлы.")
		addSample(info, line)
	}

	if severity.Rank() >= model.SeverityWarn.Rank() && component != "" {
		state.addComponent(component, severity)
	}
}

func buildFindings(path string, info model.LogInfo) []model.Finding {
	var findings []model.Finding
	if !info.LatestLogFound {
		findings = append(findings, model.Finding{
			ID:             "logs.latest.not_found",
			Severity:       model.SeverityWarn,
			Category:       "логи",
			Title:          "Лог-файл не найден",
			Message:        "CraftDoctor не смог прочитать файл лога: " + path + ".",
			Recommendation: "Если сервер уже запускался, проверить наличие директории logs и права доступа.",
			File:           path,
		})
		return findings
	}
	if info.ErrorCount > 0 {
		findings = append(findings, model.Finding{
			ID:             "logs.errors.detected",
			Severity:       model.SeverityDanger,
			Category:       "логи",
			Title:          "В latest.log найдены ERROR-записи",
			Message:        "Количество ERROR-записей: " + itoa(info.ErrorCount) + ".",
			Recommendation: "Открыть Log Doctor, проверить root cause, stack trace fingerprint и компоненты с наибольшим числом ошибок.",
			File:           path,
		})
	}
	if info.ExceptionCount > 0 {
		findings = append(findings, model.Finding{
			ID:             "logs.exceptions.detected",
			Severity:       model.SeverityDanger,
			Category:       "логи",
			Title:          "В latest.log найдены исключения Java",
			Message:        "Количество строк Exception/Caused by/Error: " + itoa(info.ExceptionCount) + ".",
			Recommendation: "Сгруппировать stack traces по fingerprint и устранить наиболее частую первопричину.",
			File:           path,
		})
	}
	if info.LongTickCount > 0 {
		findings = append(findings, model.Finding{
			ID:             "logs.long_tick.detected",
			Severity:       model.SeverityWarn,
			Category:       "логи",
			Title:          "В latest.log есть признаки lag spike / long tick",
			Message:        "Количество long tick предупреждений: " + itoa(info.LongTickCount) + ".",
			Recommendation: "Запустить Performance Doctor и profiling через spark/timings.",
			File:           path,
		})
	}
	if len(info.StackTraces) > 0 {
		findings = append(findings, model.Finding{
			ID:             "logs.stacktrace.fingerprints.detected",
			Severity:       model.SeverityDanger,
			Category:       "логи",
			Title:          "Stack trace сгруппированы по fingerprint",
			Message:        "Найдено уникальных групп stack trace: " + itoa(len(info.StackTraces)) + ".",
			Recommendation: "Начать с группы с наибольшим числом повторов и проверить её component/cause.",
			File:           path,
		})
	}
	if info.MainRootCause != nil {
		findings = append(findings, model.Finding{
			ID:             "logs.root_cause.primary",
			Severity:       info.MainRootCause.Severity,
			Category:       "логи",
			Title:          "Определена главная причина проблем в логе",
			Message:        info.MainRootCause.Title + ". Компонент: " + info.MainRootCause.Component + ". Повторов: " + itoa(info.MainRootCause.Count) + ".",
			Recommendation: info.MainRootCause.Recommendation,
			File:           path,
		})
	}
	return findings
}

func (s *logState) addIssue(kind string, severity model.Severity, component string, lineNo int, line, message, recommendation string) {
	if component == "" {
		component = "не определён"
	}
	key := kind + "|" + component + "|" + message
	if issue, ok := s.issuesByKey[key]; ok {
		issue.Count++
		s.addIssueType(kind, severity)
		return
	}
	issue := &model.LogIssue{Type: kind, Severity: severity, Component: component, Line: lineNo, Count: 1, Message: message, Evidence: trimLine(line), Recommendation: recommendation}
	s.issuesByKey[key] = issue
	s.addIssueType(kind, severity)
}

func (s *logState) addIssueType(kind string, severity model.Severity) {
	key := kind
	item, ok := s.issueTypesByKey[key]
	if !ok {
		item = &model.LogIssueTypeSummary{Type: kind, Severity: severity}
		s.issueTypesByKey[key] = item
	}
	item.Count++
	if severity.Rank() > item.Severity.Rank() {
		item.Severity = severity
	}
}

func (s *logState) addComponent(component string, severity model.Severity) {
	if component == "" {
		return
	}
	item, ok := s.components[component]
	if !ok {
		item = &model.LogComponentSummary{Component: component}
		s.components[component] = item
	}
	switch severity {
	case model.SeverityCritical:
		item.Critical++
	case model.SeverityDanger:
		item.Errors++
	case model.SeverityWarn:
		item.Warnings++
	}
}

func (s *logState) addRoot(id string, severity model.Severity, component, line, title, recommendation string) {
	if component == "" {
		component = "не определён"
	}
	key := id + "|" + component
	if root, ok := s.rootByKey[key]; ok {
		root.Count++
		if line != "" && root.Message == "" {
			root.Message = trimLine(line)
		}
		return
	}
	s.rootByKey[key] = &model.LogRootCause{ID: id, Severity: severity, Component: component, Title: title, Message: trimLine(line), Recommendation: recommendation, Count: 1}
}

func (s *logState) componentSummaries() []model.LogComponentSummary {
	out := make([]model.LogComponentSummary, 0, len(s.components))
	for _, item := range s.components {
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		ai := out[i].Critical*1000 + out[i].Errors*100 + out[i].Warnings
		aj := out[j].Critical*1000 + out[j].Errors*100 + out[j].Warnings
		if ai == aj {
			return out[i].Component < out[j].Component
		}
		return ai > aj
	})
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}

func (s *logState) rootCauses() []model.LogRootCause {
	out := make([]model.LogRootCause, 0, len(s.rootByKey))
	for _, item := range s.rootByKey {
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Severity.Rank() == out[j].Severity.Rank() {
			if out[i].Count == out[j].Count {
				return out[i].ID < out[j].ID
			}
			return out[i].Count > out[j].Count
		}
		return out[i].Severity.Rank() > out[j].Severity.Rank()
	})
	if len(out) > 12 {
		out = out[:12]
	}
	return out
}

func (s *logState) issues() []model.LogIssue {
	out := make([]model.LogIssue, 0, len(s.issuesByKey))
	for _, item := range s.issuesByKey {
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Severity.Rank() == out[j].Severity.Rank() {
			if out[i].Count == out[j].Count {
				return out[i].Type < out[j].Type
			}
			return out[i].Count > out[j].Count
		}
		return out[i].Severity.Rank() > out[j].Severity.Rank()
	})
	if len(out) > 30 {
		out = out[:30]
	}
	return out
}

func (s *logState) issueTypes() []model.LogIssueTypeSummary {
	out := make([]model.LogIssueTypeSummary, 0, len(s.issueTypesByKey))
	for _, item := range s.issueTypesByKey {
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Severity.Rank() == out[j].Severity.Rank() {
			if out[i].Count == out[j].Count {
				return out[i].Type < out[j].Type
			}
			return out[i].Count > out[j].Count
		}
		return out[i].Severity.Rank() > out[j].Severity.Rank()
	})
	return out
}

func splitLines(data []byte) []string {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 1024*1024)
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func detectSessions(lines []logLine) []model.LogSession {
	if len(lines) == 0 {
		return nil
	}
	starts := []int{0}
	for i, line := range lines {
		lower := strings.ToLower(line.Text)
		if i == 0 {
			continue
		}
		if strings.Contains(lower, "starting minecraft server") || strings.Contains(lower, "loading properties") || strings.Contains(lower, "this server is running") || strings.Contains(lower, "server permissions file permissions.yml is empty") {
			if i-starts[len(starts)-1] > 2 {
				starts = append(starts, i)
			}
		}
	}
	sessions := make([]model.LogSession, 0, len(starts))
	for idx, start := range starts {
		end := len(lines) - 1
		if idx+1 < len(starts) {
			end = starts[idx+1] - 1
		}
		sessions = append(sessions, model.LogSession{Index: idx + 1, StartLine: lines[start].No, EndLine: lines[end].No, StartTime: lines[start].Time, EndTime: lines[end].Time, Reason: sessionReason(lines[start].Text)})
	}
	return sessions
}

func sessionReason(line string) string {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "starting minecraft server"):
		return "starting minecraft server"
	case strings.Contains(lower, "loading properties"):
		return "loading properties"
	case strings.Contains(lower, "this server is running"):
		return "server banner"
	default:
		return "начало файла"
	}
}

func selectLines(lines []logLine, sessions []model.LogSession, opts Options) []logLine {
	selected := lines
	if opts.LastRun && len(sessions) > 0 {
		last := sessions[len(sessions)-1]
		selected = filterByLineRange(selected, last.StartLine, last.EndLine)
	}
	if opts.Since > 0 {
		selected = filterSince(selected, opts.Since)
	}
	return selected
}

func filterByLineRange(lines []logLine, start, end int) []logLine {
	out := []logLine{}
	for _, line := range lines {
		if line.No >= start && line.No <= end {
			out = append(out, line)
		}
	}
	return out
}

func filterSince(lines []logLine, d time.Duration) []logLine {
	if d <= 0 {
		return lines
	}
	// Для latest.log без даты CraftDoctor использует относительный режим консервативно: если нельзя сопоставить время, строка остаётся в анализе.
	if len(lines) == 0 {
		return lines
	}
	cutoff := time.Now().Add(-d)
	out := []logLine{}
	for _, line := range lines {
		if line.Time == "" {
			out = append(out, line)
			continue
		}
		if t, ok := parseAbsoluteTime(line.Time); ok {
			if !t.Before(cutoff) {
				out = append(out, line)
			}
		} else {
			out = append(out, line)
		}
	}
	return out
}

func parseLineTime(line string) string {
	if m := isoTimestampRe.FindStringSubmatch(line); len(m) == 3 {
		return m[1] + " " + m[2]
	}
	if m := logTimestampRe.FindStringSubmatch(line); len(m) == 2 {
		return m[1]
	}
	return ""
}

func parseAbsoluteTime(s string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func extractStackTraces(lines []logLine) []model.LogStackTraceGroup {
	groups := map[string]*model.LogStackTraceGroup{}
	for i := 0; i < len(lines); i++ {
		line := lines[i].Text
		if !looksLikeExceptionStart(line) {
			continue
		}
		block := []logLine{lines[i]}
		for j := i + 1; j < len(lines) && len(block) < 40; j++ {
			text := lines[j].Text
			trimmed := strings.TrimSpace(text)
			if atLineRe.MatchString(trimmed) || strings.HasPrefix(trimmed, "Caused by:") || strings.HasPrefix(trimmed, "Suppressed:") || strings.HasPrefix(trimmed, "...") {
				block = append(block, lines[j])
				i = j
				continue
			}
			if looksLikeExceptionStart(text) {
				block = append(block, lines[j])
				i = j
				continue
			}
			break
		}
		fingerprint, exception, cause, message := stackFingerprint(block)
		if fingerprint == "" {
			continue
		}
		component := detectComponent(block[0].Text)
		g, ok := groups[fingerprint]
		if !ok {
			g = &model.LogStackTraceGroup{Fingerprint: fingerprint, Severity: model.SeverityDanger, Component: component, Exception: exception, FirstLine: block[0].No, LastLine: block[len(block)-1].No, Message: message, Cause: cause, Count: 0, Sample: stackSample(block)}
			groups[fingerprint] = g
		}
		g.Count++
		if block[0].No < g.FirstLine {
			g.FirstLine = block[0].No
		}
		if block[len(block)-1].No > g.LastLine {
			g.LastLine = block[len(block)-1].No
		}
	}
	out := make([]model.LogStackTraceGroup, 0, len(groups))
	for _, g := range groups {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Fingerprint < out[j].Fingerprint
		}
		return out[i].Count > out[j].Count
	})
	if len(out) > 16 {
		out = out[:16]
	}
	return out
}

func looksLikeExceptionStart(line string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.Contains(trimmed, "Exception") || strings.Contains(trimmed, "NoClassDefFoundError") || strings.Contains(trimmed, "NoSuchMethodError") || strings.Contains(trimmed, "UnsupportedClassVersionError") {
		return true
	}
	return false
}

func stackFingerprint(block []logLine) (fingerprint, exception, cause, message string) {
	parts := []string{}
	for _, item := range block {
		line := strings.TrimSpace(item.Text)
		if m := exceptionNameRe.FindStringSubmatch(line); len(m) > 1 && exception == "" {
			exception = m[1]
			if len(m) > 2 {
				message = strings.TrimSpace(m[2])
			}
		}
		if m := causedByRe.FindStringSubmatch(line); len(m) > 1 {
			cause = strings.TrimSpace(m[1])
		}
		if atLineRe.MatchString(line) {
			parts = append(parts, normalizeStackLine(line))
		}
		if len(parts) >= 8 {
			break
		}
	}
	if len(parts) == 0 && exception == "" {
		return "", "", "", ""
	}
	base := exception + "|" + strings.Join(parts, "|")
	if cause != "" {
		base += "|cause:" + normalizeStackLine(cause)
	}
	h := sha1.Sum([]byte(base))
	return hex.EncodeToString(h[:8]), exception, cause, message
}

func normalizeStackLine(line string) string {
	line = strings.TrimSpace(line)
	line = regexp.MustCompile(`:\d+`).ReplaceAllString(line, ":#")
	line = regexp.MustCompile(`\$\$Lambda\$\d+`).ReplaceAllString(line, "$$Lambda$#")
	return line
}

func stackSample(block []logLine) []string {
	out := []string{}
	for i, item := range block {
		if i >= 8 {
			break
		}
		out = append(out, trimLine(item.Text))
	}
	return out
}

func containsAny(original, lower string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(original, needle) || strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func isMissingDependencyLine(lower string) bool {
	return strings.Contains(lower, "unknown dependency") || strings.Contains(lower, "missing dependency") || strings.Contains(lower, "could not find dependency") || strings.Contains(lower, "depend") && strings.Contains(lower, "not found")
}

func captureMissingDependency(line string) string {
	m := missingPluginRe.FindStringSubmatch(line)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func detectComponent(line string) string {
	if m := couldNotLoadPluginRe.FindStringSubmatch(line); len(m) > 1 {
		return strings.TrimSuffix(m[1], ".jar")
	}
	if m := enablePluginRe.FindStringSubmatch(line); len(m) > 1 {
		return cleanPluginName(m[1])
	}
	if m := disablePluginRe.FindStringSubmatch(line); len(m) > 1 {
		return cleanPluginName(m[1])
	}
	if m := pluginBracketRe.FindStringSubmatch(line); len(m) > 1 {
		name := strings.TrimSpace(m[1])
		lower := strings.ToLower(name)
		if lower != "server" && lower != "minecraft" && lower != "async chat thread" && lower != "main" && lower != "warn" && lower != "error" && lower != "info" {
			return cleanPluginName(name)
		}
	}
	if strings.Contains(strings.ToLower(line), "minecraft") {
		return "Minecraft"
	}
	return ""
}

func cleanPluginName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, ".jar")
	name = strings.Trim(name, "'\"[]():")
	if strings.Contains(name, " ") {
		name = strings.Fields(name)[0]
	}
	return name
}

func readTail(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	st, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if st.Size() > limit {
		_, _ = file.Seek(st.Size()-limit, io.SeekStart)
	}
	return io.ReadAll(file)
}

func addSample(info *model.LogInfo, line string) {
	line = trimLine(line)
	if line == "" {
		return
	}
	if len(info.Samples) < 16 {
		info.Samples = append(info.Samples, line)
	}
}

func trimLine(line string) string {
	line = strings.TrimSpace(line)
	if len(line) > 280 {
		line = line[:280] + "..."
	}
	return line
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }

// ParseSinceDuration разбирает пользовательский аргумент --since: 24h, 30m, 7d.
func ParseSinceDuration(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("пустое значение --since")
	}
	if strings.HasSuffix(raw, "d") || strings.HasSuffix(raw, "д") {
		n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSuffix(raw, "d"), "д"))
		if err != nil {
			return 0, fmt.Errorf("неверное значение --since: %s", raw)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("неверное значение --since: %s", raw)
	}
	return d, nil
}

func FormatAnalysisText(info model.LogInfo, findings []model.Finding) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Log Doctor 2.0\n")
	fmt.Fprintf(&b, "Файл: %s\n", info.AnalyzedFile)
	if info.Options.LastRun {
		fmt.Fprintf(&b, "Режим: только последняя сессия запуска\n")
	}
	if info.Options.Since != "" {
		fmt.Fprintf(&b, "Режим since: %s\n", info.Options.Since)
	}
	fmt.Fprintf(&b, "Строк проанализировано: %d\n", info.LinesAnalyzed)
	fmt.Fprintf(&b, "WARN: %d · ERROR: %d · Exception/Caused by: %d · Long tick: %d\n\n", info.WarnCount, info.ErrorCount, info.ExceptionCount, info.LongTickCount)
	if len(info.Sessions) > 0 {
		fmt.Fprintf(&b, "Сессии запуска:\n")
		for _, s := range info.Sessions {
			marker := ""
			if info.SelectedSession != nil && info.SelectedSession.Index == s.Index {
				marker = " ← выбрана"
			}
			fmt.Fprintf(&b, "- #%d строки %d-%d · %s%s\n", s.Index, s.StartLine, s.EndLine, s.Reason, marker)
		}
		fmt.Fprintf(&b, "\n")
	}
	if info.MainRootCause != nil {
		fmt.Fprintf(&b, "Главная причина:\n- [%s] %s · компонент: %s · повторов: %d\n  Рекомендация: %s\n\n", info.MainRootCause.Severity, info.MainRootCause.Title, info.MainRootCause.Component, info.MainRootCause.Count, info.MainRootCause.Recommendation)
	}
	if len(info.RootCauses) > 0 {
		fmt.Fprintf(&b, "Предполагаемые root cause:\n")
		for _, rc := range info.RootCauses {
			fmt.Fprintf(&b, "- [%s] %s", rc.Severity, rc.Title)
			if rc.Component != "" && rc.Component != "не определён" {
				fmt.Fprintf(&b, " · компонент: %s", rc.Component)
			}
			fmt.Fprintf(&b, " · повторов: %d\n  Рекомендация: %s\n", rc.Count, rc.Recommendation)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(info.StackTraces) > 0 {
		fmt.Fprintf(&b, "Stack trace fingerprint:\n")
		for _, st := range info.StackTraces {
			fmt.Fprintf(&b, "- %s · %s · повторов: %d · строки: %d-%d", st.Fingerprint, st.Exception, st.Count, st.FirstLine, st.LastLine)
			if st.Component != "" {
				fmt.Fprintf(&b, " · компонент: %s", st.Component)
			}
			if st.Cause != "" {
				fmt.Fprintf(&b, "\n  Cause: %s", st.Cause)
			}
			fmt.Fprintf(&b, "\n")
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(info.IssueTypes) > 0 {
		fmt.Fprintf(&b, "Типы проблем:\n")
		for _, t := range info.IssueTypes {
			fmt.Fprintf(&b, "- [%s] %s · повторов: %d\n", t.Severity, t.Type, t.Count)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(info.Components) > 0 {
		fmt.Fprintf(&b, "Компоненты с проблемами:\n")
		for _, c := range info.Components {
			fmt.Fprintf(&b, "- %s · CRITICAL: %d · ERROR: %d · WARN: %d\n", c.Component, c.Critical, c.Errors, c.Warnings)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(info.Issues) > 0 {
		fmt.Fprintf(&b, "Сгруппированные проблемы:\n")
		for _, issue := range info.Issues {
			fmt.Fprintf(&b, "- [%s] %s · %s · повторов: %d\n  %s\n", issue.Severity, issue.Type, issue.Component, issue.Count, issue.Message)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(findings) > 0 {
		fmt.Fprintf(&b, "Находки:\n")
		for _, f := range findings {
			fmt.Fprintf(&b, "- [%s] %s (%s)\n", f.Severity, f.Title, f.ID)
		}
	}
	return b.String()
}

func FormatAnalysisJSON(info model.LogInfo, findings []model.Finding) ([]byte, error) {
	return json.MarshalIndent(struct {
		Logs     model.LogInfo   `json:"logs"`
		Findings []model.Finding `json:"findings"`
	}{Logs: info, Findings: findings}, "", "  ")
}
