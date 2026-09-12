package production

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitflic.ru/skif4er/doctortools/internal/model"
)

func TestAnalyzeDetectsProductionRisks(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "server.properties"), "online-mode=true\n")
	mustWrite(t, filepath.Join(root, "paper-world-defaults.yml"), "keep-spawn-loaded: true\n")
	mustWrite(t, filepath.Join(root, "start.sh"), "#!/usr/bin/env bash\njava -jar paper.jar nogui\n")
	mustWrite(t, filepath.Join(root, "minecraft.service"), "[Service]\nUser=root\nExecStart=/srv/minecraft/start.sh\n")
	mustMkdir(t, filepath.Join(root, "logs"))
	mustMkdir(t, filepath.Join(root, "crash-reports"))
	mustWrite(t, filepath.Join(root, "crash-reports", "crash-1.txt"), "crash")

	mc := model.MinecraftInfo{ConfigFiles: []string{"paper-world-defaults.yml"}, CrashReports: 1, Properties: map[string]string{"online-mode": "true"}}
	sys := model.SystemInfo{FreeDiskMB: 9000, WritableRoot: true, WritablePlugins: false}
	info, findings := Analyze(root, sys, mc)

	if info.Status.Rank() < model.SeverityDanger.Rank() {
		t.Fatalf("ожидался статус не ниже DANGER, получен %s", info.Status)
	}
	assertHasCheck(t, info, "production.backups.not_found")
	assertHasCheck(t, info, "production.crash_reports.present")
	assertHasCheck(t, info, "production.restart_policy.missing_restart")
	assertHasCheck(t, info, "production.restart_policy.root_user")
	assertHasCheck(t, info, "production.log_rotation.not_found")
	assertHasCheck(t, info, "production.rollback.not_found")
	assertHasFinding(t, findings, "production.backups.not_found")

	text := FormatAnalysisText(info, findings)
	if !strings.Contains(text, "Production Doctor") || !strings.Contains(text, "production.backups.not_found") {
		t.Fatalf("текстовый вывод не содержит ожидаемые данные: %s", text)
	}
}

func TestAnalyzeDetectsBackupCandidates(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "backups"))
	mustWrite(t, filepath.Join(root, "backups", "world-backup.zip"), "backup")
	mustWrite(t, filepath.Join(root, "logrotate.conf"), "/srv/minecraft/logs/*.log {}")
	mustWrite(t, filepath.Join(root, "minecraft.service"), "[Service]\nUser=minecraft\nRestart=on-failure\n")
	mustWrite(t, filepath.Join(root, "start.sh"), "#!/usr/bin/env bash\njava -jar paper.jar nogui\n")
	_ = os.Chmod(filepath.Join(root, "start.sh"), 0755)

	mc := model.MinecraftInfo{ConfigFiles: []string{"paper-global.yml"}, CrashReports: 0}
	sys := model.SystemInfo{FreeDiskMB: 50000, WritableRoot: true, WritablePlugins: true}
	info, _ := Analyze(root, sys, mc)
	if len(info.BackupCandidates) == 0 {
		t.Fatalf("ожидались backup-кандидаты")
	}
	assertHasCheck(t, info, "production.backups.detected")
	assertHasCheck(t, info, "production.restart_policy.service_found")
	assertHasCheck(t, info, "production.log_rotation.detected")
}

func assertHasCheck(t *testing.T, info model.ProductionInfo, id string) {
	t.Helper()
	for _, check := range info.Checks {
		if check.ID == id {
			return
		}
	}
	t.Fatalf("проверка %s не найдена: %#v", id, info.Checks)
}

func assertHasFinding(t *testing.T, findings []model.Finding, id string) {
	t.Helper()
	for _, finding := range findings {
		if finding.ID == id {
			return
		}
	}
	t.Fatalf("находка %s не найдена: %#v", id, findings)
}

func mustWrite(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
}

func TestAnalyzeProductionDoctor2DetectsDockerRestoreAndExternalDirs(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "server.properties"), "online-mode=true\n")
	mustWrite(t, filepath.Join(root, "paper-global.yml"), "")
	mustWrite(t, filepath.Join(root, "start.sh"), "#!/usr/bin/env bash\njava -Xms1G -Xmx2G -jar paper.jar nogui\n")
	if err := os.Chmod(filepath.Join(root, "start.sh"), 0755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "docker-compose.yml"), "services:\n  minecraft:\n    image: eclipse-temurin:21\n    restart: unless-stopped\n    volumes:\n      - ./world:/server/world\n")
	mustWrite(t, filepath.Join(root, "Dockerfile"), "FROM eclipse-temurin:21\n")
	mustWrite(t, filepath.Join(root, "RESTORE.md"), "# Восстановление\n")
	mustMkdir(t, filepath.Join(root, "current"))
	mustMkdir(t, filepath.Join(root, "previous"))
	mustMkdir(t, filepath.Join(root, "releases", "1.0.0"))
	mustMkdir(t, filepath.Join(root, "backups"))
	mustWrite(t, filepath.Join(root, "backups", "world.zip"), "backup")

	systemdDir := filepath.Join(root, "external-systemd")
	logrotateDir := filepath.Join(root, "external-logrotate")
	mustWrite(t, filepath.Join(systemdDir, "minecraft.service"), "[Service]\nUser=minecraft\nWorkingDirectory=/srv/minecraft\nExecStart=/srv/minecraft/start.sh\nRestart=on-failure\n")
	mustWrite(t, filepath.Join(logrotateDir, "minecraft"), "/srv/minecraft/logs/*.log { weekly rotate 4 compress }\n")

	mc := model.MinecraftInfo{ConfigFiles: []string{"paper-global.yml"}, CrashReports: 0}
	sys := model.SystemInfo{FreeDiskMB: 50000, WritableRoot: true, WritablePlugins: true}
	info, findings := AnalyzeWithOptions(root, sys, mc, model.ProductionOptions{SystemdDir: systemdDir, LogrotateDir: logrotateDir, BackupMaxAge: "24h"})

	if !info.Docker.Detected || len(info.Docker.ComposeFiles) == 0 || len(info.Docker.Dockerfiles) == 0 {
		t.Fatalf("ожидался Docker/Compose в Production Doctor 2.0: %#v", info.Docker)
	}
	if len(info.StartupCommands) == 0 {
		t.Fatalf("ожидались извлечённые команды запуска")
	}
	if len(info.RestoreChecklist) == 0 {
		t.Fatalf("ожидался restore checklist")
	}
	if info.ReleaseLayout.Current == "" || info.ReleaseLayout.Previous == "" || len(info.ReleaseLayout.Releases) == 0 {
		t.Fatalf("ожидался release layout: %#v", info.ReleaseLayout)
	}
	assertHasCheck(t, info, "production.docker.detected")
	assertHasCheck(t, info, "production.restore.checklist_found")
	assertHasCheck(t, info, "production.release_layout.detected")
	if len(findings) != 0 && info.Status.Rank() >= model.SeverityDanger.Rank() {
		t.Fatalf("не ожидался DANGER/CRITICAL для подготовленного production-проекта: status=%s findings=%#v", info.Status, findings)
	}
}
