package craftadvanced

import (
    "fmt"
    "io/fs"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "time"

    "gitflic.ru/skif4er/doctortools/internal/model"
)

// Analyze выполняет расширенный CraftDoctor-аудит Minecraft-сервера: миры,
// JVM-флаги, proxy-layout и production-gate признаки.
func Analyze(root string, mc model.MinecraftInfo, perf model.PerformanceInfo, proxy model.ProxyInfo, production model.ProductionInfo) (model.CraftAdvancedInfo, []model.Finding) {
    info := model.CraftAdvancedInfo{Status: model.SeverityInfo}
    var findings []model.Finding

    info.Worlds = inspectWorlds(root, mc.Properties)
    findings = append(findings, buildWorldFindings(info.Worlds, mc.Properties)...)

    info.JavaFlagChecks = inspectJavaFlags(perf)
    findings = append(findings, buildJavaFindings(info.JavaFlagChecks)...)

    info.GateChecks = buildGateChecks(mc, perf, proxy, production)
    findings = append(findings, buildGateFindings(info.GateChecks)...)

    info.Recommendations = buildRecommendations(info)
    info.Status = statusFromFindings(findings)
    return info, findings
}

func inspectWorlds(root string, props map[string]string) []model.CraftWorldInfo {
    names := map[string]string{}
    levelName := strings.TrimSpace(props["level-name"])
    if levelName == "" {
        levelName = "world"
    }
    names[levelName] = "overworld"
    names[levelName+"_nether"] = "nether"
    names[levelName+"_the_end"] = "end"

    entries, _ := os.ReadDir(root)
    for _, e := range entries {
        if !e.IsDir() {
            continue
        }
        name := e.Name()
        if hasFile(filepath.Join(root, name, "level.dat")) || hasDir(filepath.Join(root, name, "region")) {
            if _, ok := names[name]; !ok {
                names[name] = detectWorldKind(name)
            }
        }
    }

    worlds := make([]model.CraftWorldInfo, 0, len(names))
    for name, kind := range names {
        path := filepath.Join(root, name)
        w := model.CraftWorldInfo{Name: name, Path: name, Kind: kind, Exists: hasDir(path)}
        if w.Exists {
            w.HasLevelDat = hasFile(filepath.Join(path, "level.dat"))
            w.HasSessionLock = hasFile(filepath.Join(path, "session.lock"))
            w.HasUIDDat = hasFile(filepath.Join(path, "uid.dat"))
            w.RegionFiles, w.RegionSizeMB = countFiles(path, []string{".mca"}, []string{filepath.Join("region")})
            _, w.EntitiesSizeMB = countFiles(path, []string{".mca"}, []string{filepath.Join("entities")})
            w.PlayerDataFiles, _ = countFiles(filepath.Join(path, "playerdata"), []string{".dat"}, nil)
            w.DataPacks = countDatapacks(filepath.Join(path, "datapacks"))
            if st, err := os.Stat(path); err == nil {
                w.LastModified = st.ModTime().Format(time.RFC3339)
            }
        }
        worlds = append(worlds, w)
    }
    sort.Slice(worlds, func(i, j int) bool { return worlds[i].Name < worlds[j].Name })
    return worlds
}

