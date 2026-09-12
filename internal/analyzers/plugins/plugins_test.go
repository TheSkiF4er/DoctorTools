package plugins

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"gitflic.ru/skif4er/doctortools/internal/model"
)

func TestParsePluginYAML(t *testing.T) {
	text := `name: TestPlugin
version: "1.0.0"
main: ru.example.TestPlugin
api-version: '1.20'
author: SkiF4er
depend: [Vault, LuckPerms]
softdepend:
  - PlaceholderAPI
  - spark
loadbefore:
  - ChatPlugin
provides: [LegacyName]
`
	p := parsePluginYAML(text)
	if p.Name != "TestPlugin" {
		t.Fatalf("name mismatch: %q", p.Name)
	}
	if p.Version != "1.0.0" {
		t.Fatalf("version mismatch: %q", p.Version)
	}
	if len(p.Depends) != 2 {
		t.Fatalf("depends mismatch: %#v", p.Depends)
	}
	if len(p.SoftDepends) != 2 {
		t.Fatalf("softdepends mismatch: %#v", p.SoftDepends)
	}
	if len(p.LoadBefore) != 1 || p.LoadBefore[0] != "ChatPlugin" {
		t.Fatalf("loadbefore mismatch: %#v", p.LoadBefore)
	}
	if len(p.Provides) != 1 || p.Provides[0] != "LegacyName" {
		t.Fatalf("provides mismatch: %#v", p.Provides)
	}
	if len(p.Authors) != 1 || p.Authors[0] != "SkiF4er" {
		t.Fatalf("authors mismatch: %#v", p.Authors)
	}
}

func TestAnalyzeDetailedDetectsMissingDependencyDuplicateAndCategories(t *testing.T) {
	root := t.TempDir()
	pluginsDir := filepath.Join(root, "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		t.Fatal(err)
	}
	writePluginJar(t, filepath.Join(pluginsDir, "Shop.jar"), `name: ShopGUIPlus
version: 1.0.0
main: ru.example.Shop
depend: [Vault, MissingCore]
`)
	writePluginJar(t, filepath.Join(pluginsDir, "Shop-copy.jar"), `name: ShopGUIPlus
version: 1.0.1
main: ru.example.Shop
`)
	writePluginJar(t, filepath.Join(pluginsDir, "Vault.jar"), `name: Vault
version: 1.7
main: net.milkbowl.vault.Vault
`)

	list, audit, findings := AnalyzeDetailed(root)
	if len(list) != 3 {
		t.Fatalf("plugins mismatch: %d", len(list))
	}
	if audit.Total != 3 || audit.Valid != 3 || audit.Invalid != 0 {
		t.Fatalf("audit counters mismatch: %#v", audit)
	}
	if len(audit.MissingDependencies) != 1 || audit.MissingDependencies[0].To != "MissingCore" {
		t.Fatalf("missing dependency mismatch: %#v", audit.MissingDependencies)
	}
	if len(audit.DuplicateNames) == 0 {
		t.Fatalf("expected duplicate names in audit")
	}
	if len(audit.Categories) == 0 {
		t.Fatalf("expected plugin categories")
	}
	foundMissing := false
	foundDuplicate := false
	for _, f := range findings {
		if f.ID == "plugins.dependency.missing.shopguiplus.missingcore" {
			foundMissing = true
		}
		if f.ID == "plugins.duplicate.shopguiplus" {
			foundDuplicate = true
		}
	}
	if !foundMissing || !foundDuplicate {
		t.Fatalf("expected missing and duplicate findings, got %#v", findings)
	}
}

func TestDetectCycles(t *testing.T) {
	list := pluginInfoFixtures{
		{name: "A", depends: []string{"B"}},
		{name: "B", depends: []string{"C"}},
		{name: "C", depends: []string{"A"}},
	}.toModel()
	cycles := detectCycles(list)
	if len(cycles) != 1 {
		t.Fatalf("expected one cycle, got %#v", cycles)
	}
}

type pluginInfoFixtures []pluginInfoFixture

type pluginInfoFixture struct {
	name    string
	depends []string
}

func (items pluginInfoFixtures) toModel() []model.PluginInfo {
	out := make([]model.PluginInfo, 0, len(items))
	for _, item := range items {
		out = append(out, model.PluginInfo{Name: item.name, Depends: item.depends, Valid: true})
	}
	return out
}

func writePluginJar(t *testing.T, path, pluginYAML string) {
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
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPluginIntelligenceDetectsMetadataClassesAndGraph(t *testing.T) {
	root := t.TempDir()
	pluginsDir := filepath.Join(root, "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		t.Fatal(err)
	}
	writePluginJarWithFiles(t, filepath.Join(pluginsDir, "Modern.jar"), map[string]string{
		"plugin.yml": `name: ModernPlugin
version: 2.0.0
main: ru.example.ModernPlugin
api-version: 1.21
folia-supported: true
libraries: [com.zaxxer:HikariCP:5.1.0]
commands:
  modern:
    description: Main command
permissions:
  modern.admin:
    default: op
`,
		"ru/example/ModernPlugin.class":        "class",
		"com/zaxxer/hikari/HikariConfig.class": "class",
	})
	writePluginJarWithFiles(t, filepath.Join(pluginsDir, "Legacy.jar"), map[string]string{
		"plugin.yml": `name: LegacyPlugin
version: 1.0.0
main: ru.example.LegacyPlugin
depend: [ModernPlugin]
`,
		"ru/example/LegacyPlugin.class": "class",
	})

	list, audit, findings := AnalyzeDetailed(root)
	if len(list) != 2 {
		t.Fatalf("plugins mismatch: %#v", list)
	}
	var modern model.PluginInfo
	for _, p := range list {
		if p.Name == "ModernPlugin" {
			modern = p
		}
	}
	if len(modern.Libraries) != 1 || len(modern.Commands) != 1 || len(modern.Permissions) != 1 {
		t.Fatalf("expected libraries/commands/permissions, got %#v", modern)
	}
	if !modern.MainClassPresent || modern.ClassCount == 0 || len(modern.ShadedLibraries) == 0 {
		t.Fatalf("expected class intelligence, got %#v", modern)
	}
	if len(audit.Graph.Nodes) != 2 || len(audit.Graph.Edges) == 0 {
		t.Fatalf("expected graph, got %#v", audit.Graph)
	}
	foundAPIMissing := false
	for _, f := range findings {
		if f.ID == "plugins.api_version.missing.legacyplugin" {
			foundAPIMissing = true
		}
	}
	if !foundAPIMissing {
		t.Fatalf("expected api-version finding, got %#v", findings)
	}
}

func writePluginJarWithFiles(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
