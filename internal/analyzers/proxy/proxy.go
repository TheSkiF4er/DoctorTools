package proxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gitflic.ru/skif4er/doctortools/internal/model"
)

// Analyze выполняет read-only аудит proxy/network-слоя Minecraft-проекта.
func Analyze(root string, mc model.MinecraftInfo) (model.ProxyInfo, []model.Finding) {
	info := model.ProxyInfo{Status: model.SeverityInfo}
	var findings []model.Finding

	velocityFiles := existing(root, []string{"velocity.toml", "proxy/velocity.toml", "velocity/velocity.toml", "runtime/velocity/velocity.toml"})
	bungeeFiles := existing(root, []string{"bungeecord.yml", "bungee.yml", "waterfall.yml", "config.yml", "proxy/config.yml", "bungee/config.yml", "waterfall/config.yml"})

	for _, rel := range velocityFiles {
		cfg := parseVelocity(root, rel)
		info.Configs = append(info.Configs, cfg)
		info.ConfigFiles = append(info.ConfigFiles, rel)
		addType(&info, cfg.Type)
		analyzeVelocity(mc, cfg, &info)
	}
	for _, rel := range bungeeFiles {
		cfg, ok := parseBungee(root, rel)
		if !ok {
			continue
		}
		info.Configs = append(info.Configs, cfg)
		info.ConfigFiles = append(info.ConfigFiles, rel)
		addType(&info, cfg.Type)
		analyzeBungee(mc, cfg, &info)
	}

	info.Detected = len(info.Configs) > 0
	if info.Detected {
		addCheck(&info, "proxy.detected", model.SeverityInfo, "proxy", "network", "найдено", "В проекте найдены признаки proxy/network-конфигурации Minecraft-сервера.", "Проверить forwarding, backend bind, firewall и соответствие backend online-mode выбранной proxy-схеме.", "")
		buildNetworkMap(mc, &info)
	} else {
		addCheck(&info, "proxy.not_found", model.SeverityInfo, "proxy", "network", "не найдено", "Velocity/BungeeCord/Waterfall конфигурации рядом с проектом не обнаружены.", "Для одиночного сервера это нормально. Для сети серверов добавьте proxy-конфиги в проверяемый проект или сканируйте корень всей сети.", "")
		if strings.EqualFold(mc.Properties["online-mode"], "false") && !mc.ProxyForwarded {
			addCheck(&info, "proxy.backend.offline_without_proxy", model.SeverityCritical, "backend", "online-mode", "опасно", "server.properties содержит online-mode=false, но proxy forwarding или proxy-конфигурация не обнаружены.", "Включить online-mode=true либо корректно настроить Velocity/BungeeCord forwarding и закрыть backend-сервер от прямого доступа.", "server.properties")
		}
	}

	checkBackendProperties(mc, &info)
	finalize(&info)
	buildFindings(info, &findings)
	return info, findings
}

func parseVelocity(root, rel string) model.ProxyConfig {
	cfg := model.ProxyConfig{Type: "Velocity", File: rel, RawSettings: map[string]string{}}
	path := filepath.Join(root, rel)
	file, err := os.Open(path)
	if err != nil {
		return cfg
	}
	defer file.Close()

	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := stripComment(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[]")
			continue
		}
		key, value, ok := splitKeyValue(line)
		if !ok {
			continue
		}
		clean := cleanScalar(value)
		switch {
		case section == "" && key == "bind":
			cfg.Bind = clean
			cfg.RawSettings["bind"] = clean
		case section == "" && key == "player-info-forwarding-mode":
			cfg.ForwardingMode = clean
			cfg.RawSettings["player-info-forwarding-mode"] = clean
		case section == "" && key == "forwarding-secret-file":
			cfg.SecretFile = clean
			cfg.RawSettings["forwarding-secret-file"] = clean
		case section == "servers":
			cfg.Servers = append(cfg.Servers, serverFromAddress(key, clean))
		case section == "forced-hosts":
			cfg.ForcedHosts = append(cfg.ForcedHosts, model.ProxyForcedHost{Host: key, Servers: parseStringList(value)})
		}
	}
	if cfg.SecretFile == "" {
		cfg.SecretFile = "forwarding.secret"
	}
	cfg.SecretPresent = nonEmptyFile(filepath.Join(root, filepath.Dir(rel), cfg.SecretFile)) || nonEmptyFile(filepath.Join(root, cfg.SecretFile))
	sortServers(cfg.Servers)
	return cfg
}

