package production

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gitflic.ru/skif4er/doctortools/internal/model"
)

const defaultBackupMaxAge = 7 * 24 * time.Hour

// Analyze выполняет read-only аудит production-ready состояния Minecraft-серверного проекта.
func Analyze(root string, system model.SystemInfo, mc model.MinecraftInfo) (model.ProductionInfo, []model.Finding) {
	return AnalyzeWithOptions(root, system, mc, model.ProductionOptions{})
}

// AnalyzeWithOptions выполняет Production Doctor 2.0 с дополнительными production-параметрами.
func AnalyzeWithOptions(root string, system model.SystemInfo, mc model.MinecraftInfo, opts model.ProductionOptions) (model.ProductionInfo, []model.Finding) {
	maxAge, normalizedAge := parseBackupMaxAge(opts.BackupMaxAge)
	opts.BackupMaxAge = normalizedAge
	info := model.ProductionInfo{Status: model.SeverityInfo, Options: opts}
	var findings []model.Finding

	info.BackupCandidates = findBackupCandidates(root)
	info.ServiceFiles = mergeUnique(findFilesByPatterns(root, []string{"*.service"}, []string{".", "ops", "deploy", "deployment", "systemd", "infra", "scripts"}), findExternalFiles(opts.SystemdDir, []string{"*.service"})...)
	info.LogrotateFiles = mergeUnique(findFilesByPatterns(root, []string{"*logrotate*", "*.logrotate"}, []string{".", "ops", "deploy", "deployment", "logrotate", "infra", "scripts"}), findExternalFiles(opts.LogrotateDir, []string{"*", "*.conf"})...)
	info.StartupScripts = findStartupScripts(root)
	info.StartupCommands = extractStartupCommands(root, info.StartupScripts, info.ServiceFiles)
	info.StagingCandidates = findNamedPaths(root, []string{"staging", "stage", "test-server", "testing", "preview", "preprod", "sandbox"})
	info.RollbackCandidates = findNamedPaths(root, []string{"rollback", "rollbacks", "snapshots", "snapshot", "releases", "previous", "prev", "restore", "restores"})
	info.ReleaseLayout = detectReleaseLayout(root)
	info.Docker = detectDocker(root)
	info.RestoreChecklist = buildRestoreChecklist(root)

	checkBackups(&info, system, maxAge)
	checkCrashReports(&info, mc)
	checkConfigs(&info, mc)
	checkDisk(&info, system)
	checkDocker(&info)
	checkRestartPolicy(root, &info)
	checkLogRotation(root, &info)
	checkStartupScripts(root, &info)
	checkRollbackAndStaging(&info)
	checkReleaseLayout(&info)
	checkRestoreReadiness(&info)
	checkWritableRuntime(&info, system)

	buildFindings(info, &findings)
	finalize(&info)
	return info, findings
}

func parseBackupMaxAge(raw string) (time.Duration, string) {
	if strings.TrimSpace(raw) == "" {
		return defaultBackupMaxAge, "168h"
	}
	normalized := strings.TrimSpace(strings.ToLower(raw))
	if strings.HasSuffix(normalized, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(normalized, "d"))
		if err == nil && days > 0 {
			return time.Duration(days) * 24 * time.Hour, fmt.Sprintf("%dh", days*24)
		}
	}
	d, err := time.ParseDuration(normalized)
	if err != nil || d <= 0 {
		return defaultBackupMaxAge, "168h"
	}
	return d, normalized
}

