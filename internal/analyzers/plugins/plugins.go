package plugins

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"gitflic.ru/skif4er/doctortools/internal/model"
)

func Analyze(root string) ([]model.PluginInfo, []model.Finding) {
	list, _, findings := AnalyzeDetailed(root)
	return list, findings
}

func AnalyzeDetailed(root string) ([]model.PluginInfo, model.PluginAuditInfo, []model.Finding) {
	pluginsDir := filepath.Join(root, "plugins")
	var findings []model.Finding
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		findings = append(findings, model.Finding{
			ID:             "plugins.dir.not_found",
			Severity:       model.SeverityInfo,
			Category:       "плагины",
			Title:          "Директория plugins не найдена",
			Message:        "CraftDoctor не нашёл директорию plugins. Для Vanilla-сервера это нормально.",
			Recommendation: "Если используется Paper/Purpur/Folia/Spigot, проверить путь к серверу и наличие директории plugins.",
			File:           "plugins",
		})
		return nil, model.PluginAuditInfo{}, findings
	}

	var list []model.PluginInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".jar") {
			continue
		}
		jarPath := filepath.Join(pluginsDir, entry.Name())
		plugin, metaErr := readPluginMetadata(jarPath)
		plugin.JarFile = filepath.Join("plugins", entry.Name())
		if metaErr != nil {
			plugin.Name = strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			plugin.Valid = false
			plugin.Problem = metaErr.Error()
			severity := model.SeverityWarn
			title := "Метаданные плагина не найдены"
			message := "В jar-файле " + entry.Name() + " не найден plugin.yml или paper-plugin.yml."
			recommendation := "Проверить, является ли файл корректным серверным плагином Minecraft."
			id := "plugins.metadata.missing." + safeID(plugin.Name)
			if _, ok := metaErr.(errInvalidJar); ok {
				severity = model.SeverityDanger
				title = "Jar-файл плагина повреждён или недоступен"
				message = "CraftDoctor не смог открыть " + entry.Name() + " как jar/zip-архив: " + metaErr.Error() + "."
				recommendation = "Пересобрать или заново скачать плагин. Повреждённый jar может сорвать запуск сервера."
				id = "plugins.jar.invalid." + safeID(plugin.Name)
			}
			findings = append(findings, model.Finding{
				ID:             id,
				Severity:       severity,
				Category:       "плагины",
				Title:          title,
				Message:        message,
				Recommendation: recommendation,
				File:           plugin.JarFile,
			})
		} else {
			plugin.Valid = true
		}
		if plugin.Valid {
			plugin = enrichPluginFromJar(jarPath, plugin)
		}
		plugin.Categories = detectCategories(plugin)
		list = append(list, plugin)
	}

	sort.SliceStable(list, func(i, j int) bool { return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name) })
	list = fillDuplicateClasses(root, list)
	audit := BuildAudit(list)
	findings = append(findings, analyzeDuplicates(audit)...)
	findings = append(findings, analyzeDependencies(audit)...)
	findings = append(findings, analyzeCycles(audit)...)
	findings = append(findings, analyzeRoleConflicts(audit)...)
	findings = append(findings, analyzeKnownPlugins(list)...)
	findings = append(findings, analyzePluginIntelligence(root, list, audit)...)

	if len(list) == 0 {
		findings = append(findings, model.Finding{
			ID:       "plugins.none",
			Severity: model.SeverityInfo,
			Category: "плагины",
			Title:    "Плагины не найдены",
			Message:  "В директории plugins нет .jar-файлов.",
			File:     "plugins",
		})
	}

	return list, audit, findings
}

func readPluginMetadata(path string) (model.PluginInfo, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return model.PluginInfo{}, errInvalidJar{Err: err}
	}
	defer zr.Close()

	for _, name := range []string{"paper-plugin.yml", "plugin.yml"} {
		for _, f := range zr.File {
			if f.Name != name {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				return model.PluginInfo{}, err
			}
			data, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				return model.PluginInfo{}, err
			}
			plugin := parsePluginYAML(string(data))
			plugin.Metadata = name
			if plugin.Name == "" {
				plugin.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			}
			return plugin, nil
		}
	}
	return model.PluginInfo{}, errNoMetadata{}
}

type errNoMetadata struct{}

func (errNoMetadata) Error() string { return "plugin.yml или paper-plugin.yml не найден" }

type errInvalidJar struct{ Err error }

func (e errInvalidJar) Error() string { return e.Err.Error() }

