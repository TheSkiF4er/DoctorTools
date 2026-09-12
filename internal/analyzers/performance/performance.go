package performance

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gitflic.ru/skif4er/doctortools/internal/model"
)

// Analyze выполняет read-only аудит производительности Minecraft-сервера.
func Analyze(root string, system model.SystemInfo, javaInfo model.JavaInfo, mc model.MinecraftInfo, plugins []model.PluginInfo, logInfo model.LogInfo) (model.PerformanceInfo, []model.Finding) {
	return AnalyzeWithOptions(root, system, javaInfo, mc, plugins, logInfo, model.PerformanceOptions{})
}

// AnalyzeWithOptions выполняет аудит производительности с учётом целевого онлайна и типа сервера.
func AnalyzeWithOptions(root string, system model.SystemInfo, javaInfo model.JavaInfo, mc model.MinecraftInfo, plugins []model.PluginInfo, logInfo model.LogInfo, opts model.PerformanceOptions) (model.PerformanceInfo, []model.Finding) {
	info := model.PerformanceInfo{Status: model.SeverityInfo, Options: normalizeOptions(opts)}
	var findings []model.Finding

	info.JVMFlags, info.JVMFlagSources = detectJVMFlags(root)
	info.HeapMinMB, info.HeapMaxMB = parseHeapFlags(info.JVMFlags)
	info.HasSpark = hasPlugin(plugins, "spark")
	info.SparkReports = detectSparkReports(root)
	info.Profiles = detectPerformanceProfiles(root)

	addServerPropertiesMetrics(&info, mc.Properties)
	analyzeCoreConfig(root, &info)
	analyzeJVM(&info, system, javaInfo)
	analyzePlugins(&info, plugins)
	analyzeLogs(&info, logInfo)
	analyzeImportedProfiles(&info)
	analyzeCapacity(&info, system, mc.Properties)
	buildFindings(&info, &findings)
	finalize(&info)
	return info, findings
}

func normalizeOptions(opts model.PerformanceOptions) model.PerformanceOptions {
	target := strings.ToLower(strings.TrimSpace(opts.Target))
	target = strings.ReplaceAll(target, "_", "-")
	switch target {
	case "", "online", "survival", "rpg", "minigames":
		// допустимые значения
	default:
		target = "custom"
	}
	if opts.Players < 0 {
		opts.Players = 0
	}
	opts.Target = target
	return opts
}

func addServerPropertiesMetrics(info *model.PerformanceInfo, props map[string]string) {
	if len(props) == 0 {
		return
	}
	addIntMetric(info, "view-distance", props["view-distance"], "server.properties", 10, 6, "Для публичного сервера обычно стоит начинать с view-distance 6–10 и проверять MSPT/TPS.")
	addIntMetric(info, "simulation-distance", props["simulation-distance"], "server.properties", 8, 4, "Для survival/RPG-сервера обычно стоит начинать с simulation-distance 4–8.")
	addIntMetric(info, "entity-broadcast-range-percentage", props["entity-broadcast-range-percentage"], "server.properties", 100, 60, "При высокой нагрузке можно аккуратно снижать entity-broadcast-range-percentage, проверяя игровой опыт.")
	if v := strings.TrimSpace(props["max-tick-time"]); v != "" {
		status := model.SeverityInfo
		rec := "Значение max-tick-time выглядит штатно."
		if parseInt(v) <= 0 {
			status = model.SeverityWarn
			rec = "max-tick-time отключает watchdog. Это может скрывать зависания сервера; отключать стоит только осознанно."
		}
		info.Metrics = append(info.Metrics, model.PerformanceMetric{Key: "max-tick-time", Value: v, Source: "server.properties", Status: status, Recommendation: rec})
	}
	if v := strings.TrimSpace(props["network-compression-threshold"]); v != "" {
		status := model.SeverityInfo
		rec := "Порог сетевого сжатия задан. Для публичного сервера стоит проверять CPU/network trade-off под реальной нагрузкой."
		if parseInt(v) < 0 {
			status = model.SeverityWarn
			rec = "Сжатие пакетов отключено. Это может увеличить сетевой трафик на публичном сервере."
		}
		info.Metrics = append(info.Metrics, model.PerformanceMetric{Key: "network-compression-threshold", Value: v, Source: "server.properties", Status: status, Recommendation: rec})
	}
}