func checkBackups(info *model.ProductionInfo, system model.SystemInfo, maxAge time.Duration) {
	if len(info.BackupCandidates) == 0 {
		addCheck(info, "production.backups.not_found", model.SeverityDanger, "бэкапы", "backup strategy", "не найдено", "CraftDoctor не нашёл backup/backups/world_backups/snapshots/restic/borg или похожую структуру бэкапов.", "Настроить автоматические бэкапы миров, конфигов, плагинов и данных. Отдельно проверить восстановление из бэкапа до публичного запуска.", "")
		return
	}
	addCheck(info, "production.backups.detected", model.SeverityInfo, "бэкапы", "backup strategy", "найдено", fmt.Sprintf("Найдены признаки бэкапов: %d.", len(info.BackupCandidates)), "Проверить расписание, ретеншн, внешнее хранилище и реальное восстановление на тестовой копии.", "")
	fresh := false
	threshold := time.Now().Add(-maxAge)
	for _, candidate := range info.BackupCandidates {
		if candidate.SizeBytes == 0 {
			addCheck(info, "production.backups.empty_candidate", model.SeverityWarn, "бэкапы", candidate.Path, "требует проверки", "Backup-кандидат имеет нулевой размер или размер не удалось определить.", "Проверить, что бэкап реально содержит миры, конфиги, плагины и данные, а не является пустой директорией/файлом.", candidate.Path)
		}
		if candidate.Modified == "" {
			continue
		}
		if ts, err := time.Parse(time.RFC3339, candidate.Modified); err == nil && ts.After(threshold) {
			fresh = true
		}
	}
	if !fresh {
		addCheck(info, "production.backups.no_recent_backup", model.SeverityWarn, "бэкапы", "backup freshness", "требует проверки", "Найдены backup-кандидаты, но свежий бэкап за выбранный период не определён.", "Проверить расписание бэкапов и выполнить тестовый restore. Если бэкапы лежат вне директории сервера — задокументировать это в GitFlic Wiki проекта.", "")
	}
	if system.FreeDiskMB > 0 && system.FreeDiskMB < 10240 {
		addCheck(info, "production.backups.disk_risk", model.SeverityDanger, "бэкапы", "disk space", "опасно", "Свободного места меньше 10 ГБ. Бэкапы и обновления могут сорваться.", "Освободить место или подключить внешнее хранилище до запуска регулярных бэкапов и обновлений.", "")
	}
}

func checkCrashReports(info *model.ProductionInfo, mc model.MinecraftInfo) {
	if mc.CrashReports > 0 {
		addCheck(info, "production.crash_reports.present", model.SeverityDanger, "стабильность", "crash-reports", "опасно", fmt.Sprintf("В директории crash-reports найдено отчётов: %d.", mc.CrashReports), "Разобрать последние crash reports перед обновлением, публичным запуском или миграцией сервера.", "crash-reports")
		return
	}
	addCheck(info, "production.crash_reports.none", model.SeverityInfo, "стабильность", "crash-reports", "норма", "Crash reports не обнаружены.", "Сохранять crash reports в бэкапах и анализировать их перед релизами.", "crash-reports")
}

func checkConfigs(info *model.ProductionInfo, mc model.MinecraftInfo) {
	if len(mc.ConfigFiles) == 0 {
		addCheck(info, "production.configs.not_found", model.SeverityWarn, "конфигурация", "server configs", "не найдено", "Не найдены bukkit.yml, spigot.yml, paper/purpur-конфиги или аналогичные файлы.", "Если сервер уже запускался, проверить путь сканирования. Если сервер новый — запустить ядро один раз для генерации конфигов.", "")
		return
	}
	addCheck(info, "production.configs.detected", model.SeverityInfo, "конфигурация", "server configs", "найдено", fmt.Sprintf("Найдены конфигурационные файлы ядра: %d.", len(mc.ConfigFiles)), "Перед релизом хранить ожидаемые изменения конфигов в changelog или GitFlic Wiki.", "")
}

func checkDisk(info *model.ProductionInfo, system model.SystemInfo) {
	if system.FreeDiskMB == 0 {
		return
	}
	if system.FreeDiskMB < 5120 {
		addCheck(info, "production.disk.critical_free_space", model.SeverityDanger, "диск", "free space", "опасно", "Свободного места меньше 5 ГБ. Это критично для логов, карт, бэкапов и обновлений.", "Освободить место до запуска сервера или перенести миры/бэкапы на отдельный диск.", "")
		return
	}
	if system.FreeDiskMB < 20480 {
		addCheck(info, "production.disk.low_free_space", model.SeverityWarn, "диск", "free space", "требует проверки", "Свободного места меньше 20 ГБ. Для production Minecraft-сервера это может быть мало.", "Проверить рост миров, логов, crash reports и размер будущих бэкапов.", "")
		return
	}
	addCheck(info, "production.disk.ok", model.SeverityInfo, "диск", "free space", "норма", fmt.Sprintf("Свободно на диске: %d МБ.", system.FreeDiskMB), "Следить за ростом миров, логов и бэкапов после публичного запуска.", "")
}

