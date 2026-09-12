package logs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeFileFindsRootCauses(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "latest.log")
	content := `[12:00:01 WARN]: Can't keep up! Is the server overloaded? Running 5000ms behind, skipping 100 tick(s)
[12:00:02 ERROR]: Could not load 'plugins/EconomyPlus.jar' in folder 'plugins'
org.bukkit.plugin.UnknownDependencyException: Unknown dependency Vault. Please download and install Vault to run this plugin.
[12:00:03 ERROR]: Error occurred while enabling EconomyPlus v1.0.0 (Is it up to date?)
java.sql.SQLException: Access denied for user 'mc'@'localhost'
Caused by: java.lang.ClassNotFoundException: com.example.Missing
`
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	info, findings := AnalyzeFile(logPath)
	if !info.LatestLogFound {
		t.Fatal("ожидался найденный лог")
	}
	if info.LongTickCount != 1 {
		t.Fatalf("ожидался 1 long tick, получено %d", info.LongTickCount)
	}
	if len(info.RootCauses) == 0 {
		t.Fatal("ожидались root cause")
	}
	if len(info.Issues) == 0 {
		t.Fatal("ожидались сгруппированные проблемы")
	}
	if info.MainRootCause == nil {
		t.Fatal("ожидалась главная причина")
	}
	if len(info.StackTraces) == 0 {
		t.Fatal("ожидались stack trace fingerprints")
	}
	if len(info.IssueTypes) == 0 {
		t.Fatal("ожидалась сводка по типам проблем")
	}
	var hasMissing, hasDB bool
	for _, rc := range info.RootCauses {
		if rc.ID == "plugins.missing_dependency" {
			hasMissing = true
		}
		if rc.ID == "database.connection" {
			hasDB = true
		}
	}
	if !hasMissing {
		t.Fatal("не найден root cause plugins.missing_dependency")
	}
	if !hasDB {
		t.Fatal("не найден root cause database.connection")
	}
	var hasFinding bool
	for _, f := range findings {
		if strings.HasPrefix(f.ID, "plugins.") || f.ID == "database.connection" || strings.HasPrefix(f.ID, "logs.root_cause") {
			hasFinding = true
		}
	}
	if !hasFinding {
		t.Fatalf("ожидались findings по root cause, получено %#v", findings)
	}
}

func TestAnalyzeTargetAcceptsServerDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "logs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "logs", "latest.log"), []byte("[12:00:00 INFO]: Done\n"), 0644); err != nil {
		t.Fatal(err)
	}
	info, _, err := AnalyzeTarget(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !info.LatestLogFound {
		t.Fatal("лог должен быть найден")
	}
}

func TestAnalyzeFileLastRun(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "latest.log")
	content := `[12:00:00 INFO]: Starting minecraft server version 1.21.1
[12:00:01 ERROR]: Error occurred while enabling OldPlugin v1.0.0
java.lang.IllegalStateException: old failure
[13:00:00 INFO]: Starting minecraft server version 1.21.1
[13:00:01 WARN]: Can't keep up! Is the server overloaded? Running 1000ms behind
`
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	info, _ := AnalyzeFileWithOptions(logPath, Options{LastRun: true})
	if len(info.Sessions) < 2 {
		t.Fatalf("ожидались две сессии, получено %d", len(info.Sessions))
	}
	if info.SelectedSession == nil || info.SelectedSession.Index != 2 {
		t.Fatalf("ожидалась последняя выбранная сессия, получено %#v", info.SelectedSession)
	}
	if info.ErrorCount != 0 {
		t.Fatalf("ошибка из первой сессии не должна учитываться, получено ERROR=%d", info.ErrorCount)
	}
	if info.LongTickCount != 1 {
		t.Fatalf("ожидался long tick из второй сессии")
	}
}