func analyzeCoreConfig(root string, info *model.PerformanceInfo) {
	checks := []struct {
		path string
		keys []string
	}{
		{"bukkit.yml", []string{"spawn-limits", "ticks-per", "chunk-gc"}},
		{"spigot.yml", []string{"entity-activation-range", "entity-tracking-range", "merge-radius", "mob-spawn-range", "view-distance"}},
		{"paper.yml", []string{"max-auto-save-chunks-per-tick", "delay-chunk-unloads-by", "keep-spawn-loaded"}},
		{"paper-global.yml", []string{"chunk-system", "async-chunks", "unsupported-settings"}},
		{"paper-world-defaults.yml", []string{"entities", "chunks", "max-auto-save-chunks-per-tick", "keep-spawn-loaded"}},
		{"config/paper-global.yml", []string{"chunk-system", "async-chunks", "unsupported-settings"}},
		{"config/paper-world-defaults.yml", []string{"entities", "chunks", "max-auto-save-chunks-per-tick", "keep-spawn-loaded"}},
		{"purpur.yml", []string{"settings", "blocks", "mobs", "ridables"}},
	}
	for _, check := range checks {
		data, err := os.ReadFile(filepath.Join(root, check.path))
		if err != nil {
			continue
		}
		text := string(data)
		for _, key := range check.keys {
			if strings.Contains(strings.ToLower(text), strings.ToLower(key)) {
				info.Metrics = append(info.Metrics, model.PerformanceMetric{Key: key, Value: "найдено", Source: check.path, Status: model.SeverityInfo, Recommendation: "Параметр найден в конфиге ядра. При лагах проверьте его значение в связке с профилем нагрузки."})
			}
		}
		if strings.Contains(strings.ToLower(text), "keep-spawn-loaded: true") || strings.Contains(strings.ToLower(text), "keep-spawn-loaded-range:") {
			info.Risks = append(info.Risks, model.PerformanceRisk{ID: "performance.spawn.keep_loaded", Severity: model.SeverityWarn, Area: "чанки", Message: "В конфигурациях Paper/Purpur найдены признаки постоянно загруженного spawn.", Recommendation: "Если spawn не должен быть всегда активен, отключить keep-spawn-loaded или уменьшить радиус."})
		}
	}
}

func analyzeJVM(info *model.PerformanceInfo, system model.SystemInfo, javaInfo model.JavaInfo) {
	if javaInfo.Found && javaInfo.Major > 0 && javaInfo.Major < 17 {
		info.Risks = append(info.Risks, model.PerformanceRisk{ID: "performance.java.too_old", Severity: model.SeverityDanger, Area: "java", Message: "Java ниже 17 может быть несовместима с современными ядрами и ухудшать эксплуатацию.", Recommendation: "Использовать Java 21 для актуальных веток Paper/Purpur/Folia."})
	}
	if len(info.JVMFlags) == 0 {
		info.Risks = append(info.Risks, model.PerformanceRisk{ID: "performance.jvm.flags.not_found", Severity: model.SeverityWarn, Area: "jvm", Message: "CraftDoctor не нашёл файлов с JVM-флагами рядом с сервером.", Recommendation: "Хранить параметры запуска в явном файле: start.sh, user_jvm_args.txt или *.args, чтобы их можно было ревизовать."})
	}
	if info.HeapMaxMB == 0 {
		info.Risks = append(info.Risks, model.PerformanceRisk{ID: "performance.jvm.xmx.not_found", Severity: model.SeverityWarn, Area: "jvm", Message: "Не найден параметр -Xmx в обнаруженных JVM-флагах.", Recommendation: "Явно задать -Xmx под размер сервера и оставить запас RAM для ОС, дискового кэша и БД."})
	} else if system.TotalMemoryMB > 0 {
		pct := float64(info.HeapMaxMB) / float64(system.TotalMemoryMB) * 100
		if pct > 85 {
			info.Risks = append(info.Risks, model.PerformanceRisk{ID: "performance.jvm.xmx.too_high", Severity: model.SeverityWarn, Area: "jvm", Message: fmt.Sprintf("-Xmx занимает около %.0f%% RAM машины.", pct), Recommendation: "Оставить резерв памяти для ОС, файлового кэша, прокси, БД и вспомогательных сервисов."})
		}
		if info.HeapMaxMB < 2048 && system.TotalMemoryMB >= 4096 {
			info.Risks = append(info.Risks, model.PerformanceRisk{ID: "performance.jvm.xmx.low", Severity: model.SeverityInfo, Area: "jvm", Message: "-Xmx меньше 2 ГБ при наличии большего объёма RAM на машине.", Recommendation: "Для публичного Paper/Purpur-сервера проверить, достаточно ли heap под плагины, миры и онлайн."})
		}
	}
	if info.HeapMinMB > 0 && info.HeapMaxMB > 0 && info.HeapMinMB != info.HeapMaxMB {
		info.Risks = append(info.Risks, model.PerformanceRisk{ID: "performance.jvm.xms_xmx.mismatch", Severity: model.SeverityInfo, Area: "jvm", Message: "-Xms и -Xmx различаются.", Recommendation: "Для стабильного heap-профиля часто используют одинаковые -Xms и -Xmx, если хватает RAM."})
	}
	if len(info.JVMFlags) > 0 && !containsFlagPrefix(info.JVMFlags, "-XX:+UseG1GC") && !containsFlagPrefix(info.JVMFlags, "-XX:+UseZGC") && !containsFlagPrefix(info.JVMFlags, "-XX:+UseShenandoahGC") {
		info.Risks = append(info.Risks, model.PerformanceRisk{ID: "performance.jvm.gc.not_explicit", Severity: model.SeverityInfo, Area: "jvm", Message: "В JVM-флагах не найден явный выбор GC.", Recommendation: "Для большинства Minecraft-серверов обычно используют G1GC; ZGC стоит тестировать отдельно на подходящих версиях Java и нагрузке."})
	}
}