func checkDocker(info *model.ProductionInfo) {
	if !info.Docker.Detected {
		addCheck(info, "production.docker.not_detected", model.SeverityInfo, "контейнеризация", "Docker", "не найдено", "Dockerfile/docker-compose рядом с проектом не обнаружены.", "Если сервер разворачивается без Docker — это нормально. Если используется контейнеризация, положите compose/Dockerfile в проект или укажите путь в документации.", "")
		return
	}
	addCheck(info, "production.docker.detected", model.SeverityInfo, "контейнеризация", "Docker", "найдено", "Найдены Docker/Compose-файлы: "+strings.Join(mergeUnique(info.Docker.ComposeFiles, info.Docker.Dockerfiles...), ", ")+".", "Проверить volumes, restart policy, пользователя контейнера и проброс портов.", "")
	if len(info.Docker.ComposeFiles) > 0 && !info.Docker.HasVolumeHints {
		addCheck(info, "production.docker.compose_without_volumes", model.SeverityWarn, "контейнеризация", strings.Join(info.Docker.ComposeFiles, ", "), "требует проверки", "В compose-файлах не найдены явные volumes/bind-mount признаки.", "Для Minecraft production зафиксировать persistent volumes для worlds, plugins, configs, logs и backups.", strings.Join(info.Docker.ComposeFiles, ", "))
	}
	if len(info.Docker.ComposeFiles) > 0 && !info.Docker.HasRestartPolicy {
		addCheck(info, "production.docker.compose_without_restart", model.SeverityWarn, "контейнеризация", strings.Join(info.Docker.ComposeFiles, ", "), "требует проверки", "В compose-файлах не найдена restart policy.", "Добавить restart: unless-stopped/on-failure или осознанно описать внешний supervisor.", strings.Join(info.Docker.ComposeFiles, ", "))
	}
}

func checkRestartPolicy(root string, info *model.ProductionInfo) {
	if len(info.ServiceFiles) == 0 {
		addCheck(info, "production.restart_policy.not_found", model.SeverityWarn, "автозапуск", "systemd/service", "не найдено", "CraftDoctor не нашёл systemd service или похожий service-файл рядом с проектом/в указанной директории.", "Для production-сервера настроить systemd/supervisor/tmux wrapper с понятной restart policy и журналированием.", "")
		return
	}
	addCheck(info, "production.restart_policy.service_found", model.SeverityInfo, "автозапуск", "systemd/service", "найдено", "Найдены service-файлы: "+strings.Join(info.ServiceFiles, ", ")+".", "Проверить пользователя запуска, WorkingDirectory, ExecStart и Restart=on-failure/always.", "")
	for _, rel := range info.ServiceFiles {
		data, err := os.ReadFile(resolve(root, rel))
		if err != nil {
			continue
		}
		text := strings.ToLower(string(data))
		if !strings.Contains(text, "restart=always") && !strings.Contains(text, "restart=on-failure") {
			addCheck(info, "production.restart_policy.missing_restart", model.SeverityWarn, "автозапуск", rel, "требует проверки", "В service-файле не найден Restart=always или Restart=on-failure.", "Добавить осознанную restart policy, чтобы сервер поднимался после аварийного падения, но не скрывал циклический crash-loop.", rel)
		}
		if strings.Contains(text, "user=root") {
			addCheck(info, "production.restart_policy.root_user", model.SeverityWarn, "автозапуск", rel, "требует проверки", "Service-файл запускает сервер от root.", "Создать отдельного системного пользователя для Minecraft-сервера и ограничить права на файлы.", rel)
		}
		if !strings.Contains(text, "workingdirectory=") {
			addCheck(info, "production.restart_policy.working_directory_missing", model.SeverityWarn, "автозапуск", rel, "требует проверки", "В service-файле не найден WorkingDirectory=.", "Явно указать WorkingDirectory на директорию сервера, чтобы относительные пути к конфигам/мирам/логам были предсказуемыми.", rel)
		}
		if !strings.Contains(text, "execstart=") {
			addCheck(info, "production.restart_policy.execstart_missing", model.SeverityDanger, "автозапуск", rel, "опасно", "В service-файле не найден ExecStart=.", "Проверить service-файл: без ExecStart systemd-сервис не сможет корректно запускать сервер.", rel)
		}
	}
}

