package minecraft

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gitflic.ru/skif4er/doctortools/internal/model"
)

var configNames = []string{
	"server.properties",
	"bukkit.yml",
	"spigot.yml",
	"paper.yml",
	"paper-global.yml",
	"paper-world-defaults.yml",
	"config/paper-global.yml",
	"config/paper-world-defaults.yml",
	"purpur.yml",
	"velocity.toml",
	"bungeecord.yml",
}

func Analyze(root string) (model.MinecraftInfo, []model.Finding) {
	info := model.MinecraftInfo{
		CoreType:   "unknown",
		Properties: map[string]string{},
	}
	var findings []model.Finding

	jar := detectCoreJar(root)
	if jar == "" {
		findings = append(findings, model.Finding{
			ID:             "minecraft.core_jar.not_found",
			Severity:       model.SeverityCritical,
			Category:       "minecraft",
			Title:          "Серверное ядро не найдено",
			Message:        "В корне проекта не найден .jar-файл, похожий на Paper, Purpur, Folia, Spigot, Bukkit или Vanilla server.jar.",
			Recommendation: "Поместить серверное ядро в корень проекта или запускать CraftDoctor с путём к корректной директории Minecraft-сервера.",
		})
	} else {
		info.CoreJar = filepath.Base(jar)
		info.CoreType = detectCoreType(info.CoreJar)
		info.ServerVersion = detectVersionFromName(info.CoreJar)
		findings = append(findings, model.Finding{
			ID:       "minecraft.core.detected",
			Severity: model.SeverityInfo,
			Category: "minecraft",
			Title:    "Серверное ядро обнаружено",
			Message:  "Обнаружено ядро: " + info.CoreJar + " (тип: " + info.CoreType + ").",
			File:     info.CoreJar,
		})
	}

	propsPath := filepath.Join(root, "server.properties")
	if props, err := ReadProperties(propsPath); err == nil {
		info.Properties = props
		findings = append(findings, model.Finding{
			ID:       "minecraft.server_properties.found",
			Severity: model.SeverityInfo,
			Category: "конфигурация",
			Title:    "server.properties найден",
			Message:  "Основной конфигурационный файл Minecraft-сервера найден и прочитан.",
			File:     "server.properties",
		})
	} else {
		findings = append(findings, model.Finding{
			ID:             "minecraft.server_properties.not_found",
			Severity:       model.SeverityWarn,
			Category:       "конфигурация",
			Title:          "server.properties не найден",
			Message:        "CraftDoctor не нашёл server.properties в корне проекта.",
			Recommendation: "Убедиться, что указан путь именно к директории Minecraft-сервера.",
			File:           "server.properties",
		})
	}

	info.ConfigFiles = existingConfigs(root)
	info.Worlds = detectWorlds(root)
	info.PluginsDir = filepath.Join(root, "plugins")
	info.LogsDir = filepath.Join(root, "logs")
	info.CrashReports = countCrashReports(root)
	info.EULAAccepted = eulaAccepted(root)
	info.ProxyForwarded = detectProxyForwarding(root, info.Properties)

	if !info.EULAAccepted {
		findings = append(findings, model.Finding{
			ID:             "minecraft.eula.not_accepted",
			Severity:       model.SeverityWarn,
			Category:       "конфигурация",
			Title:          "EULA не принята или eula.txt отсутствует",
			Message:        "Файл eula.txt отсутствует или не содержит eula=true.",
			Recommendation: "Перед запуском сервера принять EULA, если вы согласны с условиями Mojang/Microsoft.",
			File:           "eula.txt",
		})
	}

	if len(info.Worlds) == 0 {
		findings = append(findings, model.Finding{
			ID:             "minecraft.worlds.not_found",
			Severity:       model.SeverityInfo,
			Category:       "minecraft",
			Title:          "Миры не обнаружены",
			Message:        "CraftDoctor не нашёл директории миров в корне проекта.",
			Recommendation: "Если сервер новый — это нормально. Если сервер боевой — проверить путь сканирования.",
		})
	}

	analyzeProperties(info.Properties, &findings)

	return info, findings
}

func detectCoreJar(root string) string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	var jars []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".jar") {
			continue
		}
		jars = append(jars, filepath.Join(root, e.Name()))
	}
	sort.Strings(jars)
	priority := []string{"purpur", "paper", "folia", "spigot", "bukkit", "server", "minecraft"}
	for _, key := range priority {
		for _, jar := range jars {
			if strings.Contains(strings.ToLower(filepath.Base(jar)), key) {
				return jar
			}
		}
	}
	if len(jars) > 0 {
		return jars[0]
	}
	return ""
}

