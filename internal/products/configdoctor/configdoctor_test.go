package configdoctor

import (
	"os"
	"path/filepath"
	"testing"

	"gitflic.ru/skif4er/doctortools/internal/core"
)

func TestAnalyzeDockerAndEnv(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "Dockerfile"), []byte("FROM alpine:latest\nCOPY . .\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, ".env"), []byte("TOKEN=secret-value\n"), 0644); err != nil {
		t.Fatal(err)
	}
	r, err := Analyze("audit", d, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Tool != "ConfigDoctor" || r.Summary[string(core.SeverityDanger)] == 0 {
		t.Fatalf("ожидался danger finding ConfigDoctor, получено %+v", r.Summary)
	}
}
