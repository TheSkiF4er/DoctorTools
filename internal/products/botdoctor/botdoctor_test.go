package botdoctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeDetectsTelegramEnvAndTokenLeak(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "package.json"), `{"dependencies":{"telegraf":"^4.0.0"}}`)
	mustWrite(t, filepath.Join(dir, "bot.js"), `const token = "123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghi"; bot.launch();`)
	report, err := Analyze("scan", dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Tool != "BotDoctor" {
		t.Fatalf("unexpected tool: %s", report.Tool)
	}
	if report.Summary["critical"] == 0 {
		t.Fatalf("expected critical token finding: %#v", report.Findings)
	}
	if report.Summary["danger"] == 0 {
		t.Fatalf("expected env finding: %#v", report.Findings)
	}
}

func TestAnalyzeBotHostSystemdRestart(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".env.example"), "DISCORD_BOT_TOKEN=\n")
	mustWrite(t, filepath.Join(dir, "bot.service"), "[Service]\nExecStart=/usr/bin/bot\n")
	report, err := Analyze("bothost check", dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary["warn"] == 0 {
		t.Fatalf("expected warning about systemd restart: %#v", report.Findings)
	}
}

func mustWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}