func parsePluginYAML(text string) model.PluginInfo {
	plugin := model.PluginInfo{}
	scanner := bufio.NewScanner(strings.NewReader(text))
	var activeList string
	for scanner.Scan() {
		raw := scanner.Text()
		line := stripComment(raw)
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "-") && activeList != "" {
			value := cleanValue(strings.TrimSpace(strings.TrimPrefix(trimmed, "-")))
			appendList(&plugin, activeList, value)
			continue
		}
		if strings.HasPrefix(raw, " ") || strings.HasPrefix(raw, "\t") {
			if activeList == "commands-map" || activeList == "permissions-map" || activeList == "paper-server-dependencies" || activeList == "paper-bootstrap-dependencies" {
				if key, ok := parseIndentedMapKey(raw); ok {
					appendList(&plugin, activeList, key)
				}
			}
			continue
		}
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := cleanValue(strings.TrimSpace(parts[1]))
		activeList = ""
		switch key {
		case "name":
			plugin.Name = value
		case "version":
			plugin.Version = value
		case "main":
			plugin.Main = value
		case "api-version":
			plugin.APIVersion = value
		case "folia-supported":
			if parsed, ok := parseBool(value); ok {
				plugin.FoliaSupported = &parsed
			}
		case "bootstrapper":
			plugin.Bootstrapper = value
		case "loader":
			plugin.Loader = value
		case "description":
			plugin.Description = value
		case "website":
			plugin.Website = value
		case "author":
			plugin.Authors = uniqueClean(append(plugin.Authors, value))
		case "authors":
			if value == "" {
				activeList = "authors"
			} else {
				plugin.Authors = parseInlineList(value)
			}
		case "depend":
			if value == "" {
				activeList = "depend"
			} else {
				plugin.Depends = parseInlineList(value)
			}
		case "dependencies":
			if value == "" {
				activeList = "paper-server-dependencies"
			} else {
				plugin.Depends = parseInlineList(value)
			}
		case "softdepend", "soft-dependencies":
			if value == "" {
				activeList = "softdepend"
			} else {
				plugin.SoftDepends = parseInlineList(value)
			}
		case "loadbefore", "load-before":
			if value == "" {
				activeList = "loadbefore"
			} else {
				plugin.LoadBefore = parseInlineList(value)
			}
		case "provides":
			if value == "" {
				activeList = "provides"
			} else {
				plugin.Provides = parseInlineList(value)
			}
		case "libraries":
			if value == "" {
				activeList = "libraries"
			} else {
				plugin.Libraries = parseInlineList(value)
			}
		case "commands":
			activeList = "commands-map"
		case "permissions":
			activeList = "permissions-map"
		}
	}
	plugin.Depends = uniqueClean(plugin.Depends)
	plugin.SoftDepends = uniqueClean(plugin.SoftDepends)
	plugin.LoadBefore = uniqueClean(plugin.LoadBefore)
	plugin.Provides = uniqueClean(plugin.Provides)
	plugin.Libraries = uniqueClean(plugin.Libraries)
	plugin.PaperDependencies = uniqueClean(append(plugin.PaperDependencies, parsePaperDependencies(text)...))
	plugin.Commands = uniqueClean(plugin.Commands)
	plugin.Permissions = uniqueClean(plugin.Permissions)
	plugin.Authors = uniqueClean(plugin.Authors)
	return plugin
}

func parsePaperDependencies(text string) []string {
	var out []string
	inDependencies := false
	inServerOrBootstrap := false
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		raw := scanner.Text()
		line := stripComment(raw)
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " \t"))
		if indent == 0 {
			inDependencies = strings.HasPrefix(trimmed, "dependencies:")
			inServerOrBootstrap = false
			continue
		}
		if !inDependencies {
			continue
		}
		if indent == 2 {
			parts := strings.SplitN(trimmed, ":", 2)
			key := ""
			if len(parts) == 2 {
				key = cleanValue(parts[0])
			}
			inServerOrBootstrap = key == "server" || key == "bootstrap"
			continue
		}
		if inServerOrBootstrap && indent == 4 {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				key := cleanValue(parts[0])
				if key != "" {
					out = append(out, key)
				}
			}
		}
	}
	return uniqueClean(out)
}

func stripComment(line string) string {
	idx := strings.Index(line, "#")
	if idx >= 0 {
		return line[:idx]
	}
	return line
}

func cleanValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "'\"")
	return value
}