func checkLogRotation(root string, info *model.ProductionInfo) {
	logsDir := filepath.Join(root, "logs")
	_, logsErr := os.Stat(logsDir)
	if len(info.LogrotateFiles) == 0 && logsErr == nil {
		addCheck(info, "production.log_rotation.not_found", model.SeverityWarn, "логи", "log rotation", "не найдено", "Директория logs есть, но logrotate-конфигурация или похожий механизм ротации не найден рядом с проектом/в указанной директории.", "Настроить ротацию/retention логов, чтобы latest.log и архивы не забивали диск.", "logs")
		return
	}
	if len(info.LogrotateFiles) > 0 {
		addCheck(info, "production.log_rotation.detected", model.SeverityInfo, "логи", "log rotation", "найдено", "Найдены признаки logrotate/ротации логов: "+strings.Join(info.LogrotateFiles, ", ")+".", "Проверить retention, сжатие старых логов и исключение секретов из логирования.", "")
		for _, rel := range info.LogrotateFiles {
			data, err := os.ReadFile(resolve(root, rel))
			if err != nil {
				continue
			}
			text := strings.ToLower(string(data))
			if !strings.Contains(text, "rotate") && !strings.Contains(text, "daily") && !strings.Contains(text, "weekly") {
				addCheck(info, "production.log_rotation.retention_missing", model.SeverityWarn, "логи", rel, "требует проверки", "В logrotate-файле не найден явный retention/rotate interval.", "Указать rotate/daily/weekly/monthly, compress и missingok/notifempty по необходимости.", rel)
			}
		}
	}
}

func checkStartupScripts(root string, info *model.ProductionInfo) {
	if len(info.StartupScripts) == 0 {
		addCheck(info, "production.startup_scripts.not_found", model.SeverityWarn, "запуск", "start scripts", "не найдено", "CraftDoctor не нашёл start.sh/run.sh/start.bat/run.bat или похожие скрипты запуска.", "Зафиксировать запуск сервера в явном скрипте или service-файле, чтобы команда запуска была воспроизводимой.", "")
		return
	}
	addCheck(info, "production.startup_scripts.detected", model.SeverityInfo, "запуск", "start scripts", "найдено", "Найдены скрипты запуска: "+strings.Join(info.StartupScripts, ", ")+".", "Проверить Java-путь, JVM-флаги, WorkingDirectory и отсутствие секретов в аргументах запуска.", "")
	for _, rel := range info.StartupScripts {
		if strings.HasSuffix(strings.ToLower(rel), ".sh") {
			st, err := os.Stat(filepath.Join(root, rel))
			if err == nil && st.Mode().Perm()&0111 == 0 {
				addCheck(info, "production.startup_scripts.not_executable", model.SeverityWarn, "запуск", rel, "требует проверки", "Shell-скрипт запуска не имеет execute-бита.", "Выполнить chmod +x для скрипта или запускать его явно через bash.", rel)
			}
		}
	}
	if len(info.StartupCommands) == 0 {
		addCheck(info, "production.startup_commands.not_extracted", model.SeverityInfo, "запуск", "startup command", "не извлечено", "Команды запуска из скриптов/service-файлов не извлечены.", "Проверьте, что команда Java/ExecStart явно записана в start.sh/run.sh или systemd service.", "")
	}
}

