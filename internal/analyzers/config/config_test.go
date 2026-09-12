package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitflic.ru/skif4er/doctortools/internal/model"
)

func TestAnalyzeDetectsConfigProblems(t *testing.T) {
	dir := t.TempDir()
	props := strings.Join([]string{
		"online_mode=false",
		"online-mode=maybe",
		"view-distance=40",
		"view-distance=12",
		"snooper-enabled=true",
		"enable-query=true",
	}, "\n")
	if err := os.WriteFile(filepath.Join(dir, "server.properties"), []byte(props), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "paper-world-defaults.yml"), []byte("settings:\n\tkeep-spawn-loaded: true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "velocity.toml"), []byte("player-info-forwarding-mode = \"modern\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	info, findings := Analyze(dir, model.MinecraftInfo{})
	if info.Status.Rank() < model.SeverityDanger.Rank() {
		t.Fatalf("ожидался статус DANGER или выше, получено %s", info.Status)
	}
	assertFinding(t, findings, "config.properties.invalid_bool.online_mode")
	assertFinding(t, findings, "config.properties.duplicate_key.view_distance")
	assertFinding(t, findings, "config.properties.deprecated_key.snooper_enabled")
	assertFinding(t, findings, "config.yaml.tabs.paper_world_defaults_yml")
	assertFinding(t, findings, "config.velocity.secret_file.missing")
}

func TestFormatAnalysisText(t *testing.T) {
	info := model.ConfigInfo{Status: model.SeverityWarn, Files: []model.ConfigFileInfo{{Path: "server.properties", Kind: "properties", Valid: true, KeyCount: 2}}}
	text := FormatAnalysisText(info, nil)
	if !strings.Contains(text, "Config Doctor") || !strings.Contains(text, "server.properties") {
		t.Fatalf("неожиданный текстовый вывод: %s", text)
	}
}

func assertFinding(t *testing.T, findings []model.Finding, id string) {
	t.Helper()
	for _, f := range findings {
		if f.ID == id {
			return
		}
	}
	t.Fatalf("не найдена находка %s; найдено: %#v", id, findings)
}