func parseBungee(root, rel string) (model.ProxyConfig, bool) {
	cfg := model.ProxyConfig{Type: "BungeeCord/Waterfall", File: rel, RawSettings: map[string]string{}}
	path := filepath.Join(root, rel)
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, false
	}
	text := string(data)
	lower := strings.ToLower(text)
	looksLikeBungee := strings.Contains(lower, "ip_forward:") || strings.Contains(lower, "listeners:") || strings.Contains(lower, "priorities:") || strings.Contains(lower, "server_connect_timeout:")
	if !looksLikeBungee && filepath.Base(rel) == "config.yml" {
		return cfg, false
	}
	cfg.IPForward = firstYAMLValue(text, "ip_forward")
	cfg.OnlineMode = firstYAMLValue(text, "online_mode")
	cfg.Bind = firstYAMLValue(text, "host")
	cfg.Servers = parseBungeeServers(text)
	sortServers(cfg.Servers)
	return cfg, true
}

func analyzeVelocity(mc model.MinecraftInfo, cfg model.ProxyConfig, info *model.ProxyInfo) {
	addCheck(info, "proxy.velocity.detected", model.SeverityInfo, "velocity", cfg.File, "найдено", "Обнаружена конфигурация Velocity.", "Проверить bind, player-info-forwarding-mode, forwarding.secret и список backend-серверов.", cfg.File)
	if cfg.Bind == "" {
		addCheck(info, "proxy.velocity.bind.missing", model.SeverityWarn, "velocity", "bind", "требует проверки", "В velocity.toml не найден параметр bind.", "Указать ожидаемый bind, например 0.0.0.0:25565 для публичного proxy или конкретный IP.", cfg.File)
	} else if hostPart(cfg.Bind) == "127.0.0.1" || hostPart(cfg.Bind) == "localhost" {
		addCheck(info, "proxy.velocity.bind.localhost", model.SeverityWarn, "velocity", "bind", "локальный", "Velocity привязан к localhost. Внешние игроки не смогут подключиться напрямую к proxy.", "Для публичного proxy использовать внешний IP/0.0.0.0 и firewall/DDoS-защиту по ситуации.", cfg.File)
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.ForwardingMode))
	if mode == "" || mode == "none" {
		severity := model.SeverityWarn
		if strings.EqualFold(mc.Properties["online-mode"], "false") {
			severity = model.SeverityCritical
		}
		addCheck(info, "proxy.velocity.forwarding.none", severity, "velocity", "player-info-forwarding-mode", "опасно", "Velocity не использует modern forwarding или режим forwarding не найден.", "Для Paper/Purpur backend обычно нужен modern forwarding и корректный forwarding.secret.", cfg.File)
	} else if mode == "legacy" || mode == "bungeeguard" {
		addCheck(info, "proxy.velocity.forwarding.legacy", model.SeverityWarn, "velocity", "player-info-forwarding-mode", "устаревший режим", "Velocity использует legacy/bungeeguard forwarding.", "По возможности перейти на modern forwarding и синхронизировать forwarding secret с backend-конфигами.", cfg.File)
	} else if mode == "modern" {
		addCheck(info, "proxy.velocity.forwarding.modern", model.SeverityInfo, "velocity", "player-info-forwarding-mode", "норма", "Velocity использует modern forwarding.", "Проверить совпадение forwarding secret с Paper/Purpur backend-конфигурацией.", cfg.File)
		if !cfg.SecretPresent {
			addCheck(info, "proxy.velocity.secret.missing", model.SeverityCritical, "velocity", cfg.SecretFile, "не найдено", "Для modern forwarding не найден непустой forwarding secret file.", "Создать forwarding.secret, ограничить права доступа и синхронизировать secret с backend-серверами.", cfg.File)
		}
	}
	if len(cfg.Servers) == 0 {
		addCheck(info, "proxy.velocity.servers.empty", model.SeverityDanger, "velocity", "servers", "пусто", "В velocity.toml не найден список backend-серверов.", "Добавить backend-серверы в секцию [servers].", cfg.File)
	} else {
		addCheck(info, "proxy.velocity.servers.detected", model.SeverityInfo, "velocity", "servers", "найдено", fmt.Sprintf("В Velocity найдено backend-серверов: %d.", len(cfg.Servers)), "Проверить доступность backend-серверов, bind и firewall.", cfg.File)
		for _, srv := range cfg.Servers {
			checkBackendAddress(info, cfg.File, srv)
		}
		checkForcedHosts(info, cfg)
	}
}

