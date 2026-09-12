package stackdoctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeDetectsFullstackProblems(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "package.json"), `{"scripts":{"dev":"vite"},"dependencies":{"react":"latest"}}`)
	mustWrite(t, filepath.Join(dir, ".env"), "TOKEN=real-token")
	r, err := Analyze("scan", dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Components) == 0 {
		t.Fatal("components not detected")
	}
	if r.Summary["danger"] == 0 {
		t.Fatal("expected danger findings")
	}
}

func TestAnalyzeEnvAudit(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".env.example"), "APP_ENV=production\n")
	r, err := Analyze("env audit", dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Tool != "StackDoctor" {
		t.Fatalf("unexpected tool %s", r.Tool)
	}
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