func checkRollbackAndStaging(info *model.ProductionInfo) {
	if len(info.RollbackCandidates) == 0 {
		addCheck(info, "production.rollback.not_found", model.SeverityWarn, "релизы", "rollback", "не найдено", "Признаки rollback/snapshots/releases не найдены.", "Перед обновлением ядра, Java или плагинов иметь понятный rollback-план: бэкап, предыдущие jar/config и инструкцию восстановления.", "")
	} else {
		addCheck(info, "production.rollback.detected", model.SeverityInfo, "релизы", "rollback", "найдено", "Найдены rollback/snapshot/release-кандидаты: "+strings.Join(info.RollbackCandidates, ", ")+".", "Проверить, что rollback реально тестировался и не хранится только формально.", "")
	}
	if len(info.StagingCandidates) == 0 {
		addCheck(info, "production.staging.not_found", model.SeverityInfo, "релизы", "staging", "не найдено", "Staging/test/preprod-копия рядом с проектом не обнаружена.", "Для крупных серверов проверять обновления на staging-копии перед production.", "")
	} else {
		addCheck(info, "production.staging.detected", model.SeverityInfo, "релизы", "staging", "найдено", "Найдены staging/test-кандидаты: "+strings.Join(info.StagingCandidates, ", ")+".", "Поддерживать staging близким к production по Java, ядру, плагинам и данным.", "")
	}
}

func checkReleaseLayout(info *model.ProductionInfo) {
	if info.ReleaseLayout.Current == "" && info.ReleaseLayout.Previous == "" && len(info.ReleaseLayout.Releases) == 0 {
		addCheck(info, "production.release_layout.not_found", model.SeverityInfo, "релизы", "release layout", "не найдено", "Структура current/previous/releases не обнаружена.", "Для простых серверов это нормально. Для крупных проектов лучше хранить current/previous/releases или аналогичный rollback layout.", "")
		return
	}
	addCheck(info, "production.release_layout.detected", model.SeverityInfo, "релизы", "release layout", "найдено", "Найдена структура релизов current/previous/releases.", "Проверить, что переключение current/previous документировано и совместимо с бэкапами миров/БД.", "")
}

func checkRestoreReadiness(info *model.ProductionInfo) {
	if len(info.RestoreChecklist) == 0 {
		addCheck(info, "production.restore.checklist_missing", model.SeverityWarn, "восстановление", "restore checklist", "не найдено", "Не найден restore checklist или инструкция восстановления.", "Добавить RESTORE.md / restore.md / Wiki-страницу с шагами восстановления из бэкапа и проверить её на тестовой копии.", "")
		return
	}
	addCheck(info, "production.restore.checklist_found", model.SeverityInfo, "восстановление", "restore checklist", "найдено", "Найдены restore-инструкции: "+strings.Join(info.RestoreChecklist, ", ")+".", "Периодически выполнять тестовое восстановление и фиксировать дату последней проверки.", "")
}

func checkWritableRuntime(info *model.ProductionInfo, system model.SystemInfo) {
	if !system.WritableRoot {
		addCheck(info, "production.runtime.root_not_writable", model.SeverityDanger, "runtime", "root directory", "опасно", "Текущий пользователь не имеет признаков записи в корневую директорию сервера.", "Проверить владельца файлов и пользователя запуска. Minecraft-сервер должен иметь запись в нужные runtime-директории, но не запускаться от root без необходимости.", "")
	}
	if !system.WritablePlugins {
		addCheck(info, "production.runtime.plugins_not_writable", model.SeverityWarn, "runtime", "plugins directory", "требует проверки", "Текущий пользователь не имеет признаков записи в plugins или директория отсутствует.", "Если сервер плагинный, проверить владельца plugins и права на обновление jar/config. Для Vanilla это может быть нормально.", "plugins")
	}
}

func buildFindings(info model.ProductionInfo, findings *[]model.Finding) {
	for _, check := range info.Checks {
		if check.Severity.Rank() < model.SeverityWarn.Rank() {
			continue
		}
		*findings = append(*findings, model.Finding{ID: check.ID, Severity: check.Severity, Category: "production-ready", Title: titleForCheck(check), Message: check.Message, Recommendation: check.Recommendation, File: check.File})
	}
}

