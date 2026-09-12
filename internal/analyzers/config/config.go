package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gitflic.ru/skif4er/doctortools/internal/model"
)

// Analyze выполняет read-only аудит конфигурационных файлов Minecraft-сервера.
func Analyze(root string, mc model.MinecraftInfo) (model.ConfigInfo, []model.Finding) {
	info := model.ConfigInfo{Status: model.SeverityInfo}
	var findings []model.Finding

	analyzeServerProperties(root, mc, &info, &findings)
	analyzeYAMLConfigs(root, &info, &findings)
	analyzeVelocity(root, &info, &findings)
	analyzeBungee(root, &info, &findings)
	analyzeJSONConfigs(root, &info, &findings)
	finalize(&info)
	return info, findings
}

func analyzeServerProperties(root string, mc model.MinecraftInfo, info *model.ConfigInfo, findings *[]model.Finding) {
	path := filepath.Join(root, "server.properties")
	parsed, err := parsePropertiesFile(path)
	if err != nil {
		addFile(info, "server.properties", "properties", false, 0, err.Error())
		addCheck(info, findings, "config.server_properties.not_found", model.SeverityWarn, "server.properties", "server.properties", "файл", "server.properties не найден или не читается.", "Проверить, что указан путь к корню Minecraft-сервера и файл создан после первого запуска.")
		return
	}
	file := addFile(info, "server.properties", "properties", true, len(parsed.Values), "")
	file.Keys = sortedKeys(parsed.Values)
	file.DuplicateKeys = sortedKeysFromCounts(parsed.Counts, 2)
	file.DeprecatedKeys = deprecatedProperties(parsed.Values)
	file.UnknownKeys = unknownProperties(parsed.Values)
	info.Files[len(info.Files)-1] = file

	addCheck(info, findings, "config.server_properties.parsed", model.SeverityInfo, "server.properties", "server.properties", "файл", "server.properties успешно разобран как key=value конфигурация.", "Использовать Config Doctor после каждого изменения сетевых, gameplay и performance-параметров.")

	for _, key := range file.DuplicateKeys {
		addCheck(info, findings, "config.properties.duplicate_key."+sanitizeID(key), model.SeverityWarn, "server.properties", key, "дубликат", "Ключ "+key+" встречается в server.properties несколько раз.", "Оставить одно актуальное значение, чтобы исключить неоднозначность конфигурации.")
	}
	for _, key := range file.DeprecatedKeys {
		addCheck(info, findings, "config.properties.deprecated_key."+sanitizeID(key), model.SeverityWarn, "server.properties", key, "устаревший", "Ключ "+key+" выглядит устаревшим для современных Minecraft-серверов.", "Проверить актуальную документацию выбранной версии ядра и удалить неиспользуемый параметр.")
	}
	for _, key := range file.UnknownKeys {
		sev := model.SeverityInfo
		msg := "Ключ " + key + " не входит в базовый каталог известных server.properties параметров."
		rec := "Если это параметр новой версии ядра — можно игнорировать. Если это опечатка — исправить имя ключа."
		if likelyTypo(key) {
			sev = model.SeverityWarn
			msg = "Ключ " + key + " похож на опечатку в server.properties."
			rec = "Сравнить имя параметра с документацией Minecraft/Paper/Purpur и исправить опечатку."
		}
		addCheck(info, findings, "config.properties.unknown_key."+sanitizeID(key), sev, "server.properties", key, "неизвестный", msg, rec)
	}

	checkIntRange(info, findings, parsed.Values, "server.properties", "view-distance", 2, 32, 6, 12)
	checkIntRange(info, findings, parsed.Values, "server.properties", "simulation-distance", 2, 32, 4, 10)
	checkIntRange(info, findings, parsed.Values, "server.properties", "max-players", 1, 5000, 1, 300)
	checkIntRange(info, findings, parsed.Values, "server.properties", "server-port", 1, 65535, 1, 65535)
	checkIntRange(info, findings, parsed.Values, "server.properties", "query.port", 1, 65535, 1, 65535)
	requireBool(info, findings, parsed.Values, "online-mode", "server.properties")
	requireBool(info, findings, parsed.Values, "enable-rcon", "server.properties")
	requireBool(info, findings, parsed.Values, "white-list", "server.properties")
	requireBool(info, findings, parsed.Values, "enforce-whitelist", "server.properties")

	if strings.EqualFold(parsed.Values["enable-query"], "true") && strings.TrimSpace(parsed.Values["query.port"]) == "" {
		addCheck(info, findings, "config.properties.query_port.missing", model.SeverityInfo, "server.properties", "query.port", "требует проверки", "Query включён, но query.port не указан явно.", "Явно указать query.port, если сервер добавляется в мониторинги/листинги и требуется предсказуемый порт.")
	}
	if strings.TrimSpace(parsed.Values["level-name"]) == "" && len(mc.Worlds) > 0 {
		addCheck(info, findings, "config.properties.level_name.empty", model.SeverityWarn, "server.properties", "level-name", "пусто", "Параметр level-name пустой, хотя миры в проекте обнаружены.", "Указать основной мир в level-name или проверить корректность server.properties.")
	}
}