func inspectJavaFlags(perf model.PerformanceInfo) []model.CraftJavaFlagCheck {
    flags := strings.ToLower(strings.Join(perf.JVMFlags, " "))
    checks := []model.CraftJavaFlagCheck{}
    add := func(id string, sev model.Severity, status, msg, rec string) {
        checks = append(checks, model.CraftJavaFlagCheck{ID: id, Severity: sev, Status: status, Message: msg, Recommendation: rec})
    }
    if len(perf.JVMFlags) == 0 {
        add("craft.java.flags.not_found", model.SeverityWarn, "не найдено", "Не найдены явные JVM-флаги запуска.", "Хранить запуск в start.sh, user_jvm_args.txt или *.args и ревизовать -Xmx/-Xms/GC.")
        return checks
    }
    add("craft.java.flags.detected", model.SeverityInfo, "найдено", fmt.Sprintf("Найдено JVM-флагов: %d.", len(perf.JVMFlags)), "Проверить флаги в связке с Java, ядром, RAM и профилем нагрузки.")
    if perf.HeapMaxMB == 0 {
        add("craft.java.xmx.missing", model.SeverityWarn, "требует настройки", "Не найден -Xmx.", "Задать явный -Xmx и оставить запас RAM для ОС, proxy, базы и файлового кэша.")
    }
    if perf.HeapMinMB > 0 && perf.HeapMaxMB > 0 && perf.HeapMinMB != perf.HeapMaxMB {
        add("craft.java.xms_xmx.mismatch", model.SeverityInfo, "требует проверки", "-Xms и -Xmx различаются.", "Для стабильного heap-профиля Minecraft-сервера часто используют одинаковые значения.")
    }
    if !strings.Contains(flags, "useg1gc") && !strings.Contains(flags, "usezgc") && !strings.Contains(flags, "useshenandoahgc") {
        add("craft.java.gc.not_explicit", model.SeverityInfo, "не задано", "В JVM-флагах не найден явный GC.", "Для большинства Paper/Purpur-серверов использовать G1GC как базовый вариант; ZGC тестировать отдельно.")
    }
    modernG1 := []string{"useg1gc", "parallelrefprocenabled", "maxgcpausemillis", "g1newsizepercent", "g1maxnewsizepercent", "g1heapsizeregion"}
    missing := []string{}
    for _, token := range modernG1 {
        if !strings.Contains(flags, token) {
            missing = append(missing, token)
        }
    }
    if strings.Contains(flags, "useg1gc") && len(missing) > 0 {
        add("craft.java.g1.partial", model.SeverityInfo, "частичная настройка", "G1GC включён, но часть типовых Minecraft G1-настроек не найдена: "+strings.Join(missing, ", ")+".", "Сравнить запуск с актуальными рекомендациями Paper/Aikar и проверить результат через spark/timings.")
    }
    legacy := map[string]string{"useconcmarksweepgc": "CMS GC устарел и удалён в новых Java.", "maxpermsize": "PermGen не используется в современных Java.", "aggresiveopts": "AggressiveOpts устарел.", "useparallelgc": "ParallelGC редко оптимален для современных Minecraft-серверов."}
    for token, msg := range legacy {
        if strings.Contains(flags, strings.ToLower(token)) {
            add("craft.java.legacy."+token, model.SeverityWarn, "устаревший флаг", msg, "Убрать устаревший флаг и протестировать современный профиль запуска на Java 17/21.")
        }
    }
    return checks
}

func buildGateChecks(mc model.MinecraftInfo, perf model.PerformanceInfo, proxy model.ProxyInfo, production model.ProductionInfo) []model.CraftGateCheck {
    checks := []model.CraftGateCheck{}
    add := func(id string, sev model.Severity, area, status, msg, rec string) {
        checks = append(checks, model.CraftGateCheck{ID: id, Severity: sev, Area: area, Status: status, Message: msg, Recommendation: rec})
    }
    if !mc.EULAAccepted {
        add("craft.gate.eula", model.SeverityCritical, "minecraft", "блокер", "EULA не принята или файл eula.txt не найден.", "Принять EULA осознанно и зафиксировать eula=true перед production-запуском.")
    }
    if perf.HeapMaxMB == 0 {
        add("craft.gate.heap", model.SeverityWarn, "java", "требует настройки", "Production-gate не видит -Xmx.", "Задать явный heap limit.")
    }
    if proxy.Detected {
        add("craft.gate.proxy.detected", model.SeverityInfo, "proxy", "найдено", "Обнаружена proxy-конфигурация.", "Проверить forwarding secret, backend bind и firewall.")
    } else if strings.EqualFold(mc.Properties["online-mode"], "false") {
        add("craft.gate.offline_without_proxy", model.SeverityCritical, "security", "опасно", "online-mode=false без обнаруженной proxy-конфигурации.", "Включить online-mode=true либо настроить proxy forwarding и закрыть backend от прямого доступа.")
    }
    if !production.Docker.Detected && len(production.ServiceFiles) == 0 && len(production.StartupScripts) == 0 {
        add("craft.gate.startup.not_found", model.SeverityWarn, "production", "не найдено", "Не найден явный production-способ запуска: Docker, systemd или startup script.", "Описать запуск через systemd/Docker/start.sh и добавить restart-policy.")
    }
    if len(production.BackupCandidates) == 0 {
        add("craft.gate.backup.not_found", model.SeverityWarn, "backup", "не найдено", "Не найдены кандидаты резервных копий.", "Настроить регулярные backup world/plugins/config и проверить restore-процедуру.")
    }
    return checks
}