func finalize(info *model.ProductionInfo) {
	status := model.SeverityInfo
	for _, check := range info.Checks {
		if check.Severity.Rank() > status.Rank() {
			status = check.Severity
		}
	}
	info.Status = status
	if len(info.Recommendations) == 0 {
		info.Recommendations = []string{
			"Перед публичным запуском проверьте бэкапы, restore checklist, restart policy, ротацию логов и rollback-план.",
			"После исправлений повторно запустите craftdoctor production check и полный scan.",
		}
	}
	sort.SliceStable(info.Checks, func(i, j int) bool {
		if info.Checks[i].Severity.Rank() == info.Checks[j].Severity.Rank() {
			return info.Checks[i].ID < info.Checks[j].ID
		}
		return info.Checks[i].Severity.Rank() > info.Checks[j].Severity.Rank()
	})
}

func FormatAnalysisText(info model.ProductionInfo, findings []model.Finding) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Production Doctor 2.0\n")
	fmt.Fprintf(&b, "Статус: %s\n", info.Status)
	fmt.Fprintf(&b, "Проверок: %d\n", len(info.Checks))
	fmt.Fprintf(&b, "Backup-кандидатов: %d\n", len(info.BackupCandidates))
	fmt.Fprintf(&b, "Service-файлов: %d\n", len(info.ServiceFiles))
	fmt.Fprintf(&b, "Logrotate-файлов: %d\n", len(info.LogrotateFiles))
	fmt.Fprintf(&b, "Скриптов запуска: %d\n", len(info.StartupScripts))
	fmt.Fprintf(&b, "Команд запуска: %d\n", len(info.StartupCommands))
	fmt.Fprintf(&b, "Docker/Compose: %t\n", info.Docker.Detected)
	fmt.Fprintf(&b, "Rollback-кандидатов: %d\n", len(info.RollbackCandidates))
	fmt.Fprintf(&b, "Staging-кандидатов: %d\n", len(info.StagingCandidates))
	fmt.Fprintf(&b, "Restore-инструкций: %d\n\n", len(info.RestoreChecklist))
	if len(info.StartupCommands) > 0 {
		fmt.Fprintf(&b, "Команды запуска:\n")
		for _, cmd := range info.StartupCommands {
			fmt.Fprintf(&b, "- %s: %s\n", cmd.Source, cmd.Command)
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
	if len(findings) > 0 {
		fmt.Fprintf(&b, "Находки production-ready:\n")
		for _, f := range findings {
			fmt.Fprintf(&b, "- [%s] %s (%s)\n", f.Severity, f.Title, f.ID)
		}
	}
	return b.String()
}

func addCheck(info *model.ProductionInfo, id string, severity model.Severity, area, target, status, message, recommendation, file string) {
	info.Checks = append(info.Checks, model.ProductionCheck{ID: id, Severity: severity, Area: area, Target: target, Status: status, Message: message, Recommendation: recommendation, File: file})
}

func titleForCheck(check model.ProductionCheck) string {
	switch check.ID {
	case "production.backups.not_found":
		return "Стратегия бэкапов не обнаружена"
	case "production.backups.no_recent_backup":
		return "Свежий бэкап не определён"
	case "production.backups.empty_candidate":
		return "Backup-кандидат пустой или непроверяемый"
	case "production.backups.disk_risk":
		return "Риск бэкапов из-за нехватки места"
	case "production.docker.compose_without_volumes":
		return "Compose без явных volumes"
	case "production.docker.compose_without_restart":
		return "Compose без restart policy"
	case "production.restart_policy.not_found":
		return "Restart policy не обнаружена"
	case "production.restart_policy.missing_restart":
		return "В service-файле не найдена restart policy"
	case "production.restart_policy.root_user":
		return "Сервер запускается от root"
	case "production.restart_policy.working_directory_missing":
		return "В service-файле не найден WorkingDirectory"
	case "production.restart_policy.execstart_missing":
		return "В service-файле не найден ExecStart"
	case "production.log_rotation.not_found":
		return "Ротация логов не обнаружена"
	case "production.log_rotation.retention_missing":
		return "В logrotate не найден retention"
	case "production.startup_scripts.not_found":
		return "Скрипты запуска не найдены"
	case "production.startup_scripts.not_executable":
		return "Скрипт запуска не исполняемый"
	case "production.rollback.not_found":
		return "Rollback-план не обнаружен"
	case "production.restore.checklist_missing":
		return "Restore checklist не найден"
	case "production.runtime.root_not_writable":
		return "Нет записи в корневую директорию сервера"
	case "production.runtime.plugins_not_writable":
		return "Нет записи в директорию plugins"
	case "production.disk.critical_free_space":
		return "Критически мало свободного места"
	case "production.disk.low_free_space":
		return "Мало свободного места для production"
	case "production.crash_reports.present":
		return "Найдены crash reports"
	case "production.configs.not_found":
		return "Конфигурационные файлы ядра не найдены"
	default:
		if check.Target != "" {
			return check.Area + ": " + check.Target
		}
		return check.Area
	}
}

func findBackupCandidates(root string) []model.BackupCandidate {
	var out []model.BackupCandidate
	backupNames := []string{"backup", "backups", "world_backups", "world-backups", "server_backups", "server-backups", "borg", "restic", "snapshots"}
	for _, rel := range backupNames {
		path := filepath.Join(root, rel)
		if st, err := os.Stat(path); err == nil && st.IsDir() {
			out = append(out, model.BackupCandidate{Path: rel, Kind: "directory", Modified: st.ModTime().Format(time.RFC3339), SizeBytes: pathSize(path)})
		}
	}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || path == root {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if depth(rel) > 3 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		name := strings.ToLower(d.Name())
		if d.IsDir() {
			if strings.Contains(name, "backup") || strings.Contains(name, "snapshot") {
				if st, err := d.Info(); err == nil {
					out = append(out, model.BackupCandidate{Path: rel, Kind: "directory", Modified: st.ModTime().Format(time.RFC3339), SizeBytes: pathSize(path)})
				}
			}
			return nil
		}
		if strings.Contains(name, "backup") || strings.Contains(name, "snapshot") || strings.Contains(name, "world") {
			if strings.HasSuffix(name, ".zip") || strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz") || strings.HasSuffix(name, ".7z") || strings.HasSuffix(name, ".bak") {
				if st, err := d.Info(); err == nil {
					out = append(out, model.BackupCandidate{Path: rel, Kind: "archive", Modified: st.ModTime().Format(time.RFC3339), SizeBytes: st.Size()})
				}
			}
		}
		return nil
	})
	return uniqueBackupCandidates(out)
}