func analyzePlugins(info *model.PerformanceInfo, plugins []model.PluginInfo) {
	if info.HasSpark {
		info.Recommendations = append(info.Recommendations, "spark найден: при жалобах на лаги используйте профилирование, а не только чтение latest.log.")
	} else {
		info.Risks = append(info.Risks, model.PerformanceRisk{ID: "performance.profiling.spark_not_found", Severity: model.SeverityInfo, Area: "профилирование", Message: "Плагин spark не найден среди установленных плагинов.", Recommendation: "Для диагностики лагов полезно установить spark и прикладывать profiler/tick health отчёты к разбору."})
	}
	perfPlugins := 0
	for _, p := range plugins {
		for _, c := range p.Categories {
			if c == "производительность" {
				perfPlugins++
			}
		}
	}
	if perfPlugins > 1 {
		info.Risks = append(info.Risks, model.PerformanceRisk{ID: "performance.plugins.multiple_optimizers", Severity: model.SeverityWarn, Area: "плагины", Message: "Найдено несколько плагинов категории производительности.", Recommendation: "Проверить, не конфликтуют ли оптимизаторы между собой и не скрывают ли они реальную причину лагов."})
	}
}

func analyzeLogs(info *model.PerformanceInfo, logInfo model.LogInfo) {
	if logInfo.LongTickCount > 0 {
		info.Risks = append(info.Risks, model.PerformanceRisk{ID: "performance.logs.long_tick", Severity: model.SeverityWarn, Area: "логи", Message: fmt.Sprintf("В latest.log найдено long tick предупреждений: %d.", logInfo.LongTickCount), Recommendation: "Собрать spark-профиль во время просадки TPS и проверить чанки, сущности, БД и тяжёлые плагины."})
	}
	if logInfo.ErrorCount > 0 && logInfo.LongTickCount > 0 {
		info.Risks = append(info.Risks, model.PerformanceRisk{ID: "performance.logs.errors_with_lag", Severity: model.SeverityWarn, Area: "логи", Message: "Ошибки в логах совпадают с признаками лагов.", Recommendation: "Сначала устранить ERROR/Exception, затем повторно измерить MSPT/TPS."})
	}
}

func analyzeImportedProfiles(info *model.PerformanceInfo) {
	if len(info.Profiles) == 0 {
		if len(info.SparkReports) > 0 {
			info.Recommendations = append(info.Recommendations, "Найдены spark-файлы, но CraftDoctor не смог извлечь из них TPS/MSPT. Прикладывайте JSON/текстовые отчёты spark или timings рядом с сервером.")
		}
		return
	}
	for _, profile := range info.Profiles {
		if profile.TPS > 0 {
			status := model.SeverityInfo
			rec := "TPS из импортированного отчёта выглядит штатно."
			if profile.TPS < 18 {
				status = model.SeverityWarn
				rec = "TPS ниже 18 указывает на заметную деградацию. Сопоставьте отчёт с временем жалоб игроков и heavy-плагинами."
			}
			if profile.TPS < 15 {
				status = model.SeverityDanger
				rec = "TPS ниже 15 — серьёзная просадка. Нужен profiler snapshot, разбор чанков/сущностей/БД и тяжёлых задач."
			}
			info.Metrics = append(info.Metrics, model.PerformanceMetric{Key: "profile.tps", Value: fmtFloat(profile.TPS), Source: profile.Source, Status: status, Recommendation: rec})
		}
		avg := firstPositive(profile.MSPTAvg, profile.MSPTP95, profile.MSPTMax)
		if avg > 0 {
			status := model.SeverityInfo
			rec := "MSPT из импортированного отчёта находится в допустимой зоне."
			if avg > 50 {
				status = model.SeverityWarn
				rec = "MSPT выше 50 означает, что tick loop не успевает держать стабильные 20 TPS. Проверьте top-плагины, чанки, сущности и IO."
			}
			if avg > 80 {
				status = model.SeverityDanger
				rec = "MSPT выше 80 указывает на тяжёлую перегрузку. Сначала найдите главный hotspot в spark/timings, затем меняйте конфиги."
			}
			info.Metrics = append(info.Metrics, model.PerformanceMetric{Key: "profile.mspt", Value: fmtFloat(avg), Source: profile.Source, Status: status, Recommendation: rec})
		}
	}
}

