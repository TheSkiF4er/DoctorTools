package app

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionJSON(t *testing.T) {
	var out, errb bytes.Buffer
	if err := Run([]string{"version", "--json"}, &out, &errb); err != nil {
		t.Fatalf("version --json вернул ошибку: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("version --json должен возвращать JSON: %v", err)
	}
	if payload["version"] != "3.0.1" {
		t.Fatalf("ожидалась версия 3.0.1, получено %v", payload["version"])
	}
}

func TestSchemaReport(t *testing.T) {
	var out, errb bytes.Buffer
	if err := Run([]string{"schema", "report"}, &out, &errb); err != nil {
		t.Fatalf("schema report вернул ошибку: %v", err)
	}
	if !strings.Contains(out.String(), "DoctorTools Report") || !strings.Contains(out.String(), "findings") {
		t.Fatalf("schema report не содержит ожидаемые маркеры")
	}
}

func TestCIQualityGateExitCode(t *testing.T) {
	dir := t.TempDir()
	server := filepath.Join(dir, "server")
	if err := os.MkdirAll(server, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(server, "server.properties"), []byte("online-mode=false\n"), 0644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	err := Run([]string{"ci", server, "--stdout"}, &out, &errb)
	if err == nil {
		t.Fatalf("ci должен вернуть ошибку quality gate для сервера без jar")
	}
	if code := ExitCode(err); code != ExitQualityGate {
		t.Fatalf("ожидался exit code %d, получен %d: %v", ExitQualityGate, code, err)
	}
	if !strings.Contains(out.String(), "\"tool_version\": \"3.0.1\"") {
		t.Fatalf("ci должен вывести JSON-отчёт")
	}
}

func TestConfigAuditJSON(t *testing.T) {
	dir := t.TempDir()
	server := filepath.Join(dir, "server")
	if err := os.MkdirAll(server, 0755); err != nil {
		t.Fatal(err)
	}
	content := []byte("online-mode=true\nview-distance=12\n")
	if err := os.WriteFile(filepath.Join(server, "server.properties"), content, 0644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if err := Run([]string{"config", "audit", server, "--format", "json"}, &out, &errb); err != nil {
		t.Fatalf("config audit вернул ошибку: %v", err)
	}
	if !strings.Contains(out.String(), "\"config\"") || !strings.Contains(out.String(), "server.properties") {
		t.Fatalf("config audit должен вывести JSON с блоком config, получено: %s", out.String())
	}
}

func TestRulesValidate(t *testing.T) {
	var out, errb bytes.Buffer
	if err := Run([]string{"rules", "validate", "--format", "json"}, &out, &errb); err != nil {
		t.Fatalf("rules validate вернул ошибку: %v", err)
	}
	if !strings.Contains(out.String(), "\"valid\": true") || !strings.Contains(out.String(), "\"rules\":") {
		t.Fatalf("rules validate должен вывести валидный JSON-отчёт, получено: %s", out.String())
	}
}

func TestPluginsGraphAndExplain(t *testing.T) {
	dir := t.TempDir()
	server := filepath.Join(dir, "server")
	pluginsDir := filepath.Join(server, "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestPluginJar(t, filepath.Join(pluginsDir, "Core.jar"), `name: CorePlugin
version: 1.0.0
main: ru.example.CorePlugin
api-version: 1.21
`)
	writeTestPluginJar(t, filepath.Join(pluginsDir, "Addon.jar"), `name: AddonPlugin
version: 1.0.0
main: ru.example.AddonPlugin
depend: [CorePlugin]
`)
	var out, errb bytes.Buffer
	if err := Run([]string{"plugins", "graph", server, "--format", "mermaid"}, &out, &errb); err != nil {
		t.Fatalf("plugins graph вернул ошибку: %v", err)
	}
	if !strings.Contains(out.String(), "graph TD") || !strings.Contains(out.String(), "CorePlugin") {
		t.Fatalf("plugins graph должен вывести mermaid-граф, получено: %s", out.String())
	}
	out.Reset()
	if err := Run([]string{"plugins", "explain", server, "AddonPlugin", "--format", "json"}, &out, &errb); err != nil {
		t.Fatalf("plugins explain вернул ошибку: %v", err)
	}
	if !strings.Contains(out.String(), "\"plugin\"") || !strings.Contains(out.String(), "AddonPlugin") {
		t.Fatalf("plugins explain должен вывести JSON по плагину, получено: %s", out.String())
	}
}

func TestPerformanceScanWithCapacityOptions(t *testing.T) {
	dir := t.TempDir()
	server := filepath.Join(dir, "server")
	if err := os.MkdirAll(filepath.Join(server, "reports"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(server, "server.properties"), []byte("view-distance=12\nsimulation-distance=10\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(server, "start.sh"), []byte("java -Xms4G -Xmx4G -XX:+UseG1GC -jar paper.jar"), 0755); err != nil {
		t.Fatal(err)
	}
	profile := []byte("spark report TPS: 16.8 average MSPT: 62.5 p95 MSPT: 90.0 max MSPT: 110.0 chunks entities scheduler")
	if err := os.WriteFile(filepath.Join(server, "reports", "spark-profile.txt"), profile, 0644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if err := Run([]string{"performance", "scan", server, "--players", "80", "--target", "rpg", "--format", "json"}, &out, &errb); err != nil {
		t.Fatalf("performance scan вернул ошибку: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "\"players\": 80") || !strings.Contains(text, "spark-profile.txt") || !strings.Contains(text, "capacity") {
		t.Fatalf("performance scan должен вывести capacity и импортированный profile, получено: %s", text)
	}
}

func writeTestPluginJar(t *testing.T, path, pluginYAML string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("plugin.yml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(pluginYAML)); err != nil {
		t.Fatal(err)
	}
	if _, err := zw.Create("ru/example/CorePlugin.class"); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLogsAnalyzeLastRunJSON(t *testing.T) {
	dir := t.TempDir()
	server := filepath.Join(dir, "server")
	logsDir := filepath.Join(server, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `[12:00:00 INFO]: Starting minecraft server version 1.21.1
[12:00:01 ERROR]: Error occurred while enabling OldPlugin v1.0.0
java.lang.IllegalStateException: old failure
[13:00:00 INFO]: Starting minecraft server version 1.21.1
[13:00:01 ERROR]: Error occurred while enabling NewPlugin v1.0.0
java.lang.IllegalStateException: new failure
`
	if err := os.WriteFile(filepath.Join(logsDir, "latest.log"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if err := Run([]string{"logs", "analyze", server, "--last-run", "--format", "json"}, &out, &errb); err != nil {
		t.Fatalf("logs analyze --last-run вернул ошибку: %v", err)
	}
	if !strings.Contains(out.String(), "selected_session") || !strings.Contains(out.String(), "stack_traces") {
		t.Fatalf("logs analyze должен вывести Log Doctor 2.0 JSON, получено: %s", out.String())
	}
}