func analyzeYAMLConfigs(root string, info *model.ConfigInfo, findings *[]model.Finding) {
	files := []string{
		"bukkit.yml", "spigot.yml", "paper.yml", "paper-global.yml", "paper-world-defaults.yml", "purpur.yml",
		"config/paper-global.yml", "config/paper-world-defaults.yml", "config/purpur.yml",
	}
	for _, rel := range files {
		path := filepath.Join(root, rel)
		parsed, err := parseYAMLFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			addFile(info, rel, "yaml", false, 0, err.Error())
			addCheck(info, findings, "config.yaml.read_failed."+sanitizeID(rel), model.SeverityWarn, "yaml", rel, "ошибка чтения", "Не удалось прочитать конфиг "+rel+".", "Проверить права доступа и корректность файла.")
			continue
		}
		file := addFile(info, rel, "yaml", true, len(parsed.Keys), "")
		file.Keys = sortedKeysFromSet(parsed.Keys)
		file.DuplicateKeys = sortedKeysFromCounts(parsed.Counts, 2)
		info.Files[len(info.Files)-1] = file
		if parsed.Empty {
			addCheck(info, findings, "config.yaml.empty."+sanitizeID(rel), model.SeverityWarn, "yaml", rel, "пустой", "Конфигурационный файл "+rel+" пустой.", "Проверить, не был ли файл повреждён или очищен случайно.")
		}
		for _, line := range parsed.TabLines {
			addCheck(info, findings, "config.yaml.tabs."+sanitizeID(rel), model.SeverityDanger, "yaml", rel, "табуляция", fmt.Sprintf("В %s найдена табуляция в отступе на строке %d.", rel, line), "YAML чувствителен к отступам. Заменить табуляции на пробелы.")
		}
		for _, key := range file.DuplicateKeys {
			addCheck(info, findings, "config.yaml.duplicate_key."+sanitizeID(rel)+"."+sanitizeID(key), model.SeverityWarn, "yaml", key, "дубликат", "Ключ "+key+" повторяется в "+rel+".", "Оставить одно значение ключа или проверить вложенность YAML-блоков.")
		}
	}
}

