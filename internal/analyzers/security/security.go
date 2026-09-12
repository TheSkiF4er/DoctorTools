package security

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gitflic.ru/skif4er/doctortools/internal/model"
)

// Analyze выполняет read-only аудит безопасности Minecraft-серверного проекта.
func Analyze(root string, system model.SystemInfo, mc model.MinecraftInfo, plugins []model.PluginInfo) (model.SecurityInfo, []model.Finding) {
	info := model.SecurityInfo{Status: model.SeverityInfo, Score: 100}
	var findings []model.Finding

	checkOnlineMode(&info, mc)
	checkRCON(&info, mc)
	checkWhitelist(&info, mc)
	checkCommandBlocksAndSpawn(&info, mc)
	checkProxyExposure(root, &info, mc)
	checkForwardingSecretPermissions(root, &info)
	checkQueryAndPorts(&info, mc)
	checkPermissions(root, &info)
	checkSensitiveFiles(root, &info)
	checkSecurityPlugins(&info, plugins)
	scanSecrets(root, &info)
	buildFindings(&info, &findings)
	finalize(&info)
	return info, findings
}

func checkOnlineMode(info *model.SecurityInfo, mc model.MinecraftInfo) {
	props := mc.Properties
	value := strings.ToLower(strings.TrimSpace(props["online-mode"]))
	if value == "" {
		addCheck(info, "security.online_mode.missing", model.SeverityWarn, "аутентификация", "server.properties", "требует проверки", "Параметр online-mode не найден в server.properties.", "Проверить, что server.properties сформирован корректно и явно содержит online-mode=true/false.", "server.properties")
		return
	}
	if value == "true" {
		addCheck(info, "security.online_mode.enabled", model.SeverityInfo, "аутентификация", "online-mode", "норма", "online-mode=true: сервер проверяет лицензированные аккаунты через штатную схему Minecraft.", "Для одиночного публичного сервера это рекомендуемый базовый режим.", "server.properties")
		return
	}
	if value == "false" && !mc.ProxyForwarded {
		addCheck(info, "security.online_mode.false_without_proxy", model.SeverityCritical, "аутентификация", "online-mode", "опасно", "online-mode=false без признаков Velocity/Bungee forwarding позволяет входить неподтверждённым аккаунтам.", "Включить online-mode=true или корректно настроить proxy forwarding и закрыть backend от прямого входа.", "server.properties")
		return
	}
	if value == "false" && mc.ProxyForwarded {
		addCheck(info, "security.online_mode.false_with_proxy", model.SeverityWarn, "аутентификация", "online-mode", "требует проверки", "online-mode=false используется вместе с признаками proxy forwarding.", "Проверить forwarding secret, force-key-authentication и firewall: backend должен принимать подключения только от proxy.", "server.properties")
	}
}

func checkRCON(info *model.SecurityInfo, mc model.MinecraftInfo) {
	props := mc.Properties
	if !strings.EqualFold(props["enable-rcon"], "true") {
		addCheck(info, "security.rcon.disabled", model.SeverityInfo, "удалённое управление", "RCON", "норма", "RCON отключён в server.properties.", "Оставлять RCON выключенным, если он не нужен для автоматизации.", "server.properties")
		return
	}
	password := strings.TrimSpace(props["rcon.password"])
	sev := model.SeverityDanger
	msg := "RCON включён. Это открывает удалённое управление сервером и требует строгого ограничения доступа."
	rec := "Ограничить RCON через firewall/VPN, использовать сильный уникальный пароль и не публиковать порт наружу без необходимости."
	if password == "" || isWeakPassword(password) {
		sev = model.SeverityCritical
		msg = "RCON включён, но пароль пустой или выглядит слабым/типовым."
		rec = "Сразу заменить rcon.password на длинный случайный пароль или отключить RCON. Ограничить rcon.port по firewall."
	}
	addCheck(info, "security.rcon.enabled", sev, "удалённое управление", "RCON", "опасно", msg, rec, "server.properties")
	if port := strings.TrimSpace(props["rcon.port"]); port != "" {
		addCheck(info, "security.rcon.port_declared", model.SeverityInfo, "удалённое управление", "rcon.port", "информация", "RCON-порт указан: "+port+".", "Проверить, что порт доступен только доверенным IP или локальной машине.", "server.properties")
	}
}