func detectDocker(root string) model.DockerInfo {
	composeNames := []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml", "deploy/docker-compose.yml", "deploy/docker-compose.yaml", "ops/docker-compose.yml", "infra/docker-compose.yml"}
	dockerfileNames := []string{"Dockerfile", "docker/Dockerfile", "deploy/Dockerfile", "ops/Dockerfile", "infra/Dockerfile"}
	info := model.DockerInfo{}
	for _, rel := range composeNames {
		path := filepath.Join(root, rel)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		info.ComposeFiles = append(info.ComposeFiles, rel)
		text := strings.ToLower(string(data))
		if strings.Contains(text, "volumes:") || strings.Contains(text, "./") || strings.Contains(text, ":/data") || strings.Contains(text, ":/server") {
			info.HasVolumeHints = true
		}
		if strings.Contains(text, "restart:") {
			info.HasRestartPolicy = true
		}
	}
	for _, rel := range dockerfileNames {
		if st, err := os.Stat(filepath.Join(root, rel)); err == nil && !st.IsDir() {
			info.Dockerfiles = append(info.Dockerfiles, rel)
		}
	}
	info.Detected = len(info.ComposeFiles) > 0 || len(info.Dockerfiles) > 0
	return info
}

func detectReleaseLayout(root string) model.ReleaseLayoutInfo {
	info := model.ReleaseLayoutInfo{}
	if st, err := os.Stat(filepath.Join(root, "current")); err == nil && st.IsDir() {
		info.Current = "current"
	}
	if st, err := os.Stat(filepath.Join(root, "previous")); err == nil && st.IsDir() {
		info.Previous = "previous"
	}
	for _, rel := range []string{"releases", "release", "versions"} {
		path := filepath.Join(root, rel)
		entries, err := os.ReadDir(path)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				info.Releases = append(info.Releases, filepath.ToSlash(filepath.Join(rel, e.Name())))
			}
		}
	}
	sort.Strings(info.Releases)
	return info
}