func analyzeVelocity(root string, info *model.ConfigInfo, findings *[]model.Finding) {
	rel := "velocity.toml"
	path := filepath.Join(root, rel)
	parsed, err := parseTOMLFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		addFile(info, rel, "toml", false, 0, err.Error())
		addCheck(info, findings, "config.toml.read_failed.velocity", model.SeverityWarn, "velocity", rel, "ошибка чтения", "Не удалось прочитать velocity.toml.", "Проверить права доступа и синтаксис TOML.")
		return
	}
	file := addFile(info, rel, "toml", true, len(parsed.Values), "")
	file.Keys = sortedKeys(parsed.Values)
	file.DuplicateKeys = sortedKeysFromCounts(parsed.Counts, 2)
	info.Files[len(info.Files)-1] = file
	addCheck(info, findings, "config.velocity.detected", model.SeverityInfo, "velocity", rel, "найдено", "Обнаружен velocity.toml.", "Проверить bind, player-info-forwarding-mode, forwarding-secret-file и backend-секции.")
	for _, key := range file.DuplicateKeys {
		addCheck(info, findings, "config.toml.duplicate_key.velocity."+sanitizeID(key), model.SeverityWarn, "velocity", key, "дубликат", "Ключ "+key+" повторяется в velocity.toml.", "Оставить одно значение или проверить секцию TOML.")
	}
	mode := trimQuotes(parsed.Values["player-info-forwarding-mode"])
	if mode == "" {
		addCheck(info, findings, "config.velocity.forwarding_mode.missing", model.SeverityWarn, "velocity", "player-info-forwarding-mode", "не задан", "В velocity.toml не найден player-info-forwarding-mode.", "Для современных Paper/Purpur backend обычно используется modern forwarding с секретом.")
	} else if !oneOf(mode, "none", "legacy", "bungeeguard", "modern") {
		addCheck(info, findings, "config.velocity.forwarding_mode.invalid", model.SeverityDanger, "velocity", "player-info-forwarding-mode", "ошибка", "Неподдерживаемое значение player-info-forwarding-mode: "+mode+".", "Использовать одно из значений: none, legacy, bungeeguard, modern.")
	} else if mode == "modern" {
		secret := trimQuotes(parsed.Values["forwarding-secret-file"])
		if secret == "" {
			secret = "forwarding.secret"
		}
		if _, err := os.Stat(filepath.Join(root, secret)); err != nil {
			addCheck(info, findings, "config.velocity.secret_file.missing", model.SeverityDanger, "velocity", "forwarding-secret-file", "отсутствует", "Velocity modern forwarding включён, но файл секрета не найден: "+secret+".", "Создать forwarding secret и настроить backend на тот же секрет.")
		}
	}
	bind := trimQuotes(parsed.Values["bind"])
	if bind == "" {
		addCheck(info, findings, "config.velocity.bind.missing", model.SeverityWarn, "velocity", "bind", "не задан", "В velocity.toml не найден bind.", "Явно указать bind, например 0.0.0.0:25565 для публичного proxy или внутренний IP для закрытой сети.")
	}
}

func analyzeBungee(root string, info *model.ConfigInfo, findings *[]model.Finding) {
	rels := []string{"config.yml", "bungee/config.yml", "waterfall/config.yml"}
	for _, rel := range rels {
		path := filepath.Join(root, rel)
		parsed, err := parseYAMLFile(path)
		if err != nil {
			continue
		}
		textBytes, _ := os.ReadFile(path)
		text := strings.ToLower(string(textBytes))
		if !strings.Contains(text, "ip_forward") && !strings.Contains(text, "listeners:") && !strings.Contains(text, "priorities:") {
			continue
		}
		file := addFile(info, rel, "bungee-yaml", true, len(parsed.Keys), "")
		file.Keys = sortedKeysFromSet(parsed.Keys)
		file.DuplicateKeys = sortedKeysFromCounts(parsed.Counts, 2)
		info.Files[len(info.Files)-1] = file
		addCheck(info, findings, "config.bungee.detected", model.SeverityInfo, "bungee", rel, "найдено", "Обнаружена конфигурация BungeeCord/Waterfall.", "Проверить ip_forward, online_mode и backend online-mode.")
		if !strings.Contains(text, "ip_forward: true") {
			addCheck(info, findings, "config.bungee.ip_forward.not_true", model.SeverityDanger, "bungee", "ip_forward", "опасно", "В BungeeCord/Waterfall не найдено ip_forward: true.", "Для передачи UUID/IP на backend включить ip_forward: true и правильно настроить backend.")
		}
		return
	}
}

func analyzeJSONConfigs(root string, info *model.ConfigInfo, findings *[]model.Finding) {
	files := []string{"ops.json", "whitelist.json", "banned-players.json", "banned-ips.json", "usercache.json", "permissions.json"}
	for _, rel := range files {
		path := filepath.Join(root, rel)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		file := addFile(info, rel, "json", true, 0, "")
		var v any
		if err := json.Unmarshal(data, &v); err != nil {
			file.Valid = false
			file.Error = err.Error()
			addCheck(info, findings, "config.json.invalid."+sanitizeID(rel), model.SeverityDanger, "json", rel, "ошибка синтаксиса", "JSON-файл "+rel+" не разбирается.", "Исправить JSON-синтаксис перед запуском сервера, иначе whitelist/ops/ban-листы могут не примениться.")
		}
		info.Files[len(info.Files)-1] = file
	}
}