func parseInlineList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	value = strings.TrimPrefix(value, "[")
	value = strings.TrimSuffix(value, "]")
	parts := strings.Split(value, ",")
	var out []string
	for _, part := range parts {
		part = cleanValue(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return uniqueClean(out)
}

func appendList(plugin *model.PluginInfo, listName string, value string) {
	if value == "" {
		return
	}
	switch listName {
	case "depend":
		plugin.Depends = append(plugin.Depends, value)
	case "softdepend":
		plugin.SoftDepends = append(plugin.SoftDepends, value)
	case "loadbefore":
		plugin.LoadBefore = append(plugin.LoadBefore, value)
	case "provides":
		plugin.Provides = append(plugin.Provides, value)
	case "authors":
		plugin.Authors = append(plugin.Authors, value)
	case "libraries":
		plugin.Libraries = append(plugin.Libraries, value)
	case "paper-server-dependencies", "paper-bootstrap-dependencies":
		plugin.PaperDependencies = append(plugin.PaperDependencies, value)
	case "commands-map":
		plugin.Commands = append(plugin.Commands, value)
	case "permissions-map":
		plugin.Permissions = append(plugin.Permissions, value)
	}
}

func uniqueClean(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = cleanValue(value)
		if value == "" || seen[strings.ToLower(value)] {
			continue
		}
		seen[strings.ToLower(value)] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func BuildAudit(list []model.PluginInfo) model.PluginAuditInfo {
	audit := model.PluginAuditInfo{
		Total:          len(list),
		DuplicateNames: map[string][]string{},
	}
	installed := installedPlugins(list)
	categories := map[string][]string{}
	byName := map[string][]string{}
	for _, p := range list {
		if p.Valid {
			audit.Valid++
		} else {
			audit.Invalid++
			if strings.Contains(strings.ToLower(p.Problem), "zip") || strings.Contains(strings.ToLower(p.Problem), "archive") || strings.Contains(strings.ToLower(p.Problem), "not a valid") {
				audit.BrokenJars = append(audit.BrokenJars, p.JarFile)
			}
		}
		if p.Name != "" {
			byName[strings.ToLower(p.Name)] = append(byName[strings.ToLower(p.Name)], p.JarFile)
		}
		for _, cat := range p.Categories {
			categories[cat] = append(categories[cat], p.Name)
		}
		for _, dep := range p.Depends {
			present := installed[strings.ToLower(dep)]
			edge := model.PluginDependencyEdge{From: p.Name, To: dep, Kind: "depend", Present: present, File: p.JarFile}
			audit.DependencyEdges = append(audit.DependencyEdges, edge)
			if !present {
				audit.MissingDependencies = append(audit.MissingDependencies, edge)
			}
		}
		for _, dep := range p.SoftDepends {
			audit.DependencyEdges = append(audit.DependencyEdges, model.PluginDependencyEdge{From: p.Name, To: dep, Kind: "softdepend", Present: installed[strings.ToLower(dep)], File: p.JarFile})
		}
		for _, dep := range p.LoadBefore {
			audit.DependencyEdges = append(audit.DependencyEdges, model.PluginDependencyEdge{From: p.Name, To: dep, Kind: "loadbefore", Present: installed[strings.ToLower(dep)], File: p.JarFile})
		}
	}
	for name, files := range byName {
		if len(files) > 1 {
			sort.Strings(files)
			audit.DuplicateNames[name] = files
		}
	}
	if len(audit.DuplicateNames) == 0 {
		audit.DuplicateNames = nil
	}
	for cat, plugins := range categories {
		plugins = uniqueClean(plugins)
		audit.Categories = append(audit.Categories, model.PluginCategoryInfo{Category: cat, Plugins: plugins})
	}
	sort.SliceStable(audit.Categories, func(i, j int) bool { return audit.Categories[i].Category < audit.Categories[j].Category })
	sort.SliceStable(audit.DependencyEdges, func(i, j int) bool {
		return audit.DependencyEdges[i].From+audit.DependencyEdges[i].To < audit.DependencyEdges[j].From+audit.DependencyEdges[j].To
	})
	audit.Cycles = detectCycles(list)
	audit.DuplicateClasses = detectDuplicateClasses(list)
	audit.Conflicts = detectKnownConflicts(audit)
	audit.Graph = buildPluginGraph(list, audit.DependencyEdges)
	return audit
}

func installedPlugins(list []model.PluginInfo) map[string]bool {
	installed := map[string]bool{}
	for _, p := range list {
		if p.Valid && p.Name != "" {
			installed[strings.ToLower(p.Name)] = true
			for _, alias := range p.Provides {
				installed[strings.ToLower(alias)] = true
			}
		}
	}
	return installed
}

func detectCategories(p model.PluginInfo) []string {
	blob := strings.ToLower(strings.Join([]string{p.Name, p.Main, p.Description, strings.Join(p.Provides, " ")}, " "))
	checks := []struct {
		category string
		terms    []string
	}{
		{"права", []string{"luckperms", "permission", "permissions", "groupmanager", "pex"}},
		{"экономика", []string{"economy", "eco", "money", "vault", "shop", "jobs", "playerpoints", "token"}},
		{"чат", []string{"chat", "venturechat", "deluxechat", "chatcontrol"}},
		{"защита территорий", []string{"claim", "lands", "towny", "factions", "residence", "grief", "worldguard"}},
		{"производительность", []string{"spark", "lag", "chunky", "clearlagg", "profiler"}},
		{"миры", []string{"world", "multiverse", "worldedit", "worldborder", "chunky"}},
		{"предметы и RPG", []string{"itemsadder", "oraxen", "mmoitems", "mythic", "rpg", "quest", "skill", "level"}},
		{"протокол", []string{"protocollib", "packet", "protocol"}},
		{"интеграции", []string{"placeholderapi", "discord", "webhook", "dynmap", "bluemap"}},
		{"база данных", []string{"mysql", "database", "mongodb", "redis", "sql"}},
	}
	var out []string
	for _, check := range checks {
		for _, term := range check.terms {
			if strings.Contains(blob, term) {
				out = append(out, check.category)
				break
			}
		}
	}
	return uniqueClean(out)
}

func analyzeDuplicates(audit model.PluginAuditInfo) []model.Finding {
	var findings []model.Finding
	for name, files := range audit.DuplicateNames {
		findings = append(findings, model.Finding{
			ID:             "plugins.duplicate." + safeID(name),
			Severity:       model.SeverityDanger,
			Category:       "плагины",
			Title:          "Дублирующийся плагин",
			Message:        "Плагин " + name + " найден несколько раз: " + strings.Join(files, ", ") + ".",
			Recommendation: "Оставить одну актуальную версию плагина и удалить старые jar-файлы.",
		})
	}
	return findings
}

func analyzeDependencies(audit model.PluginAuditInfo) []model.Finding {
	var findings []model.Finding
	for _, edge := range audit.MissingDependencies {
		findings = append(findings, model.Finding{
			ID:             "plugins.dependency.missing." + safeID(edge.From) + "." + safeID(edge.To),
			Severity:       model.SeverityCritical,
			Category:       "плагины",
			Title:          "Отсутствует обязательная зависимость плагина",
			Message:        "Плагин " + edge.From + " требует зависимость " + edge.To + ", но она не найдена среди установленных плагинов.",
			Recommendation: "Установить зависимость или удалить/заменить плагин, который от неё зависит.",
			File:           edge.File,
		})
	}
	return findings
}

func analyzeCycles(audit model.PluginAuditInfo) []model.Finding {
	var findings []model.Finding
	for _, cycle := range audit.Cycles {
		findings = append(findings, model.Finding{
			ID:             "plugins.dependency.cycle." + safeID(strings.Join(cycle.Plugins, "_")),
			Severity:       model.SeverityDanger,
			Category:       "плагины",
			Title:          "Циклическая зависимость плагинов",
			Message:        "Найдена циклическая обязательная зависимость: " + strings.Join(cycle.Plugins, " -> ") + ".",
			Recommendation: "Проверить plugin.yml/paper-plugin.yml этих плагинов и убрать цикл обязательных depend-связей.",
		})
	}
	return findings
}

func analyzeRoleConflicts(audit model.PluginAuditInfo) []model.Finding {
	roleSeverity := map[string]model.Severity{"права": model.SeverityWarn, "чат": model.SeverityWarn, "защита территорий": model.SeverityWarn}
	var findings []model.Finding
	for _, cat := range audit.Categories {
		sev, ok := roleSeverity[cat.Category]
		if !ok || len(cat.Plugins) < 2 {
			continue
		}
		findings = append(findings, model.Finding{
			ID:             "plugins.category.multiple." + safeID(cat.Category),
			Severity:       sev,
			Category:       "плагины",
			Title:          "Несколько плагинов одной роли",
			Message:        "В категории «" + cat.Category + "» найдено несколько плагинов: " + strings.Join(cat.Plugins, ", ") + ".",
			Recommendation: "Проверить, не дублируют ли плагины одну и ту же функцию и нет ли конфликта команд, прав или обработчиков событий.",
		})
	}
	return findings
}

func analyzeKnownPlugins(list []model.PluginInfo) []model.Finding {
	installed := map[string]bool{}
	for _, p := range list {
		installed[strings.ToLower(p.Name)] = true
	}
	var findings []model.Finding
	if installed["vault"] {
		findings = append(findings, model.Finding{ID: "plugins.known.vault", Severity: model.SeverityInfo, Category: "плагины", Title: "Найден Vault", Message: "Обнаружен Vault — стандартный слой интеграции экономики/прав для многих старых плагинов."})
	}
	if installed["luckperms"] {
		findings = append(findings, model.Finding{ID: "plugins.known.luckperms", Severity: model.SeverityInfo, Category: "плагины", Title: "Найден LuckPerms", Message: "Обнаружен LuckPerms — распространённая система прав."})
	}
	if installed["placeholderapi"] {
		findings = append(findings, model.Finding{ID: "plugins.known.placeholderapi", Severity: model.SeverityInfo, Category: "плагины", Title: "Найден PlaceholderAPI", Message: "Обнаружен PlaceholderAPI — часто используется для интеграций UI, scoreboard и chat-плагинов."})
	}
	if installed["spark"] {
		findings = append(findings, model.Finding{ID: "plugins.known.spark", Severity: model.SeverityInfo, Category: "производительность", Title: "Найден spark", Message: "Обнаружен spark — полезен для профилирования производительности Minecraft-сервера."})
	}
	return findings
}

func detectCycles(list []model.PluginInfo) []model.PluginCycle {
	nameByLower := map[string]string{}
	graph := map[string][]string{}
	installed := installedPlugins(list)
	for _, p := range list {
		if !p.Valid || p.Name == "" {
			continue
		}
		from := strings.ToLower(p.Name)
		nameByLower[from] = p.Name
		for _, dep := range p.Depends {
			to := strings.ToLower(dep)
			if installed[to] {
				graph[from] = append(graph[from], to)
			}
		}
	}
	var cycles []model.PluginCycle
	seenCycles := map[string]bool{}
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var stack []string
	var dfs func(string)
	dfs = func(node string) {
		if visiting[node] {
			idx := -1
			for i, item := range stack {
				if item == node {
					idx = i
					break
				}
			}
			if idx >= 0 {
				raw := append([]string{}, stack[idx:]...)
				display := make([]string, 0, len(raw)+1)
				keyParts := append([]string{}, raw...)
				sort.Strings(keyParts)
				key := strings.Join(keyParts, ":")
				if !seenCycles[key] {
					seenCycles[key] = true
					for _, item := range raw {
						display = append(display, nameByLower[item])
					}
					display = append(display, nameByLower[node])
					cycles = append(cycles, model.PluginCycle{Plugins: display})
				}
			}
			return
		}
		if visited[node] {
			return
		}
		visiting[node] = true
		stack = append(stack, node)
		for _, next := range graph[node] {
			dfs(next)
		}
		stack = stack[:len(stack)-1]
		visiting[node] = false
		visited[node] = true
	}
	keys := make([]string, 0, len(graph))
	for key := range graph {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		dfs(key)
	}
	return cycles
}

func safeID(value string) string {
	value = strings.ToLower(value)
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteRune('_')
			lastUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "unknown"
	}
	return out
}

func FormatAuditText(audit model.PluginAuditInfo, plugins []model.PluginInfo, findings []model.Finding) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Аудит плагинов CraftDoctor\n\n")
	fmt.Fprintf(&b, "Плагинов всего: %d\n", audit.Total)
	fmt.Fprintf(&b, "Корректных: %d\n", audit.Valid)
	fmt.Fprintf(&b, "Проблемных: %d\n", audit.Invalid)
	fmt.Fprintf(&b, "Связей зависимостей: %d\n", len(audit.DependencyEdges))
	fmt.Fprintf(&b, "Отсутствующих обязательных зависимостей: %d\n", len(audit.MissingDependencies))
	fmt.Fprintf(&b, "Циклов обязательных зависимостей: %d\n\n", len(audit.Cycles))

	if len(audit.Categories) > 0 {
		fmt.Fprintf(&b, "Категории:\n")
		for _, cat := range audit.Categories {
			fmt.Fprintf(&b, "- %s: %s\n", cat.Category, strings.Join(cat.Plugins, ", "))
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(plugins) > 0 {
		fmt.Fprintf(&b, "Плагины:\n")
		for _, p := range plugins {
			status := "OK"
			if !p.Valid {
				status = "ПРОБЛЕМА: " + p.Problem
			}
			fmt.Fprintf(&b, "- %s %s · %s · %s\n", p.Name, emptyDash(p.Version), p.JarFile, status)
		}
		fmt.Fprintf(&b, "\n")
	}

	pluginFindings := filterPluginFindings(findings)
	if len(pluginFindings) > 0 {
		fmt.Fprintf(&b, "Проблемы и рекомендации:\n")
		for _, f := range pluginFindings {
			fmt.Fprintf(&b, "- [%s] %s (%s)\n", f.Severity, f.Title, f.ID)
			if f.Recommendation != "" {
				fmt.Fprintf(&b, "  Рекомендация: %s\n", f.Recommendation)
			}
		}
	}
	return b.String()
}

func filterPluginFindings(findings []model.Finding) []model.Finding {
	var out []model.Finding
	for _, f := range findings {
		if f.Category == "плагины" || strings.HasPrefix(f.ID, "plugins.") {
			out = append(out, f)
		}
	}
	return out
}

func emptyDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}

func parseBool(value string) (bool, bool) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return false, false
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, false
	}
	return parsed, true
}