func checkWhitelist(info *model.SecurityInfo, mc model.MinecraftInfo) {
	props := mc.Properties
	if strings.EqualFold(props["white-list"], "true") && !strings.EqualFold(props["enforce-whitelist"], "true") {
		addCheck(info, "security.whitelist.not_enforced", model.SeverityWarn, "доступ игроков", "whitelist", "требует проверки", "Whitelist включён, но enforce-whitelist=false.", "Для закрытых серверов включить enforce-whitelist=true, чтобы изменения whitelist применялись строже.", "server.properties")
	}
	if strings.EqualFold(props["white-list"], "false") && strings.EqualFold(props["online-mode"], "false") {
		addCheck(info, "security.whitelist.disabled_with_offline_mode", model.SeverityDanger, "доступ игроков", "whitelist", "опасно", "Whitelist выключен при online-mode=false.", "Для непубличного backend-сервера включить whitelist/firewall или закрыть прямой вход через сетевые правила.", "server.properties")
	}
}

func checkCommandBlocksAndSpawn(info *model.SecurityInfo, mc model.MinecraftInfo) {
	props := mc.Properties
	if strings.EqualFold(props["enable-command-block"], "true") {
		addCheck(info, "security.command_blocks.enabled", model.SeverityWarn, "игровая логика", "command blocks", "требует проверки", "Командные блоки включены.", "Оставлять enable-command-block=true только при реальной необходимости и контролируемом доступе к картам/командам.", "server.properties")
	}
	if strings.EqualFold(props["spawn-protection"], "0") {
		addCheck(info, "security.spawn_protection.disabled", model.SeverityInfo, "защита мира", "spawn-protection", "информация", "spawn-protection=0.", "Проверить, что спавн защищён WorldGuard/claims/region-системой или не требует защиты по дизайну сервера.", "server.properties")
	}
}

func checkProxyExposure(root string, info *model.SecurityInfo, mc model.MinecraftInfo) {
	props := mc.Properties
	bind := strings.TrimSpace(props["server-ip"])
	port := strings.TrimSpace(props["server-port"])
	if port == "" {
		port = "25565"
	}
	if mc.ProxyForwarded && strings.EqualFold(props["online-mode"], "false") {
		if bind == "" || bind == "0.0.0.0" || bind == "::" {
			addCheck(info, "security.proxy.backend_public_bind", model.SeverityDanger, "proxy/backend", "server-ip", "опасно", "Backend-сервер с online-mode=false выглядит привязанным ко всем интерфейсам.", "Для backend за Velocity/BungeeCord привязать сервер к 127.0.0.1/внутреннему IP или закрыть порт firewall от внешнего доступа.", "server.properties")
		}
		if !hasForwardingSecret(root) {
			addCheck(info, "security.proxy.forwarding_secret.not_found", model.SeverityWarn, "proxy/backend", "forwarding secret", "требует проверки", "Не найден forwarding.secret или явный секрет Velocity рядом с проектом.", "Проверить, что Velocity modern forwarding использует секрет и backend не принимает прямые подключения.", "forwarding.secret")
		}
	}
	addCheck(info, "security.network.server_port", model.SeverityInfo, "сеть", "server-port", "информация", fmt.Sprintf("Основной порт сервера: %s, bind: %s.", port, valueOr(bind, "не задан / все интерфейсы")), "Проверить firewall и убедиться, что наружу опубликованы только необходимые порты.", "server.properties")
}

func checkForwardingSecretPermissions(root string, info *model.SecurityInfo) {
	for _, rel := range []string{"forwarding.secret", "velocity/forwarding.secret", "config/forwarding.secret"} {
		path := filepath.Join(root, rel)
		st, err := os.Stat(path)
		if err != nil {
			continue
		}
		mode := st.Mode().Perm()
		info.SensitiveFiles = append(info.SensitiveFiles, model.SecuritySensitiveFile{Path: rel, Kind: "Velocity forwarding secret", Mode: fmt.Sprintf("%#o", mode), Recommendation: "Хранить forwarding.secret с доступом только для пользователя запуска proxy/backend."})
		if mode&0004 != 0 {
			addCheck(info, "security.forwarding_secret.world_readable", model.SeverityDanger, "proxy/backend", "forwarding.secret", "опасно", fmt.Sprintf("Файл %s доступен на чтение всем пользователям (%#o).", rel, mode), "Ограничить права файла, например chmod 600 forwarding.secret, и проверить владельца файла.", rel)
		} else if mode&0040 != 0 {
			addCheck(info, "security.forwarding_secret.group_readable", model.SeverityWarn, "proxy/backend", "forwarding.secret", "требует проверки", fmt.Sprintf("Файл %s доступен группе (%#o).", rel, mode), "Проверить, что группа содержит только доверенных пользователей и сервисы.", rel)
		}
	}
}