func checkIntRange(info *model.ConfigInfo, findings *[]model.Finding, values map[string]string, file, key string, min, max, recommendedMin, recommendedMax int) {
	value, ok := values[key]
	if !ok || strings.TrimSpace(value) == "" {
		return
	}
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		addCheck(info, findings, "config.properties.invalid_int."+sanitizeID(key), model.SeverityDanger, "server.properties", key, "ошибка типа", "Параметр "+key+" должен быть числом, но содержит: "+value+".", "Исправить значение на целое число в допустимом диапазоне.")
		return
	}
	if n < min || n > max {
		addCheck(info, findings, "config.properties.out_of_range."+sanitizeID(key), model.SeverityDanger, "server.properties", key, "вне диапазона", fmt.Sprintf("Параметр %s=%d вне технически допустимого диапазона %d..%d.", key, n, min, max), "Проверить значение и вернуть его в допустимый диапазон.")
		return
	}
	if n < recommendedMin || n > recommendedMax {
		addCheck(info, findings, "config.properties.out_of_recommended_range."+sanitizeID(key), model.SeverityWarn, "server.properties", key, "требует проверки", fmt.Sprintf("Параметр %s=%d выходит за безопасный стартовый диапазон %d..%d.", key, n, recommendedMin, recommendedMax), "Проверить значение под реальную нагрузку, онлайн, режим сервера и железо.")
	}
}

func requireBool(info *model.ConfigInfo, findings *[]model.Finding, values map[string]string, key, file string) {
	value, ok := values[key]
	if !ok || strings.TrimSpace(value) == "" {
		return
	}
	v := strings.ToLower(strings.TrimSpace(value))
	if v != "true" && v != "false" {
		addCheck(info, findings, "config.properties.invalid_bool."+sanitizeID(key), model.SeverityDanger, "server.properties", key, "ошибка типа", "Параметр "+key+" должен быть true или false, но содержит: "+value+".", "Исправить значение на true или false.")
	}
}

type parsedProperties struct {
	Values map[string]string
	Counts map[string]int
}

func parsePropertiesFile(path string) (parsedProperties, error) {
	f, err := os.Open(path)
	if err != nil {
		return parsedProperties{}, err
	}
	defer f.Close()
	res := parsedProperties{Values: map[string]string{}, Counts: map[string]int{}}
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		idx := strings.IndexAny(line, "=:")
		if idx < 0 {
			res.Counts[line]++
			res.Values[line] = ""
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		if key == "" {
			continue
		}
		res.Counts[key]++
		res.Values[key] = value
	}
	return res, s.Err()
}

type parsedYAML struct {
	Keys     map[string]bool
	Counts   map[string]int
	TabLines []int
	Empty    bool
}

func parseYAMLFile(path string) (parsedYAML, error) {
	f, err := os.Open(path)
	if err != nil {
		return parsedYAML{}, err
	}
	defer f.Close()
	res := parsedYAML{Keys: map[string]bool{}, Counts: map[string]int{}, Empty: true}
	s := bufio.NewScanner(f)
	stack := []string{}
	lineNo := 0
	for s.Scan() {
		lineNo++
		raw := s.Text()
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		res.Empty = false
		if strings.HasPrefix(raw, "\t") || strings.Contains(raw, "\t") && strings.TrimLeft(raw, "\t") != raw {
			res.TabLines = append(res.TabLines, lineNo)
		}
		if strings.HasPrefix(trimmed, "-") {
			continue
		}
		idx := strings.Index(trimmed, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:idx])
		if key == "" || strings.ContainsAny(key, "{}[]") {
			continue
		}
		indent := leadingSpaces(raw) / 2
		if indent < 0 {
			indent = 0
		}
		if indent < len(stack) {
			stack = stack[:indent]
		}
		if indent > len(stack) {
			indent = len(stack)
		}
		pathParts := append(append([]string{}, stack...), key)
		full := strings.Join(pathParts, ".")
		res.Keys[full] = true
		res.Counts[full]++
		if strings.TrimSpace(trimmed[idx+1:]) == "" {
			if indent == len(stack) {
				stack = append(stack, key)
			} else if indent < len(stack) {
				stack = append(stack[:indent], key)
			}
		}
	}
	return res, s.Err()
}