func parseIndentedMapKey(raw string) (string, bool) {
	if strings.TrimSpace(raw) == "" {
		return "", false
	}
	indent := len(raw) - len(strings.TrimLeft(raw, " \t"))
	if indent != 2 {
		return "", false
	}
	trimmed := strings.TrimSpace(stripComment(raw))
	if trimmed == "" || strings.HasPrefix(trimmed, "-") {
		return "", false
	}
	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) != 2 {
		return "", false
	}
	key := cleanValue(strings.TrimSpace(parts[0]))
	if key == "server" || key == "bootstrap" || key == "load" || key == "required" || key == "join-classpath" {
		return "", false
	}
	if strings.Contains(key, " ") || strings.Contains(key, "/") {
		return "", false
	}
	return key, key != ""
}

func enrichPluginFromJar(path string, plugin model.PluginInfo) model.PluginInfo {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return plugin
	}
	defer zr.Close()
	mainPath := strings.ReplaceAll(plugin.Main, ".", "/") + ".class"
	classes := make([]string, 0)
	packageRoots := map[string]bool{}
	shaded := map[string]bool{}
	for _, f := range zr.File {
		name := f.Name
		if !strings.HasSuffix(name, ".class") || strings.HasPrefix(name, "META-INF/") {
			continue
		}
		plugin.ClassCount++
		classes = append(classes, name)
		if name == mainPath {
			plugin.MainClassPresent = true
		}
		parts := strings.Split(name, "/")
		if len(parts) >= 2 {
			packageRoots[parts[0]+"."+parts[1]] = true
		}
		for label, prefixes := range commonShadedPrefixes() {
			for _, prefix := range prefixes {
				if strings.HasPrefix(name, prefix) {
					shaded[label] = true
					break
				}
			}
		}
	}
	for root := range packageRoots {
		plugin.PackageRoots = append(plugin.PackageRoots, root)
	}
	for label := range shaded {
		plugin.ShadedLibraries = append(plugin.ShadedLibraries, label)
	}
	sort.Strings(plugin.PackageRoots)
	sort.Strings(plugin.ShadedLibraries)
	return plugin
}

