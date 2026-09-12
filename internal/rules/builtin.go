package rules

import (
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed catalog/*.json
var embeddedCatalogFS embed.FS

func EmbeddedCatalog() (Catalog, error) {
	entries, err := embeddedCatalogFS.ReadDir("catalog")
	if err != nil {
		return Catalog{}, fmt.Errorf("не удалось открыть встроенный каталог правил: %w", err)
	}
	out := Catalog{Version: CatalogVersion}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || entry.Name() == "catalog.json" {
			continue
		}
		path := filepath.Join("catalog", entry.Name())
		data, err := embeddedCatalogFS.ReadFile(path)
		if err != nil {
			return Catalog{}, fmt.Errorf("не удалось прочитать встроенный файл правил %s: %w", path, err)
		}
		var module Catalog
		if err := json.Unmarshal(data, &module); err != nil {
			return Catalog{}, fmt.Errorf("не удалось разобрать встроенный файл правил %s: %w", path, err)
		}
		out.Rules = append(out.Rules, module.Rules...)
	}
	out.Rules = deduplicateRules(out.Rules)
	if report := ValidateCatalog(out); !report.Valid {
		return Catalog{}, fmt.Errorf("встроенный каталог правил не прошёл проверку: %s", firstValidationMessage(report.Issues))
	}
	return out, nil
}

func EmbeddedCatalogIndex() (CatalogIndex, error) {
	data, err := embeddedCatalogFS.ReadFile("catalog/catalog.json")
	if err != nil {
		return CatalogIndex{}, fmt.Errorf("не удалось прочитать индекс встроенного каталога правил: %w", err)
	}
	var index CatalogIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return CatalogIndex{}, fmt.Errorf("не удалось разобрать индекс встроенного каталога правил: %w", err)
	}
	sort.Strings(index.Modules)
	return index, nil
}

func ValidateEmbeddedCatalog() (ValidationReport, error) {
	catalog, err := EmbeddedCatalog()
	if err != nil {
		return ValidationReport{}, err
	}
	report := ValidateCatalog(catalog)
	index, err := EmbeddedCatalogIndex()
	if err == nil {
		report.Files = append(report.Files, index.Modules...)
	}
	return report, nil
}