func buildRestoreChecklist(root string) []string {
	candidates := []string{"RESTORE.md", "restore.md", "BACKUP_RESTORE.md", "backup-restore.md", "ops/restore.md", "ops/RESTORE.md", "backup/restore.md", "backups/restore.md", ".gitflic/wiki/restore.md", ".gitflic/wiki/backup-restore.md"}
	var out []string
	for _, rel := range candidates {
		if st, err := os.Stat(filepath.Join(root, rel)); err == nil && !st.IsDir() {
			out = append(out, rel)
		}
	}
	return out
}

func extractStartupCommands(root string, scripts, services []string) []model.StartupCommand {
	var out []model.StartupCommand
	for _, rel := range scripts {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			lower := strings.ToLower(trimmed)
			if strings.HasPrefix(trimmed, "#") || trimmed == "" {
				continue
			}
			if strings.Contains(lower, "java") || strings.Contains(lower, "paper") || strings.Contains(lower, "purpur") || strings.Contains(lower, "spigot") || strings.Contains(lower, "server.jar") {
				out = append(out, model.StartupCommand{Source: rel, Command: trimmed})
				break
			}
		}
	}
	for _, rel := range services {
		data, err := os.ReadFile(resolve(root, rel))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(strings.ToLower(trimmed), "execstart=") {
				out = append(out, model.StartupCommand{Source: rel, Command: strings.TrimSpace(strings.TrimPrefix(trimmed, "ExecStart="))})
			}
		}
	}
	return out
}

func findFilesByPatterns(root string, patterns []string, dirs []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, dir := range dirs {
		base := filepath.Join(root, dir)
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			for _, pattern := range patterns {
				if ok, _ := filepath.Match(pattern, e.Name()); ok {
					rel := filepath.Clean(filepath.Join(dir, e.Name()))
					if rel == "."+string(filepath.Separator)+e.Name() {
						rel = e.Name()
					}
					if !seen[rel] {
						out = append(out, rel)
						seen[rel] = true
					}
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

func findExternalFiles(dir string, patterns []string) []string {
	if strings.TrimSpace(dir) == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		for _, pattern := range patterns {
			if ok, _ := filepath.Match(pattern, e.Name()); ok {
				out = append(out, filepath.Join(dir, e.Name()))
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

func findStartupScripts(root string) []string {
	candidates := []string{"start.sh", "run.sh", "restart.sh", "stop.sh", "start.bat", "run.bat", "server.sh", "minecraft.sh", "scripts/start.sh", "scripts/run.sh", "ops/start.sh", "ops/run.sh"}
	var out []string
	for _, rel := range candidates {
		if st, err := os.Stat(filepath.Join(root, rel)); err == nil && !st.IsDir() {
			out = append(out, rel)
		}
	}
	sort.Strings(out)
	return out
}

func findNamedPaths(root string, names []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, name := range names {
		if st, err := os.Stat(filepath.Join(root, name)); err == nil && st.IsDir() {
			out = append(out, name)
			seen[name] = true
		}
	}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || path == root || !d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if depth(rel) > 2 {
			return filepath.SkipDir
		}
		lower := strings.ToLower(d.Name())
		for _, name := range names {
			if lower == name || strings.Contains(lower, name) {
				if !seen[rel] {
					out = append(out, rel)
					seen[rel] = true
				}
				break
			}
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func uniqueBackupCandidates(in []model.BackupCandidate) []model.BackupCandidate {
	seen := map[string]bool{}
	out := make([]model.BackupCandidate, 0, len(in))
	for _, item := range in {
		if item.Path == "" || seen[item.Path] {
			continue
		}
		seen[item.Path] = true
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func pathSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if st, err := d.Info(); err == nil {
			total += st.Size()
		}
		return nil
	})
	return total
}

func resolve(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

func mergeUnique(base []string, extra ...string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range append(base, extra...) {
		if strings.TrimSpace(v) == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func depth(rel string) int {
	if rel == "." || rel == "" {
		return 0
	}
	return strings.Count(filepath.ToSlash(rel), "/") + 1
}