func commonShadedPrefixes() map[string][]string {
	return map[string][]string{
		"Guava":              {"com/google/common/"},
		"Gson":               {"com/google/gson/"},
		"Apache Commons":     {"org/apache/commons/"},
		"OkHttp":             {"okhttp3/", "okio/"},
		"HikariCP":           {"com/zaxxer/hikari/"},
		"Jedis":              {"redis/clients/jedis/"},
		"Adventure":          {"net/kyori/adventure/"},
		"bStats":             {"org/bstats/"},
		"Configurate":        {"org/spongepowered/configurate/"},
		"Kotlin Runtime":     {"kotlin/", "kotlinx/"},
		"Jackson":            {"com/fasterxml/jackson/"},
		"Cloud Command":      {"cloud/commandframework/"},
		"PlaceholderAPI API": {"me/clip/placeholderapi/"},
	}
}

func detectDuplicateClasses(list []model.PluginInfo) []model.PluginDuplicateClassInfo {
	classOwners := map[string][]string{}
	for _, p := range list {
		if !p.Valid || p.JarFile == "" {
			continue
		}
		path := p.JarFile
		if !filepath.IsAbs(path) {
			path = "" // duplicate class scan is handled from relative jar names in scanDuplicateClassesFromJars.
		}
		_ = path
	}
	return scanDuplicateClassesFromPlugins(list, classOwners)
}