func analyzeBungee(mc model.MinecraftInfo, cfg model.ProxyConfig, info *model.ProxyInfo) {
	addCheck(info, "proxy.bungee.detected", model.SeverityInfo, "bungee", cfg.File, "найдено", "Обнаружена конфигурация BungeeCord/Waterfall.", "Проверить ip_forward, online_mode, listeners.host, backend online-mode и firewall.", cfg.File)
	if strings.EqualFold(cfg.IPForward, "false") || cfg.IPForward == "" {
		severity := model.SeverityWarn
		if strings.EqualFold(mc.Properties["online-mode"], "false") {
			severity = model.SeverityCritical
		}
		addCheck(info, "proxy.bungee.ip_forward.disabled", severity, "bungee", "ip_forward", "опасно", "В BungeeCord/Waterfall не включён ip_forward или параметр не найден.", "Включить ip_forward: true и настроить backend под Bungee/Waterfall forwarding либо использовать Velocity modern forwarding.", cfg.File)
	} else {
		addCheck(info, "proxy.bungee.ip_forward.enabled", model.SeverityInfo, "bungee", "ip_forward", "норма", "В BungeeCord/Waterfall включён ip_forward.", "Проверить, что backend закрыт от прямого доступа и настроен bungeecord: true.", cfg.File)
	}
	if strings.EqualFold(cfg.OnlineMode, "false") {
		addCheck(info, "proxy.bungee.online_mode.disabled", model.SeverityDanger, "bungee", "online_mode", "опасно", "Proxy работает с online_mode=false.", "Для публичного сервера включить online_mode: true на proxy. Offline proxy допустим только для строго закрытых dev-сред.", cfg.File)
	}
	if len(cfg.Servers) == 0 {
		addCheck(info, "proxy.bungee.servers.empty", model.SeverityWarn, "bungee", "servers", "не найдено", "CraftDoctor не смог определить backend-серверы в BungeeCord/Waterfall config.yml.", "Проверить секцию servers и формат YAML; при необходимости добавить явные backend-адреса.", cfg.File)
	} else {
		for _, srv := range cfg.Servers {
			checkBackendAddress(info, cfg.File, srv)
		}
	}
}

func checkBackendProperties(mc model.MinecraftInfo, info *model.ProxyInfo) {
	if !info.Detected {
		return
	}
	online := strings.ToLower(strings.TrimSpace(mc.Properties["online-mode"]))
	serverIP := strings.TrimSpace(mc.Properties["server-ip"])
	if online == "true" {
		addCheck(info, "proxy.backend.online_mode.true", model.SeverityWarn, "backend", "online-mode", "требует проверки", "Backend server.properties содержит online-mode=true при наличии proxy-конфигурации.", "Для Velocity/Bungee backend обычно используют online-mode=false, а аутентификацию выполняет proxy. Проверить архитектуру сети.", "server.properties")
	}
	if online == "false" {
		addCheck(info, "proxy.backend.online_mode.false", model.SeverityInfo, "backend", "online-mode", "ожидаемо для proxy", "Backend работает с online-mode=false при наличии proxy-конфигурации.", "Убедиться, что backend не доступен напрямую из интернета и forwarding защищён secret/firewall.", "server.properties")
	}
	if serverIP == "" || serverIP == "0.0.0.0" {
		addCheck(info, "proxy.backend.bind.public_or_blank", model.SeverityWarn, "backend", "server-ip", "требует проверки", "Backend server-ip пустой или 0.0.0.0. При proxy-архитектуре backend может быть доступен не только proxy.", "Ограничить backend firewall-ом или привязать к 127.0.0.1/внутреннему IP, если proxy находится на той же машине/внутренней сети.", "server.properties")
	} else if isLoopback(serverIP) || isPrivateHost(serverIP) {
		addCheck(info, "proxy.backend.bind.private", model.SeverityInfo, "backend", "server-ip", "норма", "Backend привязан к локальному или приватному адресу.", "Дополнительно проверить firewall и список backend-серверов в proxy-конфигурации.", "server.properties")
	}
}