func buildWorldFindings(worlds []model.CraftWorldInfo, props map[string]string) []model.Finding {
    var fs []model.Finding
    for _, w := range worlds {
        if !w.Exists {
            sev := model.SeverityWarn
            if w.Kind == "overworld" {
                sev = model.SeverityDanger
            }
            fs = append(fs, finding("craft.world.missing", sev, "миры", w.Path, "Мир не найден: "+w.Name, "Проверить level-name в server.properties и наличие директории мира."))
            continue
        }
        if !w.HasLevelDat {
            fs = append(fs, finding("craft.world.level_dat.missing", model.SeverityDanger, "миры", w.Path, "В мире "+w.Name+" не найден level.dat.", "Проверить целостность мира и корректность пути."))
        }
        if !w.HasSessionLock {
            fs = append(fs, finding("craft.world.session_lock.missing", model.SeverityWarn, "миры", w.Path, "В мире "+w.Name+" не найден session.lock.", "Проверить, что мир не повреждён и корректно открывается сервером."))
        }
        if w.RegionFiles > 15000 || w.RegionSizeMB > 10240 {
            fs = append(fs, finding("craft.world.region.large", model.SeverityInfo, "миры", w.Path, fmt.Sprintf("Мир %s крупный: region-файлов %d, размер около %d МБ.", w.Name, w.RegionFiles, w.RegionSizeMB), "Планировать backup/restore, pregeneration, trim неиспользуемых регионов и мониторинг диска."))
        }
        if w.DataPacks > 0 {
            fs = append(fs, finding("craft.world.datapacks.detected", model.SeverityInfo, "миры", w.Path, fmt.Sprintf("В мире %s найдено datapack: %d.", w.Name, w.DataPacks), "Проверить совместимость datapack с версией ядра перед обновлениями."))
        }
    }
    if len(worlds) > 0 {
        fs = append(fs, finding("craft.world.audit.done", model.SeverityInfo, "миры", "", fmt.Sprintf("Проверено миров: %d.", len(worlds)), "Использовать `craftdoctor world audit` для отдельного отчёта по мирам."))
    }
    return fs
}

func buildJavaFindings(checks []model.CraftJavaFlagCheck) []model.Finding {
    fs := []model.Finding{}
    for _, c := range checks {
        fs = append(fs, finding(c.ID, c.Severity, "java", "", c.Message, c.Recommendation))
    }
    return fs
}

func buildGateFindings(checks []model.CraftGateCheck) []model.Finding {
    fs := []model.Finding{}
    for _, c := range checks {
        fs = append(fs, finding(c.ID, c.Severity, c.Area, "", c.Message, c.Recommendation))
    }
    return fs
}

func buildRecommendations(info model.CraftAdvancedInfo) []string {
    recs := []string{}
    if len(info.Worlds) > 0 {
        recs = append(recs, "Перед обновлением ядра или массовыми изменениями плагинов делать backup миров и тестовый запуск на staging-копии.")
    }
    if len(info.JavaFlagChecks) > 0 {
        recs = append(recs, "JVM-флаги проверять не изолированно, а вместе с Java version, heap, spark-профилем и реальной нагрузкой.")
    }
    if len(info.GateChecks) > 0 {
        recs = append(recs, "Production-gate должен проходить до публикации сервера или крупного релиза сборки.")
    }
    return recs
}