func analyzeCapacity(info *model.PerformanceInfo, system model.SystemInfo, props map[string]string) {
	opts := info.Options
	if opts.Players == 0 && opts.Target == "" {
		return
	}
	cap := buildCapacity(opts, len(info.JVMFlags) > 0)
	info.Capacity = cap
	if opts.Players > 0 {
		info.Metrics = append(info.Metrics, model.PerformanceMetric{Key: "capacity.players", Value: strconv.Itoa(opts.Players), Source: "--players", Status: model.SeverityInfo, Recommendation: "Целевой онлайн используется только как ориентир для preflight-рекомендаций; реальные значения проверяйте профилированием."})
	}
	if opts.Target != "" {
		info.Metrics = append(info.Metrics, model.PerformanceMetric{Key: "capacity.target", Value: opts.Target, Source: "--target", Status: model.SeverityInfo, Recommendation: "Тип сервера влияет на стартовые рекомендации по дистанциям и heap."})
	}
	if cap.RecommendedHeapMB > 0 {
		info.Recommendations = append(info.Recommendations, fmt.Sprintf("Ориентир heap под %s/%d игроков: около %d МБ при обязательной проверке MSPT/TPS.", targetLabel(cap.Target), cap.Players, cap.RecommendedHeapMB))
		if info.HeapMaxMB > 0 && info.HeapMaxMB < cap.RecommendedHeapMB*75/100 {
			info.Risks = append(info.Risks, model.PerformanceRisk{ID: "performance.capacity.heap_below_target", Severity: model.SeverityWarn, Area: "capacity", Message: fmt.Sprintf("-Xmx=%d МБ заметно ниже ориентира %d МБ для указанного онлайна.", info.HeapMaxMB, cap.RecommendedHeapMB), Recommendation: "Проверьте фактическое потребление памяти, GC-паузы и запас RAM. Не увеличивайте heap без профилирования и резерва для ОС."})
		}
		if system.TotalMemoryMB > 0 && uint64(cap.RecommendedHeapMB) > system.TotalMemoryMB*85/100 {
			info.Risks = append(info.Risks, model.PerformanceRisk{ID: "performance.capacity.ram_insufficient", Severity: model.SeverityWarn, Area: "capacity", Message: "Ориентир heap под указанный онлайн занимает слишком большую долю RAM машины.", Recommendation: "Снизить целевой онлайн/дистанции, добавить RAM или вынести proxy/БД/карты на отдельные сервисы."})
		}
	}
	compareDistanceWithCapacity(info, props, "view-distance", cap.RecommendedViewDistance, "performance.capacity.view_distance_above_target")
	compareDistanceWithCapacity(info, props, "simulation-distance", cap.RecommendedSimulationDistance, "performance.capacity.simulation_distance_above_target")
	for _, note := range cap.Notes {
		info.Recommendations = append(info.Recommendations, note)
	}
}

