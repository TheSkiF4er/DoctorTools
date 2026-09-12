package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitflic.ru/skif4er/doctortools/internal/model"
)

func TestAnalyzeDetectsSecurityRisks(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "server.properties"), []byte("online-mode=false\nenable-rcon=true\nrcon.password=changeme\nserver-ip=0.0.0.0\nwhite-list=false\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "plugins", "Notifier"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugins", "Notifier", "config.yml"), []byte("webhook: https://discord.com/api/webhooks/123456789012345678/abcdefghijklmnopqrstuvwxyzABCDEF\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mc := model.MinecraftInfo{Properties: map[string]string{"online-mode": "false", "enable-rcon": "true", "rcon.password": "changeme", "server-ip": "0.0.0.0", "white-list": "false"}, ProxyForwarded: true}
	info, findings := Analyze(dir, model.SystemInfo{}, mc, nil)
	if info.Status != model.SeverityCritical {
		t.Fatalf("ожидался CRITICAL-статус, получено %s", info.Status)
	}
	if len(info.Secrets) == 0 {
		t.Fatalf("ожидалось обнаружение секрета")
	}
	joined := FormatAnalysisText(info, findings)
	if !strings.Contains(joined, "Security Doctor") || !strings.Contains(joined, "security.rcon.enabled") {
		t.Fatalf("неожиданный текстовый вывод: %s", joined)
	}
	if !hasFinding(findings, "security.rcon.enabled") || !hasFinding(findings, "security.secrets.discord_webhook") {
		t.Fatalf("ожидались findings по RCON и webhook, получено: %#v", findings)
	}
}

func hasFinding(findings []model.Finding, id string) bool {
	for _, f := range findings {
		if f.ID == id {
			return true
		}
	}
	return false
}

func TestAnalyzeSecurityDoctor20SecretAllowlistAndScore(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "server.properties"), []byte("online-mode=true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".craftdoctor-secrets-ignore"), []byte("line:allowed-false-positive\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config", "secrets.yml"), []byte("ignored_token: allowed-false-positive-12345678901234567890\ntelegram_token: 1234567890:ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghiJKLMN\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mc := model.MinecraftInfo{Properties: map[string]string{"online-mode": "true"}}
	plugins := []model.PluginInfo{{Name: "DebugTools", JarFile: "DebugTools.jar", Valid: true}, {Name: "AdminPanel", JarFile: "AdminPanel.jar", Valid: true}}
	info, findings := Analyze(dir, model.SystemInfo{}, mc, plugins)
	if info.Score <= 0 || info.Score >= 100 {
		t.Fatalf("ожидался сниженный security score, получено %d", info.Score)
	}
	if info.SecretsIgnoreFile == "" || len(info.SecretsIgnorePatterns) != 1 {
		t.Fatalf("ожидался загруженный .craftdoctor-secrets-ignore, получено %#v", info.SecretsIgnorePatterns)
	}
	if len(info.Secrets) != 1 || info.Secrets[0].Kind != "Telegram bot token" {
		t.Fatalf("ожидался только Telegram bot token после allowlist, получено %#v", info.Secrets)
	}
	if !hasFinding(findings, "security.secrets.telegram_bot_token") || !hasFinding(findings, "security.plugins.debug_detected") {
		t.Fatalf("ожидались findings Security Doctor 2.0, получено: %#v", findings)
	}
	text := FormatAnalysisText(info, findings)
	if !strings.Contains(text, "Security Doctor 2.0") || !strings.Contains(text, "Security score") || !strings.Contains(text, "Allowlist секретов") {
		t.Fatalf("текстовый вывод не содержит маркеры Security Doctor 2.0: %s", text)
	}
}