func checkBackendAddress(info *model.ProxyInfo, file string, srv model.ProxyServer) {
	host := strings.ToLower(strings.TrimSpace(srv.Host))
	if host == "" {
		host = strings.ToLower(hostPart(srv.Address))
	}
	if host == "0.0.0.0" || host == "" {
		addCheck(info, "proxy.backend.address.public", model.SeverityWarn, "backend", srv.Name, "требует проверки", fmt.Sprintf("Backend %s использует адрес %s.", srv.Name, srv.Address), "Для backend лучше использовать 127.0.0.1 или приватный IP, а публичный доступ закрыть firewall-ом.", file)
		return
	}
	if isLoopback(host) || isPrivateHost(host) {
		addCheck(info, "proxy.backend.address.private", model.SeverityInfo, "backend", srv.Name, "норма", fmt.Sprintf("Backend %s использует локальный/приватный адрес %s.", srv.Name, srv.Address), "Проверить, что порт backend не открыт для прямого внешнего подключения.", file)
		return
	}
	addCheck(info, "proxy.backend.address.external", model.SeverityWarn, "backend", srv.Name, "внешний адрес", fmt.Sprintf("Backend %s использует внешний адрес %s.", srv.Name, srv.Address), "Если backend находится на другой машине, ограничить доступ по firewall/VPN/security group только с proxy.", file)
}

func checkForcedHosts(info *model.ProxyInfo, cfg model.ProxyConfig) {
	if len(cfg.ForcedHosts) == 0 {
		return
	}
	known := map[string]bool{}
	for _, srv := range cfg.Servers {
		known[strings.ToLower(srv.Name)] = true
	}
	for _, fh := range cfg.ForcedHosts {
		for _, name := range fh.Servers {
			if !known[strings.ToLower(name)] {
				addCheck(info, "proxy.velocity.forced_host.unknown_server", model.SeverityWarn, "velocity", fh.Host, "битая ссылка", fmt.Sprintf("Forced host %s ссылается на неизвестный backend %s.", fh.Host, name), "Исправить forced-hosts или добавить отсутствующий backend в [servers].", cfg.File)
			}
		}
	}
}

func buildNetworkMap(mc model.MinecraftInfo, info *model.ProxyInfo) {
	seen := map[string]bool{}
	for _, cfg := range info.Configs {
		proxyID := strings.ToLower(cfg.Type) + ":" + cfg.File
		info.Nodes = append(info.Nodes, model.ProxyNode{ID: proxyID, Kind: cfg.Type, Address: cfg.Bind, File: cfg.File})
		seen[proxyID] = true
		for _, srv := range cfg.Servers {
			id := "backend:" + strings.ToLower(srv.Name)
			if !seen[id] {
				info.Nodes = append(info.Nodes, model.ProxyNode{ID: id, Kind: "backend", Address: srv.Address})
				seen[id] = true
			}
			info.Edges = append(info.Edges, model.ProxyEdge{From: proxyID, To: id, Kind: "routes_to"})
		}
	}
	if len(mc.Worlds) > 0 && !seen["backend:current"] {
		info.Nodes = append(info.Nodes, model.ProxyNode{ID: "backend:current", Kind: "current-server", Address: mc.Properties["server-ip"]})
	}
}

func finalize(info *model.ProxyInfo) {
	status := model.SeverityInfo
	for _, check := range info.Checks {
		if check.Severity.Rank() > status.Rank() {
			status = check.Severity
		}
	}
	info.Status = status
	if len(info.Recommendations) == 0 {
		info.Recommendations = []string{
			"Для proxy-сетей проверяйте три вещи вместе: forwarding mode, backend online-mode и сетевую доступность backend-портов.",
			"После изменений повторно запустите craftdoctor proxy scan и полный craftdoctor scan.",
		}
	}
	sort.SliceStable(info.Checks, func(i, j int) bool {
		if info.Checks[i].Severity.Rank() == info.Checks[j].Severity.Rank() {
			return info.Checks[i].ID < info.Checks[j].ID
		}
		return info.Checks[i].Severity.Rank() > info.Checks[j].Severity.Rank()
	})
	sort.Strings(info.ConfigFiles)
	sort.Strings(info.Types)
}