func buildCapacity(opts model.PerformanceOptions, hasJVMFlags bool) model.PerformanceCapacity {
	target := opts.Target
	if target == "" {
		target = "online"
	}
	players := opts.Players
	cap := model.PerformanceCapacity{Players: players, Target: target}
	switch target {
	case "rpg":
		cap.RecommendedViewDistance = 6
		cap.RecommendedSimulationDistance = 4
		cap.Notes = append(cap.Notes, "Для RPG/MMO-сервера обычно важнее стабильный MSPT и scripted-контент, чем высокая дистанция прорисовки.")
	case "minigames":
		cap.RecommendedViewDistance = 6
		cap.RecommendedSimulationDistance = 4
		cap.Notes = append(cap.Notes, "Для мини-игр обычно разумно держать компактные арены, низкие дистанции и короткие сессии.")
	case "survival":
		cap.RecommendedViewDistance = 8
		cap.RecommendedSimulationDistance = 6
		cap.Notes = append(cap.Notes, "Для survival-сервера дистанции выше 8/6 стоит подтверждать профилированием под реальным онлайном.")
	default:
		cap.RecommendedViewDistance = 8
		cap.RecommendedSimulationDistance = 5
	}
	if players > 0 {
		cap.RecommendedHeapMB = recommendedHeap(players, target)
		if players >= 80 {
			cap.Notes = append(cap.Notes, "Для онлайна 80+ желательно отдельно проверять БД, карты, очередь сохранения мира и тяжёлые scheduled tasks.")
		}
		if players >= 150 {
			cap.Notes = append(cap.Notes, "Для онлайна 150+ рассматривайте разделение сети на backend-серверы и отдельный proxy, а не один монолитный мир.")
		}
	}
	if !hasJVMFlags {
		cap.Notes = append(cap.Notes, "Ориентиры capacity менее точны без явных JVM-флагов запуска.")
	}
	return cap
}

func recommendedHeap(players int, target string) int {
	base := 3072
	perPlayer := 48
	switch target {
	case "rpg":
		base = 4096
		perPlayer = 64
	case "minigames":
		base = 3072
		perPlayer = 32
	case "survival":
		base = 4096
		perPlayer = 56
	}
	value := base + players*perPlayer
	if value < 4096 && players >= 20 {
		value = 4096
	}
	if value > 24576 {
		value = 24576
	}
	return roundUp(value, 512)
}

func compareDistanceWithCapacity(info *model.PerformanceInfo, props map[string]string, key string, recommended int, id string) {
	if recommended <= 0 || len(props) == 0 {
		return
	}
	current := parseInt(props[key])
	if current <= 0 || current <= recommended {
		return
	}
	info.Risks = append(info.Risks, model.PerformanceRisk{ID: id, Severity: model.SeverityWarn, Area: "capacity", Message: fmt.Sprintf("%s=%d выше ориентира %d для указанного профиля нагрузки.", key, current, recommended), Recommendation: "Снизить параметр или подтвердить его безопасное значение через spark/timings под реальным онлайном."})
}

func buildFindings(info *model.PerformanceInfo, findings *[]model.Finding) {
	for _, risk := range info.Risks {
		*findings = append(*findings, model.Finding{ID: risk.ID, Severity: risk.Severity, Category: "производительность", Title: titleForRisk(risk), Message: risk.Message, Recommendation: risk.Recommendation})
	}
	for _, metric := range info.Metrics {
		if metric.Status.Rank() < model.SeverityWarn.Rank() {
			continue
		}
		id := "performance.metric." + normalizeID(metric.Key)
		*findings = append(*findings, model.Finding{ID: id, Severity: metric.Status, Category: "производительность", Title: "Рискованный параметр производительности: " + metric.Key, Message: "Значение " + metric.Key + " = " + metric.Value + ".", Recommendation: metric.Recommendation, File: metric.Source})
	}
}

func finalize(info *model.PerformanceInfo) {
	status := model.SeverityInfo
	for _, risk := range info.Risks {
		if risk.Severity.Rank() > status.Rank() {
			status = risk.Severity
		}
	}
	for _, metric := range info.Metrics {
		if metric.Status.Rank() > status.Rank() {
			status = metric.Status
		}
	}
	if len(info.Recommendations) == 0 {
		info.Recommendations = append(info.Recommendations, "Используйте Performance Doctor как preflight-аудит, а точные причины лагов подтверждайте profiler-отчётами spark/timings.")
	}
	info.Status = status
	sort.SliceStable(info.Metrics, func(i, j int) bool { return info.Metrics[i].Key < info.Metrics[j].Key })
	sort.SliceStable(info.Risks, func(i, j int) bool {
		if info.Risks[i].Severity.Rank() == info.Risks[j].Severity.Rank() {
			return info.Risks[i].ID < info.Risks[j].ID
		}
		return info.Risks[i].Severity.Rank() > info.Risks[j].Severity.Rank()
	})
	sort.SliceStable(info.Profiles, func(i, j int) bool { return info.Profiles[i].Source < info.Profiles[j].Source })
}

func addIntMetric(info *model.PerformanceInfo, key, value, source string, warnAbove, recommended int, recommendation string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	n := parseInt(value)
	status := model.SeverityInfo
	rec := recommendation
	if warnAbove > 0 && n > warnAbove {
		status = model.SeverityWarn
		rec = fmt.Sprintf("Текущее значение выше рекомендуемого стартового ориентира (%d). %s", recommended, recommendation)
	}
	info.Metrics = append(info.Metrics, model.PerformanceMetric{Key: key, Value: value, Source: source, Status: status, Recommendation: rec})
}