func detectCoreType(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "purpur"):
		return "Purpur"
	case strings.Contains(lower, "paper"):
		return "Paper"
	case strings.Contains(lower, "folia"):
		return "Folia"
	case strings.Contains(lower, "spigot"):
		return "Spigot"
	case strings.Contains(lower, "bukkit"):
		return "Bukkit"
	case strings.Contains(lower, "server") || strings.Contains(lower, "minecraft"):
		return "Vanilla/Unknown"
	default:
		return "unknown"
	}
}

func detectVersionFromName(name string) string {
	re := regexp.MustCompile(`1\.\d+(?:\.\d+)?`)
	return re.FindString(name)
}

func ReadProperties(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	props := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		props[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return props, scanner.Err()
}

func existingConfigs(root string) []string {
	var found []string
	for _, name := range configNames {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			found = append(found, name)
		}
	}
	return found
}

func detectWorlds(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var worlds []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		levelDat := filepath.Join(root, e.Name(), "level.dat")
		if _, err := os.Stat(levelDat); err == nil {
			worlds = append(worlds, e.Name())
		}
	}
	sort.Strings(worlds)
	return worlds
}

func countCrashReports(root string) int {
	dir := filepath.Join(root, "crash-reports")
	count := 0
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".txt") {
			count++
		}
		return nil
	})
	return count
}

func eulaAccepted(root string) bool {
	props, err := ReadProperties(filepath.Join(root, "eula.txt"))
	if err != nil {
		return false
	}
	return strings.EqualFold(props["eula"], "true")
}

func detectProxyForwarding(root string, props map[string]string) bool {
	if strings.EqualFold(props["online-mode"], "true") {
		return false
	}
	checks := []string{
		"spigot.yml",
		"paper.yml",
		"paper-global.yml",
		"config/paper-global.yml",
		"config/paper-world-defaults.yml",
		"purpur.yml",
	}
	for _, rel := range checks {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			continue
		}
		text := strings.ToLower(string(data))
		if strings.Contains(text, "bungeecord: true") || strings.Contains(text, "proxy-protocol: true") || strings.Contains(text, "velocity") && strings.Contains(text, "enabled: true") {
			return true
		}
	}
	return false
}

func analyzeProperties(props map[string]string, findings *[]model.Finding) {
	if len(props) == 0 {
		return
	}
	if props["view-distance"] != "" && parseInt(props["view-distance"]) > 10 {
		*findings = append(*findings, model.Finding{
			ID:             "performance.view_distance.high",
			Severity:       model.SeverityWarn,
			Category:       "производительность",
			Title:          "Высокий view-distance",
			Message:        "Значение view-distance больше 10 может создавать высокую нагрузку на CPU и сеть.",
			Recommendation: "Для публичного сервера начать с view-distance 6–10 и подбирать значение по MSPT/TPS.",
			File:           "server.properties",
		})
	}
	if props["simulation-distance"] != "" && parseInt(props["simulation-distance"]) > 8 {
		*findings = append(*findings, model.Finding{
			ID:             "performance.simulation_distance.high",
			Severity:       model.SeverityWarn,
			Category:       "производительность",
			Title:          "Высокий simulation-distance",
			Message:        "Значение simulation-distance больше 8 может заметно увеличивать нагрузку на серверный tick loop.",
			Recommendation: "Для production-сервера проверить MSPT и рассмотреть снижение simulation-distance до 4–8.",
			File:           "server.properties",
		})
	}
	if strings.EqualFold(props["enable-command-block"], "true") {
		*findings = append(*findings, model.Finding{
			ID:             "security.command_blocks.enabled",
			Severity:       model.SeverityWarn,
			Category:       "безопасность",
			Title:          "Командные блоки включены",
			Message:        "enable-command-block=true повышает риск ошибок и злоупотреблений при неправильном управлении доступом.",
			Recommendation: "Оставлять включённым только при реальной необходимости и ограниченном доступе к командным блокам.",
			File:           "server.properties",
		})
	}
}

func parseInt(value string) int {
	n := 0
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			break
		}
		n = n*10 + int(ch-'0')
	}
	return n
}