func buildFindings(info model.ProxyInfo, findings *[]model.Finding) {
	for _, check := range info.Checks {
		if check.Severity.Rank() < model.SeverityWarn.Rank() {
			continue
		}
		*findings = append(*findings, model.Finding{ID: check.ID, Severity: check.Severity, Category: "proxy/network", Title: titleForCheck(check), Message: check.Message, Recommendation: check.Recommendation, File: check.File})
	}
}

func FormatAnalysisText(info model.ProxyInfo, findings []model.Finding) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Proxy / Network Doctor\n")
	fmt.Fprintf(&b, "Статус: %s\n", info.Status)
	fmt.Fprintf(&b, "Proxy обнаружен: %t\n", info.Detected)
	fmt.Fprintf(&b, "Типы: %s\n", strings.Join(info.Types, ", "))
	fmt.Fprintf(&b, "Конфигов: %d\n", len(info.ConfigFiles))
	fmt.Fprintf(&b, "Проверок: %d\n", len(info.Checks))
	fmt.Fprintf(&b, "Узлов сети: %d\n", len(info.Nodes))
	fmt.Fprintf(&b, "Связей сети: %d\n", len(info.Edges))
	fmt.Fprintf(&b, "Находок: %d\n\n", len(findings))
	if len(info.Configs) > 0 {
		fmt.Fprintf(&b, "Конфигурации:\n")
		for _, cfg := range info.Configs {
			fmt.Fprintf(&b, "- %s · %s · bind=%s · forwarding=%s · servers=%d\n", cfg.Type, cfg.File, valueOrDash(cfg.Bind), valueOrDash(cfg.ForwardingMode), len(cfg.Servers))
		}
		fmt.Fprintf(&b, "\n")
	}
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
	if len(info.Edges) > 0 {
		fmt.Fprintf(&b, "Карта сети:\n")
		for _, edge := range info.Edges {
			fmt.Fprintf(&b, "- %s --%s--> %s\n", edge.From, edge.Kind, edge.To)
		}
	}
	return b.String()
}

func titleForCheck(check model.ProxyCheck) string {
	switch check.ID {
	case "proxy.backend.offline_without_proxy":
		return "Backend работает offline без proxy forwarding"
	case "proxy.velocity.secret.missing":
		return "Velocity forwarding secret не найден"
	case "proxy.velocity.forwarding.none":
		return "Velocity forwarding не настроен"
	case "proxy.velocity.forwarding.legacy":
		return "Velocity использует legacy forwarding"
	case "proxy.velocity.servers.empty":
		return "Velocity backend-серверы не настроены"
	case "proxy.velocity.forced_host.unknown_server":
		return "Forced host ссылается на неизвестный backend"
	case "proxy.bungee.ip_forward.disabled":
		return "BungeeCord/Waterfall ip_forward выключен"
	case "proxy.bungee.online_mode.disabled":
		return "Proxy работает с online_mode=false"
	case "proxy.backend.online_mode.true":
		return "Backend online-mode=true при proxy-конфигурации"
	case "proxy.backend.bind.public_or_blank":
		return "Backend может быть доступен напрямую"
	case "proxy.backend.address.public", "proxy.backend.address.external":
		return "Backend-адрес требует сетевой проверки"
	case "proxy.velocity.bind.missing":
		return "Velocity bind не найден"
	case "proxy.velocity.bind.localhost":
		return "Velocity привязан к localhost"
	}
	if check.Target != "" {
		return check.Area + ": " + check.Target
	}
	return check.Area
}

func existing(root string, candidates []string) []string {
	var out []string
	for _, rel := range candidates {
		if st, err := os.Stat(filepath.Join(root, rel)); err == nil && !st.IsDir() {
			out = append(out, rel)
		}
	}
	return out
}