func detectJVMFlags(root string) ([]string, []string) {
	candidates := []string{"user_jvm_args.txt", "jvm.args", "server.args", "start.sh", "run.sh", "start.bat", "run.bat"}
	var flags []string
	var sources []string
	seen := map[string]bool{}
	for _, rel := range candidates {
		path := filepath.Join(root, rel)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		found := extractJVMFlags(string(data))
		if len(found) == 0 {
			continue
		}
		sources = append(sources, rel)
		for _, flag := range found {
			if !seen[flag] {
				flags = append(flags, flag)
				seen[flag] = true
			}
		}
	}
	sort.Strings(flags)
	return flags, sources
}

func extractJVMFlags(text string) []string {
	scanner := bufio.NewScanner(strings.NewReader(text))
	var flags []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		for _, token := range strings.Fields(line) {
			token = strings.Trim(token, "'\"")
			if strings.HasPrefix(token, "-Xmx") || strings.HasPrefix(token, "-Xms") || strings.HasPrefix(token, "-XX:") || strings.HasPrefix(token, "-D") {
				flags = append(flags, token)
			}
		}
	}
	return flags
}

func parseHeapFlags(flags []string) (int, int) {
	minMB, maxMB := 0, 0
	for _, flag := range flags {
		if strings.HasPrefix(flag, "-Xms") {
			minMB = parseMemoryMB(strings.TrimPrefix(flag, "-Xms"))
		}
		if strings.HasPrefix(flag, "-Xmx") {
			maxMB = parseMemoryMB(strings.TrimPrefix(flag, "-Xmx"))
		}
	}
	return minMB, maxMB
}

func parseMemoryMB(value string) int {
	value = strings.TrimSpace(strings.ToLower(value))
	re := regexp.MustCompile(`^(\d+)([kmg]?)`)
	match := re.FindStringSubmatch(value)
	if len(match) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(match[1])
	unit := "m"
	if len(match) >= 3 && match[2] != "" {
		unit = match[2]
	}
	switch unit {
	case "g":
		return n * 1024
	case "k":
		if n < 1024 {
			return 1
		}
		return n / 1024
	default:
		return n
	}
}

func detectSparkReports(root string) []string {
	var reports []string
	for _, rel := range performanceReportSearchDirs() {
		dir := filepath.Join(root, rel)
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d == nil || d.IsDir() {
				return nil
			}
			lower := strings.ToLower(d.Name())
			if strings.Contains(lower, "spark") || strings.Contains(lower, "timings") || strings.Contains(lower, "profile") || strings.HasSuffix(lower, ".json") || strings.HasSuffix(lower, ".html") {
				if r, err := filepath.Rel(root, path); err == nil {
					reports = append(reports, r)
				}
			}
			return nil
		})
	}
	sort.Strings(reports)
	return uniqueStrings(reports)
}

func performanceReportSearchDirs() []string {
	return []string{"spark", "spark-reports", "timings", "reports", "profiles", "plugins/spark", "plugins/spark/profiler", "plugins/spark/reports"}
}

func detectPerformanceProfiles(root string) []model.PerformanceProfile {
	var profiles []model.PerformanceProfile
	seen := map[string]bool{}
	for _, rel := range performanceReportSearchDirs() {
		dir := filepath.Join(root, rel)
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d == nil || d.IsDir() || len(profiles) >= 30 {
				return nil
			}
			lower := strings.ToLower(d.Name())
			if !looksLikePerformanceReport(lower) {
				return nil
			}
			st, err := d.Info()
			if err != nil || st.Size() > 5*1024*1024 {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			relPath, err := filepath.Rel(root, path)
			if err != nil {
				relPath = path
			}
			if seen[relPath] {
				return nil
			}
			seen[relPath] = true
			profile := parsePerformanceReport(relPath, string(data))
			if profile.TPS > 0 || profile.MSPTAvg > 0 || profile.MSPTP95 > 0 || profile.MSPTMax > 0 || len(profile.Notes) > 0 {
				profiles = append(profiles, profile)
			}
			return nil
		})
	}
	return profiles
}

func looksLikePerformanceReport(name string) bool {
	if !(strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".html") || strings.HasSuffix(name, ".txt") || strings.HasSuffix(name, ".log")) {
		return false
	}
	for _, token := range []string{"spark", "timings", "profile", "profiler", "tick", "performance"} {
		if strings.Contains(name, token) {
			return true
		}
	}
	return false
}

