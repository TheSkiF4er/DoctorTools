package logdoctor

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeDetectsLogPatterns(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	content := "INFO start\nERROR database connection refused\nFATAL service stopped\npanic: runtime error\nPermission denied\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	report, err := Analyze("analyze", dir, Options{MaxLines: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Files) != 1 {
		t.Fatalf("ожидался 1 файл, получено %d", len(report.Files))
	}
	if report.Summary["danger"] == 0 || report.Summary["warn"] == 0 {
		t.Fatalf("ожидались danger/warn находки, summary=%v", report.Summary)
	}
	if report.Files[0].Fatal == 0 || report.Files[0].Panics == 0 || report.Files[0].Errors == 0 {
		t.Fatalf("паттерны не распознаны: %+v", report.Files[0])
	}
}

func TestRunJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.err"), []byte("ERROR failed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Run([]string{"analyze", dir, "--json"}, &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "\"tool\": \"LogDoctor\"") {
		t.Fatalf("JSON-вывод не похож на отчёт LogDoctor: %s", out.String())
	}
}