func checkSensitiveFiles(root string, info *model.SecurityInfo) {
	sensitive := map[string]string{
		"server.properties":   "основная конфигурация сервера",
		"ops.json":            "операторы сервера",
		"whitelist.json":      "белый список",
		"banned-players.json": "баны игроков",
		"banned-ips.json":     "IP-баны",
		".env":                "переменные окружения",
		"config/discord.yml":  "интеграция Discord",
		"config/database.yml": "настройки базы данных",
	}
	keys := make([]string, 0, len(sensitive))
	for rel := range sensitive {
		keys = append(keys, rel)
	}
	sort.Strings(keys)
	for _, rel := range keys {
		path := filepath.Join(root, rel)
		st, err := os.Stat(path)
		if err != nil || st.IsDir() {
			continue
		}
		mode := st.Mode().Perm()
		info.SensitiveFiles = append(info.SensitiveFiles, model.SecuritySensitiveFile{Path: rel, Kind: sensitive[rel], Mode: fmt.Sprintf("%#o", mode), Recommendation: "Проверить владельца, права доступа и отсутствие публикации файла в репозитории/архиве."})
		if mode&0002 != 0 {
			addCheck(info, "security.sensitive_file.world_writable", model.SeverityCritical, "чувствительные файлы", rel, "опасно", fmt.Sprintf("Чувствительный файл %s доступен на запись всем пользователям (%#o).", rel, mode), "Убрать world-writable права и проверить владельца файла.", rel)
		} else if mode&0004 != 0 && rel == ".env" {
			addCheck(info, "security.sensitive_file.env_readable", model.SeverityWarn, "чувствительные файлы", rel, "требует проверки", fmt.Sprintf("Файл .env доступен на чтение всем пользователям (%#o).", mode), "Ограничить доступ к .env, если в нём есть токены, пароли или ключи API.", rel)
		}
	}
}

func checkQueryAndPorts(info *model.SecurityInfo, mc model.MinecraftInfo) {
	props := mc.Properties
	if strings.EqualFold(props["enable-query"], "true") {
		addCheck(info, "security.query.enabled", model.SeverityInfo, "сеть", "Query", "информация", "Query-протокол включён.", "Это нормально для мониторингов/листингов, но порт должен быть ожидаемо опубликован и не конфликтовать с другими сервисами.", "server.properties")
	}
	if strings.EqualFold(props["prevent-proxy-connections"], "false") && strings.EqualFold(props["online-mode"], "true") {
		addCheck(info, "security.prevent_proxy_connections.disabled", model.SeverityInfo, "аутентификация", "prevent-proxy-connections", "информация", "prevent-proxy-connections=false.", "При необходимости блокировать некоторые proxy/VPN-подключения можно рассмотреть включение, но это может давать false positive.", "server.properties")
	}
}

func checkPermissions(root string, info *model.SecurityInfo) {
	paths := []string{".", "server.properties", "eula.txt", "plugins", "logs", "config", "world", "ops.json", "whitelist.json", "banned-players.json", "banned-ips.json"}
	for _, rel := range paths {
		path := filepath.Join(root, rel)
		st, err := os.Stat(path)
		if err != nil {
			continue
		}
		mode := st.Mode().Perm()
		if mode&0002 != 0 {
			sev := model.SeverityDanger
			if rel == "." || rel == "server.properties" || strings.HasSuffix(rel, ".json") {
				sev = model.SeverityCritical
			}
			addCheck(info, "security.filesystem.world_writable", sev, "файловая система", rel, "опасно", fmt.Sprintf("Путь доступен на запись всем пользователям: %s (%#o).", rel, mode), "Убрать world-writable права, проверить владельца файлов и пользователя запуска сервера.", rel)
		}
	}
}