func scanDuplicateClassesFromPlugins(list []model.PluginInfo, classOwners map[string][]string) []model.PluginDuplicateClassInfo {
	// Повторно открываем jar по относительному пути невозможно без root, поэтому фактическое наполнение
	// DuplicateClasses выполняется в analyzePluginIntelligence через сведения duplicate_classes у плагинов.
	seen := map[string][]string{}
	for _, p := range list {
		for _, cls := range p.DuplicateClasses {
			seen[cls] = append(seen[cls], p.Name)
		}
	}
	var out []model.PluginDuplicateClassInfo
	for cls, owners := range seen {
		owners = uniqueClean(owners)
		if len(owners) > 1 {
			out = append(out, model.PluginDuplicateClassInfo{Class: cls, Plugins: owners})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Class < out[j].Class })
	return out
}

func fillDuplicateClasses(root string, list []model.PluginInfo) []model.PluginInfo {
	classes := map[string][]int{}
	for idx, p := range list {
		if !p.Valid || p.JarFile == "" {
			continue
		}
		jarPath := filepath.Join(root, p.JarFile)
		zr, err := zip.OpenReader(jarPath)
		if err != nil {
			continue
		}
		for _, f := range zr.File {
			name := f.Name
			if !strings.HasSuffix(name, ".class") || strings.HasPrefix(name, "META-INF/") || isCommonShadedClass(name) {
				continue
			}
			classes[name] = append(classes[name], idx)
		}
		_ = zr.Close()
	}
	for cls, owners := range classes {
		if len(owners) < 2 {
			continue
		}
		for _, idx := range owners {
			list[idx].DuplicateClasses = append(list[idx].DuplicateClasses, cls)
		}
	}
	for i := range list {
		list[i].DuplicateClasses = uniqueClean(list[i].DuplicateClasses)
		if len(list[i].DuplicateClasses) > 25 {
			list[i].DuplicateClasses = list[i].DuplicateClasses[:25]
		}
	}
	return list
}

func isCommonShadedClass(name string) bool {
	for _, prefixes := range commonShadedPrefixes() {
		for _, prefix := range prefixes {
			if strings.HasPrefix(name, prefix) {
				return true
			}
		}
	}
	return false
}

func detectKnownConflicts(audit model.PluginAuditInfo) []model.PluginConflictInfo {
	var out []model.PluginConflictInfo
	byCategory := map[string][]string{}
	for _, cat := range audit.Categories {
		byCategory[cat.Category] = cat.Plugins
	}
	criticalRoles := map[string]string{
		"права":             "Оставить один основной permission-плагин, чтобы не получить противоречивые права и группы.",
		"экономика":         "Проверить, какой plugin реально является economy provider, и убрать дублирующие provider-слои.",
		"чат":               "Оставить один основной chat-плагин или явно разделить зоны ответственности.",
		"защита территорий": "Проверить пересечение регионов, claims и прав на строительство.",
	}
	for cat, rec := range criticalRoles {
		plugins := uniqueClean(byCategory[cat])
		if len(plugins) > 1 {
			out = append(out, model.PluginConflictInfo{Kind: "role:" + cat, Plugins: plugins, Reason: "несколько плагинов закрывают одну критичную роль: " + cat, Recommendation: rec})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Kind < out[j].Kind })
	return out
}

func buildPluginGraph(list []model.PluginInfo, edges []model.PluginDependencyEdge) model.PluginGraphInfo {
	var nodes []string
	seen := map[string]bool{}
	for _, p := range list {
		if p.Name == "" || seen[p.Name] {
			continue
		}
		seen[p.Name] = true
		nodes = append(nodes, p.Name)
	}
	sort.Strings(nodes)
	return model.PluginGraphInfo{Nodes: nodes, Edges: edges}
}

