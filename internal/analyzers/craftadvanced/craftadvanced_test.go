package craftadvanced

import (
	"os"
	"path/filepath"
	"testing"

	"gitflic.ru/skif4er/doctortools/internal/model"
)

func TestAnalyzeWorldsAndGate(t *testing.T) {
	dir := t.TempDir()
	world := filepath.Join(dir, "world")
	if err := os.MkdirAll(filepath.Join(world, "region"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(world, "level.dat"), []byte("demo"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(world, "region", "r.0.0.mca"), []byte("demo"), 0644); err != nil {
		t.Fatal(err)
	}
	mc := model.MinecraftInfo{EULAAccepted: false, Properties: map[string]string{"level-name": "world", "online-mode": "false"}}
	perf := model.PerformanceInfo{}
	info, findings := Analyze(dir, mc, perf, model.ProxyInfo{}, model.ProductionInfo{})
	if len(info.Worlds) == 0 {
		t.Fatalf("ожидались найденные миры")
	}
	if len(info.GateChecks) == 0 || len(findings) == 0 {
		t.Fatalf("ожидались production-gate проверки и findings")
	}
	if info.Status != model.SeverityCritical {
		t.Fatalf("ожидался CRITICAL статус, получено %s", info.Status)
	}
}

func TestAnalyzeJavaFlags(t *testing.T) {
	perf := model.PerformanceInfo{JVMFlags: []string{"-Xms2G", "-Xmx2G", "-XX:+UseConcMarkSweepGC"}, HeapMinMB: 2048, HeapMaxMB: 2048}
	info, findings := Analyze(t.TempDir(), model.MinecraftInfo{EULAAccepted: true, Properties: map[string]string{"level-name": "world"}}, perf, model.ProxyInfo{}, model.ProductionInfo{})
	if len(info.JavaFlagChecks) == 0 {
		t.Fatalf("ожидались проверки JVM-флагов")
	}
	foundLegacy := false
	for _, f := range findings {
		if f.ID == "craft.java.legacy.useconcmarksweepgc" {
			foundLegacy = true
		}
	}
	if !foundLegacy {
		t.Fatalf("ожидалась находка по устаревшему CMS GC")
	}
}