func parsePerformanceReport(source, text string) model.PerformanceProfile {
	kind := "unknown"
	lowerSource := strings.ToLower(source)
	lowerText := strings.ToLower(text)
	if strings.Contains(lowerSource, "spark") || strings.Contains(lowerText, "spark") {
		kind = "spark"
	}
	if strings.Contains(lowerSource, "timings") || strings.Contains(lowerText, "timings") {
		kind = "timings"
	}
	profile := model.PerformanceProfile{Source: source, Kind: kind}
	profile.TPS = clampTPS(maxRegexFloat(text, []string{`(?i)\btps\b[^0-9]{0,16}([0-9]+(?:\.[0-9]+)?)`, `(?i)ticks per second[^0-9]{0,16}([0-9]+(?:\.[0-9]+)?)`}))
	profile.MSPTAvg = maxRegexFloat(text, []string{`(?i)avg(?:erage)?\s*mspt[^0-9]{0,16}([0-9]+(?:\.[0-9]+)?)`, `(?i)mean\s*mspt[^0-9]{0,16}([0-9]+(?:\.[0-9]+)?)`, `(?i)mspt[^0-9]{0,16}avg[^0-9]{0,16}([0-9]+(?:\.[0-9]+)?)`})
	profile.MSPTP95 = maxRegexFloat(text, []string{`(?i)p95\s*mspt[^0-9]{0,16}([0-9]+(?:\.[0-9]+)?)`, `(?i)95(?:th)?[^0-9]{0,16}mspt[^0-9]{0,16}([0-9]+(?:\.[0-9]+)?)`})
	profile.MSPTMax = maxRegexFloat(text, []string{`(?i)max(?:imum)?\s*mspt[^0-9]{0,16}([0-9]+(?:\.[0-9]+)?)`, `(?i)mspt[^0-9]{0,16}max[^0-9]{0,16}([0-9]+(?:\.[0-9]+)?)`})
	profile.Samples = int(maxRegexFloat(text, []string{`(?i)samples?[^0-9]{0,16}([0-9]+)`, `(?i)ticks?[^0-9]{0,16}([0-9]+)`}))
	if strings.Contains(lowerText, "entity") || strings.Contains(lowerText, "entities") || strings.Contains(lowerText, "сущ") {
		profile.Notes = append(profile.Notes, "в отчёте есть признаки анализа сущностей")
	}
	if strings.Contains(lowerText, "chunk") || strings.Contains(lowerText, "chunks") || strings.Contains(lowerText, "чанк") {
		profile.Notes = append(profile.Notes, "в отчёте есть признаки анализа чанков")
	}
	if strings.Contains(lowerText, "scheduler") || strings.Contains(lowerText, "scheduled") || strings.Contains(lowerText, "task") {
		profile.Notes = append(profile.Notes, "в отчёте есть признаки анализа scheduled tasks")
	}
	return profile
}

func maxRegexFloat(text string, patterns []string) float64 {
	max := 0.0
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			v, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64)
			if err == nil && v > max {
				max = v
			}
		}
	}
	return max
}

func hasPlugin(plugins []model.PluginInfo, name string) bool {
	for _, p := range plugins {
		if strings.EqualFold(p.Name, name) || strings.Contains(strings.ToLower(p.JarFile), strings.ToLower(name)) {
			return true
		}
	}
	return false
}

func containsFlagPrefix(flags []string, prefix string) bool {
	for _, flag := range flags {
		if strings.HasPrefix(flag, prefix) {
			return true
		}
	}
	return false
}

func parseInt(value string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(value))
	return n
}

func normalizeID(value string) string {
	value = strings.ToLower(value)
	var b strings.Builder
	lastDot := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastDot = false
		} else if !lastDot {
			b.WriteByte('.')
			lastDot = true
		}
	}
	return strings.Trim(b.String(), ".")
}

func titleForRisk(risk model.PerformanceRisk) string {
	switch risk.ID {
	case "performance.jvm.flags.not_found":
		return "JVM-флаги запуска не найдены"
	case "performance.jvm.xmx.not_found":
		return "Не найден параметр -Xmx"
	case "performance.jvm.xmx.too_high":
		return "-Xmx занимает слишком большую долю RAM"
	case "performance.jvm.xms_xmx.mismatch":
		return "-Xms и -Xmx различаются"
	case "performance.logs.long_tick":
		return "В логах найдены long tick предупреждения"
	case "performance.profiling.spark_not_found":
		return "spark не найден"
	case "performance.spawn.keep_loaded":
		return "Spawn может быть постоянно загружен"
	case "performance.profile.mspt_high":
		return "Импортированный профиль показывает высокий MSPT"
	case "performance.profile.tps_low":
		return "Импортированный профиль показывает низкий TPS"
	case "performance.capacity.heap_below_target":
		return "Heap ниже ориентира под целевой онлайн"
	case "performance.capacity.view_distance_above_target":
		return "view-distance выше ориентира под целевой онлайн"
	case "performance.capacity.simulation_distance_above_target":
		return "simulation-distance выше ориентира под целевой онлайн"
	default:
		if risk.Area != "" {
			return "Риск производительности: " + risk.Area
		}
		return "Риск производительности"
	}
}