func checkSecurityPlugins(info *model.SecurityInfo, plugins []model.PluginInfo) {
	securityPlugins := []string{}
	adminPlugins := []string{}
	debugPlugins := []string{}
	rollbackPlugins := []string{}
	for _, p := range plugins {
		name := strings.ToLower(p.Name)
		jar := strings.ToLower(p.JarFile)
		joined := name + " " + jar + " " + strings.ToLower(strings.Join(p.Categories, " "))
		switch {
		case strings.Contains(joined, "luckperms") || strings.Contains(joined, "worldguard") || strings.Contains(joined, "anticheat") || strings.Contains(joined, "matrix") || strings.Contains(joined, "grim") || strings.Contains(joined, "vulcan") || strings.Contains(joined, "auth") || strings.Contains(joined, "login"):
			securityPlugins = append(securityPlugins, p.Name)
			info.PluginSignals = append(info.PluginSignals, model.SecurityPluginSignal{Name: p.Name, Kind: "security", Severity: model.SeverityInfo, Recommendation: "Проверить актуальность версии, конфигурацию и права доступа плагина."})
		}
		if strings.Contains(joined, "coreprotect") || strings.Contains(joined, "ledger") || strings.Contains(joined, "rollback") || strings.Contains(joined, "logblock") {
			rollbackPlugins = append(rollbackPlugins, p.Name)
			info.PluginSignals = append(info.PluginSignals, model.SecurityPluginSignal{Name: p.Name, Kind: "audit/rollback", Severity: model.SeverityInfo, Recommendation: "Проверить срок хранения логов действий и доступ к rollback-командам."})
		}
		if strings.Contains(joined, "admin") || strings.Contains(joined, "op") || strings.Contains(joined, "staff") || strings.Contains(joined, "panel") || strings.Contains(joined, "console") {
			adminPlugins = append(adminPlugins, p.Name)
			info.PluginSignals = append(info.PluginSignals, model.SecurityPluginSignal{Name: p.Name, Kind: "admin", Severity: model.SeverityWarn, Recommendation: "Проверить права команд, web-панель, токены и доступ модераторов."})
		}
		if strings.Contains(joined, "debug") || strings.Contains(joined, "test") || strings.Contains(joined, "devtools") || strings.Contains(joined, "developer") {
			debugPlugins = append(debugPlugins, p.Name)
			info.PluginSignals = append(info.PluginSignals, model.SecurityPluginSignal{Name: p.Name, Kind: "debug/dev", Severity: model.SeverityDanger, Recommendation: "Отключить debug/dev-плагины на production-сервере или жёстко ограничить доступ к командам."})
		}
	}
	sort.Strings(securityPlugins)
	sort.Strings(adminPlugins)
	sort.Strings(debugPlugins)
	sort.Strings(rollbackPlugins)
	if len(securityPlugins) == 0 {
		addCheck(info, "security.plugins.not_detected", model.SeverityInfo, "плагины", "security plugins", "информация", "CraftDoctor не нашёл очевидных security/permissions-плагинов по имени.", "Для публичного сервера обычно нужны права, защита территорий/спавна, аудит действий и rollback-инструменты — набор зависит от режима.", "plugins")
	} else {
		addCheck(info, "security.plugins.detected", model.SeverityInfo, "плагины", "security plugins", "информация", "Найдены потенциально связанные с безопасностью плагины: "+strings.Join(securityPlugins, ", ")+".", "Проверить их конфигурацию, права и актуальность версий.", "plugins")
	}
	if len(rollbackPlugins) == 0 {
		addCheck(info, "security.plugins.rollback_not_detected", model.SeverityWarn, "плагины", "rollback/audit", "требует проверки", "CraftDoctor не нашёл очевидных rollback/audit-плагинов по имени.", "Для публичного survival/RPG-сервера желательно иметь аудит действий и восстановление после грифа/ошибок администрации.", "plugins")
	}
	if len(adminPlugins) > 0 {
		addCheck(info, "security.plugins.admin_detected", model.SeverityWarn, "плагины", "admin plugins", "требует проверки", "Найдены потенциальные admin/staff/panel-плагины: "+strings.Join(adminPlugins, ", ")+".", "Проверить команды, permissions, web-доступ и отсутствие публичных токенов в конфигурации.", "plugins")
	}
	if len(debugPlugins) > 0 {
		addCheck(info, "security.plugins.debug_detected", model.SeverityDanger, "плагины", "debug/dev plugins", "опасно", "Найдены потенциальные debug/dev/test-плагины: "+strings.Join(debugPlugins, ", ")+".", "Убрать их из production-сервера или явно ограничить права доступа.", "plugins")
	}
}