func analyzePluginIntelligence(root string, list []model.PluginInfo, audit model.PluginAuditInfo) []model.Finding {
	var findings []model.Finding
	folia := hasFoliaCore(root)
	for _, p := range list {
		if !p.Valid {
			continue
		}
		if p.APIVersion == "" {
			findings = append(findings, model.Finding{ID: "plugins.api_version.missing." + safeID(p.Name), Severity: model.SeverityWarn, Category: "плагины", Title: "У плагина не указан api-version", Message: "Плагин " + p.Name + " не содержит api-version в метаданных.", Recommendation: "Для современных Paper/Purpur-серверов желательно использовать плагины с явно указанной api-version.", File: p.JarFile})
		} else if isOldAPIVersion(p.APIVersion) {
			findings = append(findings, model.Finding{ID: "plugins.api_version.old." + safeID(p.Name), Severity: model.SeverityWarn, Category: "плагины", Title: "У плагина устаревшая api-version", Message: "Плагин " + p.Name + " указывает api-version=" + p.APIVersion + ".", Recommendation: "Проверить наличие свежей версии плагина для актуального ядра Minecraft.", File: p.JarFile})
		}
		if p.Main != "" && !p.MainClassPresent && p.ClassCount > 0 {
			findings = append(findings, model.Finding{ID: "plugins.main_class.missing." + safeID(p.Name), Severity: model.SeverityDanger, Category: "плагины", Title: "Main class плагина не найден в jar", Message: "В jar-файле " + p.Name + " не найден класс " + p.Main + ".", Recommendation: "Проверить корректность сборки плагина и plugin.yml/paper-plugin.yml.", File: p.JarFile})
		}
		if folia && (p.FoliaSupported == nil || !*p.FoliaSupported) {
			findings = append(findings, model.Finding{ID: "plugins.folia.compatibility_unknown." + safeID(p.Name), Severity: model.SeverityWarn, Category: "плагины", Title: "Folia-совместимость плагина не подтверждена", Message: "Сервер похож на Folia, но плагин " + p.Name + " не указывает folia-supported=true.", Recommendation: "Перед запуском на Folia проверить документацию плагина и потокобезопасность задач/доступа к миру.", File: p.JarFile})
		}
		if len(p.ShadedLibraries) >= 4 {
			findings = append(findings, model.Finding{ID: "plugins.shaded.many_libraries." + safeID(p.Name), Severity: model.SeverityInfo, Category: "плагины", Title: "Плагин содержит несколько shaded-библиотек", Message: "В плагине " + p.Name + " обнаружены библиотеки: " + strings.Join(p.ShadedLibraries, ", ") + ".", Recommendation: "При ошибках ClassCastException/NoSuchMethodError проверить конфликты shaded-зависимостей.", File: p.JarFile})
		}
		if len(p.Libraries) > 0 {
			findings = append(findings, model.Finding{ID: "plugins.libraries.used." + safeID(p.Name), Severity: model.SeverityInfo, Category: "плагины", Title: "Плагин использует libraries в plugin.yml", Message: "Плагин " + p.Name + " объявляет libraries: " + strings.Join(p.Libraries, ", ") + ".", Recommendation: "Убедиться, что ядро сервера поддерживает загрузку libraries и имеет доступ к нужным артефактам.", File: p.JarFile})
		}
	}
	for _, dup := range audit.DuplicateClasses {
		if len(dup.Plugins) < 2 {
			continue
		}
		findings = append(findings, model.Finding{ID: "plugins.classes.duplicate." + safeID(dup.Class), Severity: model.SeverityWarn, Category: "плагины", Title: "Одинаковый класс найден в нескольких плагинах", Message: "Класс " + dup.Class + " найден в плагинах: " + strings.Join(dup.Plugins, ", ") + ".", Recommendation: "Если это не общая shaded-библиотека, проверить дублирование модулей и старые jar-файлы."})
	}
	for _, conflict := range audit.Conflicts {
		findings = append(findings, model.Finding{ID: "plugins.conflict." + safeID(conflict.Kind), Severity: model.SeverityWarn, Category: "плагины", Title: "Потенциальный конфликт плагинов", Message: conflict.Reason + ": " + strings.Join(conflict.Plugins, ", ") + ".", Recommendation: conflict.Recommendation})
	}
	return findings
}

func hasFoliaCore(root string) bool {
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		if !entry.IsDir() && strings.HasSuffix(name, ".jar") && strings.Contains(name, "folia") {
			return true
		}
	}
	return false
}

func isOldAPIVersion(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	parts := strings.Split(value, ".")
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	minor := 0
	if len(parts) > 1 {
		minor, _ = strconv.Atoi(parts[1])
	}
	return major < 1 || (major == 1 && minor < 19)
}