type parsedTOML struct {
	Values map[string]string
	Counts map[string]int
}

func parseTOMLFile(path string) (parsedTOML, error) {
	f, err := os.Open(path)
	if err != nil {
		return parsedTOML{}, err
	}
	defer f.Close()
	res := parsedTOML{Values: map[string]string{}, Counts: map[string]int{}}
	section := ""
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.Contains(line, "]") {
			section = strings.Trim(line, "[] ")
			res.Counts["["+section+"]"]++
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := stripInlineComment(strings.TrimSpace(line[idx+1:]))
		full := key
		if section != "" {
			full = section + "." + key
		}
		res.Counts[full]++
		res.Values[full] = value
		if section == "" {
			res.Values[key] = value
		}
	}
	return res, s.Err()
}

func addFile(info *model.ConfigInfo, path, kind string, valid bool, keyCount int, errText string) model.ConfigFileInfo {
	file := model.ConfigFileInfo{Path: path, Kind: kind, Valid: valid, KeyCount: keyCount, Error: errText}
	info.Files = append(info.Files, file)
	return file
}

func addCheck(info *model.ConfigInfo, findings *[]model.Finding, id string, sev model.Severity, area, target, status, msg, rec string) {
	check := model.ConfigCheck{ID: id, Severity: sev, Area: area, Target: target, Status: status, Message: msg, Recommendation: rec, File: target}
	info.Checks = append(info.Checks, check)
	if sev.Rank() >= model.SeverityWarn.Rank() {
		info.Recommendations = append(info.Recommendations, rec)
	}
	*findings = append(*findings, model.Finding{ID: id, Severity: sev, Category: "конфигурация", Title: titleFor(id, area), Message: msg, Recommendation: rec, File: target})
}

func finalize(info *model.ConfigInfo) {
	status := model.SeverityInfo
	for _, check := range info.Checks {
		if check.Severity.Rank() > status.Rank() {
			status = check.Severity
		}
	}
	info.Status = status
	sort.SliceStable(info.Files, func(i, j int) bool { return info.Files[i].Path < info.Files[j].Path })
	sort.SliceStable(info.Checks, func(i, j int) bool {
		if info.Checks[i].Severity.Rank() == info.Checks[j].Severity.Rank() {
			return info.Checks[i].ID < info.Checks[j].ID
		}
		return info.Checks[i].Severity.Rank() > info.Checks[j].Severity.Rank()
	})
	info.Recommendations = uniqueStrings(info.Recommendations)
}