func scanSecrets(root string, info *model.SecurityInfo) {
	patterns := []secretPattern{
		{kind: "Discord webhook", severity: model.SeverityDanger, re: regexp.MustCompile(`https://(?:ptb\.|canary\.)?discord(?:app)?\.com/api/webhooks/[A-Za-z0-9_\-]+/[A-Za-z0-9_\-]+`)},
		{kind: "Telegram bot token", severity: model.SeverityDanger, re: regexp.MustCompile(`\b\d{8,12}:[A-Za-z0-9_\-]{30,}\b`)},
		{kind: "Bearer token", severity: model.SeverityWarn, re: regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9_\-.=]{20,}`)},
		{kind: "API key", severity: model.SeverityWarn, re: regexp.MustCompile(`(?i)(api[_-]?key|access[_-]?key|secret[_-]?key)\s*[:=]\s*['\"]?[A-Za-z0-9_\-.=]{16,}`)},
		{kind: "Токен", severity: model.SeverityWarn, re: regexp.MustCompile(`(?i)(token|secret)\s*[:=]\s*['\"]?[A-Za-z0-9_\-.=]{20,}`)},
		{kind: "Пароль", severity: model.SeverityWarn, re: regexp.MustCompile(`(?i)(password|passwd|pwd|mysql-password|database-password|db-password)\s*[:=]\s*['\"]?[^\s'\"]{6,}`)},
		{kind: "Private key", severity: model.SeverityCritical, re: regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH |)PRIVATE KEY-----`)},
	}
	info.SecretsIgnoreFile, info.SecretsIgnorePatterns = loadSecretIgnore(root)
	maxFiles := 1800
	seen := 0
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && shouldSkipDir(d.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if seen >= maxFiles || !isConfigLike(path) {
			return nil
		}
		seen++
		scanSecretFile(root, path, patterns, info)
		return nil
	})
	info.SecretFilesScanned = seen
	if len(info.SecretsIgnorePatterns) > 0 {
		addCheck(info, "security.secrets.ignore_file_loaded", model.SeverityInfo, "секреты", ".craftdoctor-secrets-ignore", "информация", fmt.Sprintf("Загружен allowlist секретов: %d правил.", len(info.SecretsIgnorePatterns)), "Регулярно проверять, что allowlist не скрывает реальные утечки, а только осознанные false-positive.", info.SecretsIgnoreFile)
	}
	if len(info.Secrets) > 0 {
		info.Recommendations = append(info.Recommendations, "Перенести найденные секреты в переменные окружения, закрытые конфиги или секрет-хранилище; заменить уже опубликованные токены/пароли.")
	}
}

type secretPattern struct {
	kind     string
	severity model.Severity
	re       *regexp.Regexp
}

func scanSecretFile(root, path string, patterns []secretPattern, info *model.SecurityInfo) {
	st, err := os.Stat(path)
	if err != nil || st.Size() > 2*1024*1024 {
		return
	}
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	rel := relPath(root, path)
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") || strings.HasPrefix(trim, "//") {
			continue
		}
		for _, pattern := range patterns {
			if pattern.re.MatchString(line) {
				if matchesSecretIgnore(rel, line, pattern.kind, info.SecretsIgnorePatterns) {
					continue
				}
				evidence := redact(line)
				info.Secrets = append(info.Secrets, model.SecuritySecret{Kind: pattern.kind, Severity: pattern.severity, File: rel, Line: lineNo, Evidence: evidence, Recommendation: "Не хранить секреты в открытых конфигурациях репозитория/сервера; заменить значение и вынести его в безопасное место."})
				id := secretRuleID(pattern.kind)
				addCheck(info, id, pattern.severity, "секреты", pattern.kind, "опасно", fmt.Sprintf("В файле %s найдены признаки секрета: %s.", rel, pattern.kind), "Заменить секрет, убрать его из публичных файлов и проверить историю репозитория/бэкапов.", rel)
			}
		}
	}
}

func loadSecretIgnore(root string) (string, []string) {
	for _, rel := range []string{".craftdoctor-secrets-ignore", "config/.craftdoctor-secrets-ignore"} {
		path := filepath.Join(root, rel)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		patterns := make([]string, 0)
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			patterns = append(patterns, trimmed)
		}
		return rel, patterns
	}
	return "", nil
}

func matchesSecretIgnore(rel, line, kind string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	lowerRel := strings.ToLower(filepath.ToSlash(rel))
	lowerLine := strings.ToLower(line)
	lowerKind := strings.ToLower(kind)
	for _, raw := range patterns {
		pattern := strings.ToLower(strings.TrimSpace(raw))
		if pattern == "" {
			continue
		}
		if strings.HasPrefix(pattern, "kind:") && strings.Contains(lowerKind, strings.TrimSpace(strings.TrimPrefix(pattern, "kind:"))) {
			return true
		}
		if strings.HasPrefix(pattern, "file:") {
			p := strings.TrimSpace(strings.TrimPrefix(pattern, "file:"))
			if matchPathPattern(p, lowerRel) {
				return true
			}
		}
		if strings.HasPrefix(pattern, "line:") && strings.Contains(lowerLine, strings.TrimSpace(strings.TrimPrefix(pattern, "line:"))) {
			return true
		}
		if matchPathPattern(pattern, lowerRel) || strings.Contains(lowerLine, pattern) || strings.Contains(lowerKind, pattern) {
			return true
		}
	}
	return false
}

func matchPathPattern(pattern, rel string) bool {
	if ok, _ := filepath.Match(pattern, rel); ok {
		return true
	}
	if strings.Contains(pattern, "*") {
		if ok, _ := filepath.Match(strings.ReplaceAll(pattern, "**/", "*"), rel); ok {
			return true
		}
	}
	return strings.Contains(rel, strings.Trim(pattern, "*"))
}

func secretRuleID(kind string) string {
	switch kind {
	case "Discord webhook":
		return "security.secrets.discord_webhook"
	case "Telegram bot token":
		return "security.secrets.telegram_bot_token"
	case "Private key":
		return "security.secrets.private_key"
	case "API key":
		return "security.secrets.api_key"
	case "Bearer token":
		return "security.secrets.bearer_token"
	default:
		return "security.secrets.detected"
	}
}

func buildFindings(info *model.SecurityInfo, findings *[]model.Finding) {
	for _, check := range info.Checks {
		if check.Severity.Rank() < model.SeverityWarn.Rank() {
			continue
		}
		*findings = append(*findings, model.Finding{ID: check.ID, Severity: check.Severity, Category: "безопасность", Title: titleFromCheck(check), Message: check.Message, Recommendation: check.Recommendation, File: check.File})
	}
}

func FormatAnalysisText(info model.SecurityInfo, findings []model.Finding) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Security Doctor 2.0\n\n")
	fmt.Fprintf(&b, "Статус: %s\n", info.Status)
	fmt.Fprintf(&b, "Security score: %d/100\n", info.Score)
	fmt.Fprintf(&b, "Проверок: %d\n", len(info.Checks))
	fmt.Fprintf(&b, "Найденных секретов: %d\n", len(info.Secrets))
	fmt.Fprintf(&b, "Просканировано файлов на секреты: %d\n", info.SecretFilesScanned)
	if info.SecretsIgnoreFile != "" {
		fmt.Fprintf(&b, "Allowlist секретов: %s (%d правил)\n", info.SecretsIgnoreFile, len(info.SecretsIgnorePatterns))
	}
	fmt.Fprintf(&b, "Sensitive files: %d\n", len(info.SensitiveFiles))
	fmt.Fprintf(&b, "Plugin signals: %d\n", len(info.PluginSignals))
	fmt.Fprintf(&b, "Находок: %d\n\n", len(findings))
	if len(info.Checks) > 0 {
		fmt.Fprintf(&b, "Проверки:\n")
		for _, check := range info.Checks {
			fmt.Fprintf(&b, "- [%s] %s · %s · %s\n", check.Severity, check.ID, check.Area, check.Message)
			if check.Recommendation != "" {
				fmt.Fprintf(&b, "  Рекомендация: %s\n", check.Recommendation)
			}
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(info.Secrets) > 0 {
		fmt.Fprintf(&b, "Секреты:\n")
		for _, secret := range info.Secrets {
			fmt.Fprintf(&b, "- [%s] %s: %s:%d · %s\n", secret.Severity, secret.Kind, secret.File, secret.Line, secret.Evidence)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(info.SensitiveFiles) > 0 {
		fmt.Fprintf(&b, "Чувствительные файлы:\n")
		for _, file := range info.SensitiveFiles {
			fmt.Fprintf(&b, "- %s · %s · права %s\n", file.Path, file.Kind, file.Mode)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(info.PluginSignals) > 0 {
		fmt.Fprintf(&b, "Сигналы по плагинам:\n")
		for _, signal := range info.PluginSignals {
			fmt.Fprintf(&b, "- [%s] %s · %s · %s\n", signal.Severity, signal.Name, signal.Kind, signal.Recommendation)
		}
		fmt.Fprintf(&b, "\n")
	}
	return b.String()
}

func addCheck(info *model.SecurityInfo, id string, severity model.Severity, area, target, status, message, recommendation, file string) {
	info.Checks = append(info.Checks, model.SecurityCheck{ID: id, Severity: severity, Area: area, Target: target, Status: status, Message: message, Recommendation: recommendation, File: file})
}

func finalize(info *model.SecurityInfo) {
	status := model.SeverityInfo
	penalty := 0
	for _, check := range info.Checks {
		if check.Severity.Rank() > status.Rank() {
			status = check.Severity
		}
		penalty += securityPenalty(check.Severity)
	}
	for _, secret := range info.Secrets {
		if secret.Severity.Rank() > status.Rank() {
			status = secret.Severity
		}
		penalty += securityPenalty(secret.Severity)
	}
	for _, signal := range info.PluginSignals {
		penalty += securityPenalty(signal.Severity) / 2
	}
	info.Status = status
	info.Score = 100 - penalty
	if info.Score < 0 {
		info.Score = 0
	}
}

func securityPenalty(sev model.Severity) int {
	switch sev {
	case model.SeverityCritical:
		return 30
	case model.SeverityDanger:
		return 18
	case model.SeverityWarn:
		return 7
	default:
		return 0
	}
}

func titleFromCheck(check model.SecurityCheck) string {
	if check.Target != "" {
		return check.Area + ": " + check.Target
	}
	return check.Area
}

func isWeakPassword(password string) bool {
	p := strings.ToLower(strings.TrimSpace(password))
	if len(p) < 12 {
		return true
	}
	weak := map[string]bool{"changeme": true, "password": true, "admin": true, "minecraft": true, "123456": true, "qwerty": true, "rcon": true}
	return weak[p]
}

func hasForwardingSecret(root string) bool {
	checks := []string{"forwarding.secret", "velocity/forwarding.secret", "config/forwarding.secret"}
	for _, rel := range checks {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err == nil && strings.TrimSpace(string(data)) != "" {
			return true
		}
	}
	for _, rel := range []string{"velocity.toml", "config/velocity.toml"} {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err == nil {
			text := strings.ToLower(string(data))
			if strings.Contains(text, "player-info-forwarding-mode") && strings.Contains(text, "modern") && strings.Contains(text, "forwarding-secret") {
				return true
			}
		}
	}
	return false
}

func shouldSkipDir(name string) bool {
	lower := strings.ToLower(name)
	switch lower {
	case ".git", "cache", "libraries", "versions", "logs", "crash-reports", "world", "world_nether", "world_the_end", "region", "dynmap", "bluemap", "backup", "backups":
		return true
	default:
		return false
	}
}

func isConfigLike(path string) bool {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".jar") || strings.HasSuffix(lower, ".mca") || strings.HasSuffix(lower, ".dat") || strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".sqlite") || strings.HasSuffix(lower, ".db") {
		return false
	}
	ext := filepath.Ext(lower)
	switch ext {
	case ".yml", ".yaml", ".json", ".conf", ".properties", ".toml", ".env", ".txt", ".cfg", ".ini":
		return true
	default:
		base := filepath.Base(lower)
		return base == ".env" || base == "config" || base == "settings"
	}
}

func redact(line string) string {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) > 120 {
		trimmed = trimmed[:120] + "…"
	}
	parts := strings.SplitN(trimmed, "=", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]) + "=***"
	}
	parts = strings.SplitN(trimmed, ":", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]) + ": ***"
	}
	return "***"
}

func relPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