// FormatAnalysisText возвращает текстовый отчёт команды performance scan.
func FormatAnalysisText(info model.PerformanceInfo, findings []model.Finding) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Performance Doctor 2.0\n\n")
	fmt.Fprintf(&b, "Статус: %s\n", info.Status)
	if info.Options.Players > 0 || info.Options.Target != "" {
		fmt.Fprintf(&b, "Профиль: target=%s, players=%d\n", emptyDash(targetLabel(info.Options.Target)), info.Options.Players)
	}
	if info.HeapMinMB > 0 || info.HeapMaxMB > 0 {
		fmt.Fprintf(&b, "Heap: Xms=%d МБ, Xmx=%d МБ\n", info.HeapMinMB, info.HeapMaxMB)
	}
	if len(info.JVMFlagSources) > 0 {
		fmt.Fprintf(&b, "Источники JVM-флагов: %s\n", strings.Join(info.JVMFlagSources, ", "))
	}
	fmt.Fprintf(&b, "spark найден: %t\n", info.HasSpark)
	fmt.Fprintf(&b, "Импортированных profiler/timings-отчётов: %d\n", len(info.Profiles))
	fmt.Fprintf(&b, "Метрик: %d · рисков: %d · находок: %d\n\n", len(info.Metrics), len(info.Risks), len(findings))
	if info.Capacity.Players > 0 || info.Capacity.Target != "" {
		fmt.Fprintf(&b, "Capacity-ориентиры:\n")
		fmt.Fprintf(&b, "- Heap: %d МБ\n", info.Capacity.RecommendedHeapMB)
		fmt.Fprintf(&b, "- view-distance: %d\n", info.Capacity.RecommendedViewDistance)
		fmt.Fprintf(&b, "- simulation-distance: %d\n\n", info.Capacity.RecommendedSimulationDistance)
	}
	if len(info.Profiles) > 0 {
		fmt.Fprintf(&b, "Импортированные отчёты:\n")
		for _, profile := range info.Profiles {
			fmt.Fprintf(&b, "- %s (%s): TPS=%s, MSPT avg=%s, p95=%s, max=%s\n", profile.Source, profile.Kind, fmtFloatOrDash(profile.TPS), fmtFloatOrDash(profile.MSPTAvg), fmtFloatOrDash(profile.MSPTP95), fmtFloatOrDash(profile.MSPTMax))
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(info.Risks) > 0 {
		fmt.Fprintf(&b, "Риски:\n")
		for _, risk := range info.Risks {
			fmt.Fprintf(&b, "- [%s] %s: %s\n", risk.Severity, risk.Area, risk.Message)
			if risk.Recommendation != "" {
				fmt.Fprintf(&b, "  Рекомендация: %s\n", risk.Recommendation)
			}
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(info.Metrics) > 0 {
		fmt.Fprintf(&b, "Метрики:\n")
		for _, metric := range info.Metrics {
			fmt.Fprintf(&b, "- [%s] %s = %s", metric.Status, metric.Key, metric.Value)
			if metric.Source != "" {
				fmt.Fprintf(&b, " (%s)", metric.Source)
			}
			fmt.Fprintf(&b, "\n")
		}
	}
	return b.String()
}

func fmtFloat(value float64) string {
	if value == 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return "0"
	}
	return strconv.FormatFloat(value, 'f', 2, 64)
}

func fmtFloatOrDash(value float64) string {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return "-"
	}
	return fmtFloat(value)
}

func firstPositive(values ...float64) float64 {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

func clampTPS(value float64) float64 {
	if value > 20 {
		return 20
	}
	return value
}

func roundUp(value, step int) int {
	if step <= 0 {
		return value
	}
	if value%step == 0 {
		return value
	}
	return value + step - value%step
}

func targetLabel(target string) string {
	switch target {
	case "survival":
		return "survival"
	case "rpg":
		return "RPG/MMO"
	case "minigames":
		return "мини-игры"
	case "custom":
		return "custom"
	case "online":
		return "публичный сервер"
	case "":
		return "-"
	default:
		return target
	}
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