func addType(info *model.ProxyInfo, typ string) {
	for _, existing := range info.Types {
		if existing == typ {
			return
		}
	}
	info.Types = append(info.Types, typ)
}

func addCheck(info *model.ProxyInfo, id string, severity model.Severity, area, target, status, message, recommendation, file string) {
	info.Checks = append(info.Checks, model.ProxyCheck{ID: id, Severity: severity, Area: area, Target: target, Status: status, Message: message, Recommendation: recommendation, File: file})
}

func stripComment(line string) string {
	inQuote := false
	quote := rune(0)
	for i, r := range line {
		if r == '\'' || r == '"' {
			if !inQuote {
				inQuote = true
				quote = r
			} else if quote == r {
				inQuote = false
			}
		}
		if r == '#' && !inQuote {
			line = line[:i]
			break
		}
	}
	return strings.TrimSpace(line)
}

func splitKeyValue(line string) (string, string, bool) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return strings.Trim(strings.TrimSpace(parts[0]), "\"'"), strings.TrimSpace(parts[1]), true
}

func cleanScalar(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, ",")
	return strings.Trim(value, "\"'")
}

func parseStringList(value string) []string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "[]")
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.Trim(strings.TrimSpace(part), "\"'")
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func serverFromAddress(name, address string) model.ProxyServer {
	host, port := splitHostPort(address)
	return model.ProxyServer{Name: name, Address: address, Host: host, Port: port}
}

func sortServers(list []model.ProxyServer) {
	sort.SliceStable(list, func(i, j int) bool { return list[i].Name < list[j].Name })
}

func nonEmptyFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir() && st.Size() > 0
}

func firstYAMLValue(text, key string) string {
	re := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(key) + `\s*:\s*['"]?([^'"#\r\n]+)`)
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func parseBungeeServers(text string) []model.ProxyServer {
	var servers []model.ProxyServer
	lines := strings.Split(text, "\n")
	inServers := false
	current := ""
	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if strings.HasPrefix(trim, "servers:") {
			inServers = true
			current = ""
			continue
		}
		if inServers && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			break
		}
		if !inServers {
			continue
		}
		if strings.HasSuffix(trim, ":") && !strings.Contains(trim, " ") {
			current = strings.TrimSuffix(trim, ":")
			continue
		}
		if current != "" && strings.HasPrefix(trim, "address:") {
			addr := strings.TrimSpace(strings.TrimPrefix(trim, "address:"))
			addr = strings.Trim(addr, "\"'")
			servers = append(servers, serverFromAddress(current, addr))
		}
	}
	return servers
}

func hostPart(address string) string {
	host, _ := splitHostPort(address)
	return host
}

func splitHostPort(address string) (string, string) {
	address = strings.TrimSpace(address)
	address = strings.Trim(address, "\"'")
	if strings.HasPrefix(address, "[") {
		idx := strings.Index(address, "]")
		if idx > 0 {
			host := strings.Trim(address[:idx+1], "[]")
			rest := strings.TrimPrefix(address[idx+1:], ":")
			return host, rest
		}
	}
	parts := strings.Split(address, ":")
	if len(parts) >= 2 {
		return parts[0], parts[len(parts)-1]
	}
	return address, ""
}

func isLoopback(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

func isPrivateHost(host string) bool {
	host = strings.TrimSpace(host)
	return strings.HasPrefix(host, "10.") || strings.HasPrefix(host, "192.168.") || private172(host)
}

func private172(host string) bool {
	if !strings.HasPrefix(host, "172.") {
		return false
	}
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return false
	}
	var n int
	_, err := fmt.Sscanf(parts[1], "%d", &n)
	return err == nil && n >= 16 && n <= 31
}

func valueOrDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func ToJSON(info model.ProxyInfo, findings []model.Finding, toolVersion, targetPath string) ([]byte, error) {
	return json.MarshalIndent(struct {
		ToolVersion string          `json:"tool_version"`
		TargetPath  string          `json:"target_path"`
		Proxy       model.ProxyInfo `json:"proxy"`
		Findings    []model.Finding `json:"findings"`
	}{ToolVersion: toolVersion, TargetPath: targetPath, Proxy: info, Findings: findings}, "", "  ")
}