func FormatGraphText(audit model.PluginAuditInfo, format string) string {
	switch strings.ToLower(format) {
	case "mermaid":
		return FormatGraphMermaid(audit)
	case "dot":
		return FormatGraphDOT(audit)
	default:
		var b strings.Builder
		fmt.Fprintf(&b, "Граф зависимостей плагинов CraftDoctor\n\n")
		fmt.Fprintf(&b, "Узлов: %d\n", len(audit.Graph.Nodes))
		fmt.Fprintf(&b, "Связей: %d\n\n", len(audit.Graph.Edges))
		for _, edge := range audit.Graph.Edges {
			status := "OK"
			if !edge.Present {
				status = "ОТСУТСТВУЕТ"
			}
			fmt.Fprintf(&b, "- %s --%s--> %s [%s]\n", edge.From, edge.Kind, edge.To, status)
		}
		return b.String()
	}
}

func FormatGraphMermaid(audit model.PluginAuditInfo) string {
	var b strings.Builder
	b.WriteString("graph TD\n")
	for _, node := range audit.Graph.Nodes {
		fmt.Fprintf(&b, "  %s[\"%s\"]\n", safeID(node), node)
	}
	for _, edge := range audit.Graph.Edges {
		style := edge.Kind
		if !edge.Present {
			style += " / missing"
		}
		fmt.Fprintf(&b, "  %s -->|%s| %s\n", safeID(edge.From), style, safeID(edge.To))
	}
	return b.String()
}

func FormatGraphDOT(audit model.PluginAuditInfo) string {
	var b strings.Builder
	b.WriteString("digraph plugins {\n")
	for _, node := range audit.Graph.Nodes {
		fmt.Fprintf(&b, "  \"%s\";\n", strings.ReplaceAll(node, "\"", "\\\""))
	}
	for _, edge := range audit.Graph.Edges {
		label := edge.Kind
		if !edge.Present {
			label += " missing"
		}
		fmt.Fprintf(&b, "  \"%s\" -> \"%s\" [label=\"%s\"];\n", strings.ReplaceAll(edge.From, "\"", "\\\""), strings.ReplaceAll(edge.To, "\"", "\\\""), label)
	}
	b.WriteString("}\n")
	return b.String()
}

func FormatExplainText(plugin model.PluginInfo, audit model.PluginAuditInfo, findings []model.Finding) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Plugin Intelligence: %s\n\n", plugin.Name)
	fmt.Fprintf(&b, "Версия: %s\n", emptyDash(plugin.Version))
	fmt.Fprintf(&b, "Jar: %s\n", plugin.JarFile)
	fmt.Fprintf(&b, "Метаданные: %s\n", emptyDash(plugin.Metadata))
	fmt.Fprintf(&b, "Main: %s\n", emptyDash(plugin.Main))
	fmt.Fprintf(&b, "api-version: %s\n", emptyDash(plugin.APIVersion))
	if plugin.FoliaSupported != nil {
		fmt.Fprintf(&b, "Folia-supported: %t\n", *plugin.FoliaSupported)
	}
	fmt.Fprintf(&b, "Классов: %d\n", plugin.ClassCount)
	fmt.Fprintf(&b, "Main class найден: %t\n", plugin.MainClassPresent)
	if len(plugin.PackageRoots) > 0 {
		fmt.Fprintf(&b, "Package roots: %s\n", strings.Join(plugin.PackageRoots, ", "))
	}
	if len(plugin.ShadedLibraries) > 0 {
		fmt.Fprintf(&b, "Shaded-библиотеки: %s\n", strings.Join(plugin.ShadedLibraries, ", "))
	}
	if len(plugin.Commands) > 0 {
		fmt.Fprintf(&b, "Команды: %s\n", strings.Join(plugin.Commands, ", "))
	}
	if len(plugin.Permissions) > 0 {
		fmt.Fprintf(&b, "Права: %s\n", strings.Join(plugin.Permissions, ", "))
	}
	fmt.Fprintf(&b, "\nСвязи зависимостей:\n")
	for _, edge := range audit.DependencyEdges {
		if strings.EqualFold(edge.From, plugin.Name) || strings.EqualFold(edge.To, plugin.Name) {
			status := "OK"
			if !edge.Present {
				status = "ОТСУТСТВУЕТ"
			}
			fmt.Fprintf(&b, "- %s --%s--> %s [%s]\n", edge.From, edge.Kind, edge.To, status)
		}
	}
	related := filterPluginFindingsForPlugin(plugin.Name, findings)
	if len(related) > 0 {
		fmt.Fprintf(&b, "\nПроблемы и рекомендации:\n")
		for _, f := range related {
			fmt.Fprintf(&b, "- [%s] %s (%s)\n", f.Severity, f.Title, f.ID)
			if f.Recommendation != "" {
				fmt.Fprintf(&b, "  Рекомендация: %s\n", f.Recommendation)
			}
		}
	}
	return b.String()
}

func filterPluginFindingsForPlugin(name string, findings []model.Finding) []model.Finding {
	needle := strings.ToLower(name)
	var out []model.Finding
	for _, f := range findings {
		blob := strings.ToLower(strings.Join([]string{f.ID, f.Title, f.Message, f.File}, " "))
		if strings.Contains(blob, needle) {
			out = append(out, f)
		}
	}
	return out
}
