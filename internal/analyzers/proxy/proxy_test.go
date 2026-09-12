package proxy

import (
	"os"
	"path/filepath"
	"testing"

	"gitflic.ru/skif4er/doctortools/internal/analyzers/minecraft"
	"gitflic.ru/skif4er/doctortools/internal/model"
)

func TestAnalyzeVelocityModernMissingSecret(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "server.properties"), "online-mode=false\nserver-ip=0.0.0.0\n")
	writeFile(t, filepath.Join(dir, "velocity.toml"), `bind = "0.0.0.0:25565"
player-info-forwarding-mode = "modern"
forwarding-secret-file = "forwarding.secret"

[servers]
lobby = "127.0.0.1:25566"

[forced-hosts]
"play.example.org" = ["missing"]
`)
	mc, _ := minecraft.Analyze(dir)
	info, findings := Analyze(dir, mc)
	if !info.Detected {
		t.Fatal("ожидался обнаруженный Velocity proxy")
	}
	if info.Status != model.SeverityCritical {
		t.Fatalf("ожидался CRITICAL, получен %s", info.Status)
	}
	assertCheck(t, info, "proxy.velocity.secret.missing")
	assertCheck(t, info, "proxy.velocity.forced_host.unknown_server")
	if len(info.Nodes) == 0 || len(info.Edges) == 0 {
		t.Fatalf("ожидалась карта сети, nodes=%d edges=%d", len(info.Nodes), len(info.Edges))
	}
	if len(findings) == 0 {
		t.Fatal("ожидались findings")
	}
}

func TestAnalyzeOfflineWithoutProxy(t *testing.T) {
	mc := model.MinecraftInfo{Properties: map[string]string{"online-mode": "false"}}
	info, _ := Analyze(t.TempDir(), mc)
	assertCheck(t, info, "proxy.backend.offline_without_proxy")
}

func TestAnalyzeBungee(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.yml"), `online_mode: false
ip_forward: false
listeners:
- host: 0.0.0.0:25565
servers:
  lobby:
    address: 10.0.0.5:25566
`)
	mc := model.MinecraftInfo{Properties: map[string]string{"online-mode": "false"}}
	info, _ := Analyze(dir, mc)
	assertCheck(t, info, "proxy.bungee.ip_forward.disabled")
	assertCheck(t, info, "proxy.bungee.online_mode.disabled")
}

func assertCheck(t *testing.T, info model.ProxyInfo, id string) {
	t.Helper()
	for _, check := range info.Checks {
		if check.ID == id {
			return
		}
	}
	t.Fatalf("проверка %s не найдена среди %+v", id, info.Checks)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