func FormatWorldText(info model.CraftAdvancedInfo) string {
    var b strings.Builder
    fmt.Fprintln(&b, "CraftDoctor World Audit")
    fmt.Fprintf(&b, "Статус: %s\n", info.Status)
    if len(info.Worlds) == 0 {
        fmt.Fprintln(&b, "Миры не обнаружены.")
        return b.String()
    }
    for _, w := range info.Worlds {
        fmt.Fprintf(&b, "\n- %s (%s)\n", w.Name, w.Kind)
        fmt.Fprintf(&b, "  exists=%v level.dat=%v session.lock=%v uid.dat=%v\n", w.Exists, w.HasLevelDat, w.HasSessionLock, w.HasUIDDat)
        if w.Exists {
            fmt.Fprintf(&b, "  region=%d файлов / %d МБ, entities=%d МБ, playerdata=%d, datapacks=%d\n", w.RegionFiles, w.RegionSizeMB, w.EntitiesSizeMB, w.PlayerDataFiles, w.DataPacks)
        }
    }
    return b.String()
}

func FormatJavaText(info model.CraftAdvancedInfo) string {
    var b strings.Builder
    fmt.Fprintln(&b, "CraftDoctor Java Flags Audit")
    if len(info.JavaFlagChecks) == 0 {
        fmt.Fprintln(&b, "Проверки JVM-флагов не сформированы.")
        return b.String()
    }
    for _, c := range info.JavaFlagChecks {
        fmt.Fprintf(&b, "- [%s] %s: %s\n", c.Severity, c.Status, c.Message)
        if c.Recommendation != "" {
            fmt.Fprintf(&b, "  Рекомендация: %s\n", c.Recommendation)
        }
    }
    return b.String()
}

func FormatGateText(info model.CraftAdvancedInfo) string {
    var b strings.Builder
    fmt.Fprintln(&b, "CraftDoctor Production Gate")
    fmt.Fprintf(&b, "Статус: %s\n", info.Status)
    for _, c := range info.GateChecks {
        fmt.Fprintf(&b, "- [%s] %s / %s: %s\n", c.Severity, c.Area, c.Status, c.Message)
        if c.Recommendation != "" {
            fmt.Fprintf(&b, "  Рекомендация: %s\n", c.Recommendation)
        }
    }
    return b.String()
}

func finding(id string, severity model.Severity, category, file, message, rec string) model.Finding {
    title := message
    if len(title) > 96 {
        title = title[:96]
    }
    return model.Finding{ID: id, Severity: severity, Category: category, Title: title, Message: message, Recommendation: rec, File: file}
}

func detectWorldKind(name string) string {
    lower := strings.ToLower(name)
    switch {
    case strings.Contains(lower, "nether"):
        return "nether"
    case strings.Contains(lower, "end"):
        return "end"
    default:
        return "custom"
    }
}

func hasFile(path string) bool { st, err := os.Stat(path); return err == nil && !st.IsDir() }
func hasDir(path string) bool { st, err := os.Stat(path); return err == nil && st.IsDir() }

func countDatapacks(path string) int {
    entries, err := os.ReadDir(path)
    if err != nil { return 0 }
    n := 0
    for _, e := range entries { if e.IsDir() || strings.HasSuffix(strings.ToLower(e.Name()), ".zip") { n++ } }
    return n
}

func countFiles(root string, exts []string, requiredParts []string) (int, int64) {
    st, err := os.Stat(root)
    if err != nil || !st.IsDir() { return 0, 0 }
    extSet := map[string]bool{}
    for _, e := range exts { extSet[strings.ToLower(e)] = true }
    var count int
    var bytes int64
    _ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
        if err != nil || d.IsDir() { return nil }
        if len(requiredParts) > 0 {
            ok := false
            clean := filepath.ToSlash(path)
            for _, p := range requiredParts { if strings.Contains(clean, filepath.ToSlash(p)) { ok = true; break } }
            if !ok { return nil }
        }
        if len(extSet) > 0 && !extSet[strings.ToLower(filepath.Ext(path))] { return nil }
        count++
        if info, err := d.Info(); err == nil { bytes += info.Size() }
        return nil
    })
    return count, bytes / 1024 / 1024
}

func statusFromFindings(findings []model.Finding) model.Severity {
    status := model.SeverityInfo
    for _, f := range findings { if f.Severity.Rank() > status.Rank() { status = f.Severity } }
    return status
}
