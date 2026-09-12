package rules

import (
	"os"
	"path/filepath"
	"testing"

	"gitflic.ru/skif4er/doctortools/internal/model"
)

func TestCatalogFindWildcard(t *testing.T) {
	catalog := BuiltinCatalog()
	if _, ok := catalog.Find("plugins.dependency.missing.vault"); !ok {
		t.Fatal("wildcard rule was not found")
	}
}

func TestIgnoreMatcher(t *testing.T) {
	matcher := IgnoreMatcher{Patterns: []string{"plugins.duplicate.*", "severity:INFO", "category:логи"}}
	cases := []model.Finding{
		{ID: "plugins.duplicate.example", Severity: model.SeverityDanger, Category: "плагины"},
		{ID: "minecraft.core.detected", Severity: model.SeverityInfo, Category: "minecraft"},
		{ID: "logs.errors.detected", Severity: model.SeverityDanger, Category: "логи"},
	}
	for _, f := range cases {
		if !matcher.Match(f) {
			t.Fatalf("finding should be ignored: %#v", f)
		}
	}
}

func TestLoadIgnore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".craftdoctorignore")
	if err := os.WriteFile(path, []byte("# test\nlogs.*\n"), 0644); err != nil {
		t.Fatal(err)
	}
	matcher, err := LoadIgnore(dir, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(matcher.Patterns) != 1 || matcher.Patterns[0] != "logs.*" {
		t.Fatalf("unexpected patterns: %#v", matcher.Patterns)
	}
}

func TestEmbeddedCatalogValidation(t *testing.T) {
	report, err := ValidateEmbeddedCatalog()
	if err != nil {
		t.Fatalf("встроенный каталог должен валидироваться без ошибки: %v", err)
	}
	if !report.Valid {
		t.Fatalf("встроенный каталог должен быть валидным: %#v", report.Issues)
	}
	if report.Rules < 100 {
		t.Fatalf("ожидался полный каталог правил, получено %d", report.Rules)
	}
}

func TestValidateCatalogDetectsDuplicate(t *testing.T) {
	catalog := Catalog{Version: "test", Rules: []Rule{
		{ID: "test.duplicate", Severity: model.SeverityInfo, Category: "тест", Title: "Тест", Description: "Описание"},
		{ID: "test.duplicate", Severity: model.SeverityWarn, Category: "тест", Title: "Тест 2", Description: "Описание 2"},
	}}
	report := ValidateCatalog(catalog)
	if report.Valid {
		t.Fatal("каталог с дублем id не должен быть валидным")
	}
}
