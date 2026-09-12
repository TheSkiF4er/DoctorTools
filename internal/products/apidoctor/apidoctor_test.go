package apidoctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeOpenAPIAndRoutes(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "openapi.yaml"), []byte("openapi: 3.0.3\ninfo:\n  title: Test\n  version: 1.0.0\npaths:\n  /health:\n    get:\n      responses:\n        '200':\n          description: ok\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(d, "routes"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "routes", "api.js"), []byte("app.get('/health', health);\napp.post('/users', createUser);\n"), 0644); err != nil {
		t.Fatal(err)
	}
	r, err := Analyze("scan", d, Options{Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Endpoints) < 2 {
		t.Fatalf("ожидались endpoints, получено %d", len(r.Endpoints))
	}
	if len(r.Files) == 0 {
		t.Fatal("ожидались API-файлы")
	}
}

func TestAnalyzeMissingOpenAPI(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "main.go"), []byte("func main(){}"), 0644); err != nil {
		t.Fatal(err)
	}
	r, err := Analyze("openapi validate", d, Options{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range r.Findings {
		if f.ID == "api.openapi.missing" {
			found = true
		}
	}
	if !found {
		t.Fatal("ожидалась находка api.openapi.missing")
	}
}