func FormatAnalysisText(info model.ConfigInfo, findings []model.Finding) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Config Doctor\n")
	fmt.Fprintf(&b, "Статус: %s\n", info.Status)
	fmt.Fprintf(&b, "Файлов конфигурации: %d\n", len(info.Files))
	fmt.Fprintf(&b, "Проверок: %d\n\n", len(info.Checks))
	if len(info.Files) > 0 {
		fmt.Fprintf(&b, "Файлы:\n")
		for _, f := range info.Files {
			state := "ok"
			if !f.Valid {
				state = "problem"
			}
			fmt.Fprintf(&b, "- %s · %s · ключей: %d · %s\n", f.Path, f.Kind, f.KeyCount, state)
			if len(f.DuplicateKeys) > 0 {
				fmt.Fprintf(&b, "  - дубли: %s\n", strings.Join(f.DuplicateKeys, ", "))
			}
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(findings) > 0 {
		fmt.Fprintf(&b, "Находки:\n")
		for _, f := range findings {
			fmt.Fprintf(&b, "- [%s] %s — %s\n", f.Severity, f.ID, f.Message)
			if f.Recommendation != "" {
				fmt.Fprintf(&b, "  Рекомендация: %s\n", f.Recommendation)
			}
		}
	}
	return b.String()
}

func knownServerProperties() map[string]bool {
	keys := []string{"accepts-transfers", "allow-flight", "allow-nether", "broadcast-console-to-ops", "broadcast-rcon-to-ops", "bug-report-link", "debug", "difficulty", "enable-command-block", "enable-jmx-monitoring", "enable-query", "enable-rcon", "enable-status", "enforce-secure-profile", "enforce-whitelist", "entity-broadcast-range-percentage", "force-gamemode", "function-permission-level", "gamemode", "generate-structures", "generator-settings", "hardcore", "hide-online-players", "initial-disabled-packs", "initial-enabled-packs", "level-name", "level-seed", "level-type", "log-ips", "max-chained-neighbor-updates", "max-players", "max-tick-time", "max-world-size", "motd", "network-compression-threshold", "online-mode", "op-permission-level", "pause-when-empty-seconds", "player-idle-timeout", "prevent-proxy-connections", "pvp", "query.port", "rate-limit", "rcon.password", "rcon.port", "region-file-compression", "require-resource-pack", "resource-pack", "resource-pack-id", "resource-pack-prompt", "resource-pack-sha1", "server-ip", "server-port", "simulation-distance", "spawn-animals", "spawn-monsters", "spawn-npcs", "spawn-protection", "sync-chunk-writes", "text-filtering-config", "text-filtering-version", "use-native-transport", "view-distance", "white-list"}
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}

func deprecatedProperties(values map[string]string) []string {
	deprecated := map[string]bool{"snooper-enabled": true, "max-build-height": true, "announce-player-achievements": true, "enable-command-block-output": true}
	var out []string
	for key := range values {
		if deprecated[key] {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

func unknownProperties(values map[string]string) []string {
	known := knownServerProperties()
	var out []string
	for key := range values {
		if !known[key] {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

func likelyTypo(key string) bool {
	candidates := []string{"online-mode", "view-distance", "simulation-distance", "server-port", "enable-rcon", "white-list", "max-players"}
	compact := strings.NewReplacer("-", "", "_", "", ".", "").Replace(strings.ToLower(key))
	for _, c := range candidates {
		cc := strings.NewReplacer("-", "", "_", "", ".", "").Replace(c)
		if compact == cc && key != c {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedKeysFromSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedKeysFromCounts(m map[string]int, min int) []string {
	out := []string{}
	for k, c := range m {
		if c >= min {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func leadingSpaces(s string) int {
	count := 0
	for _, r := range s {
		if r == ' ' {
			count++
			continue
		}
		break
	}
	return count
}

func stripInlineComment(s string) string {
	inQuote := false
	quote := rune(0)
	for i, r := range s {
		if r == '\'' || r == '"' {
			if !inQuote {
				inQuote = true
				quote = r
			} else if quote == r {
				inQuote = false
			}
		}
		if r == '#' && !inQuote {
			return strings.TrimSpace(s[:i])
		}
	}
	return strings.TrimSpace(s)
}

func trimQuotes(s string) string {
	return strings.Trim(strings.TrimSpace(s), "\"'")
}

func oneOf(v string, allowed ...string) bool {
	for _, a := range allowed {
		if strings.EqualFold(v, a) {
			return true
		}
	}
	return false
}

func sanitizeID(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "unknown"
	}
	return out
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func titleFor(id, area string) string {
	if strings.Contains(id, "duplicate_key") {
		return "Дублирующийся ключ конфигурации"
	}
	if strings.Contains(id, "invalid_bool") || strings.Contains(id, "invalid_int") || strings.Contains(id, "out_of_range") {
		return "Некорректное значение конфигурации"
	}
	if strings.Contains(id, "unknown_key") {
		return "Неизвестный ключ конфигурации"
	}
	if strings.Contains(id, "deprecated_key") {
		return "Устаревший ключ конфигурации"
	}
	if strings.Contains(id, "velocity") {
		return "Проверка Velocity-конфигурации"
	}
	if strings.Contains(id, "bungee") {
		return "Проверка BungeeCord/Waterfall-конфигурации"
	}
	return "Проверка конфигурации: " + area
}
