package deploydoctor

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"gitflic.ru/skif4er/doctortools/internal/core"
)

func TestAnalyzeArchiveFindsForbiddenFiles(t *testing.T) {
	d := t.TempDir()
	zipPath := filepath.Join(d, "release.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create(".env")
	_, _ = w.Write([]byte("TOKEN=secret"))
	w, _ = zw.Create("node_modules/pkg/index.js")
	_, _ = w.Write([]byte("console.log(1)"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	r, err := Analyze("archive check", zipPath, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Tool != "DeployDoctor" || r.Summary[string(core.SeverityDanger)] == 0 {
		t.Fatalf("ожидался danger finding DeployDoctor, получено %+v", r.Summary)
	}
}
