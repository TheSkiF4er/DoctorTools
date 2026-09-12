package performance

import (
	"os"
	"path/filepath"
	"testing"

	"gitflic.ru/skif4er/doctortools/internal/model"
)

func TestAnalyzeDetectsPerformanceRisks(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "server.properties"), "view-distance=16\nsimulation-distance=12\nmax-tick-time=0\nnetwork-compression-threshold=-1\n")
	write(t, filepath.Join(root, "start.sh"), "java -Xms1G -Xmx3900M -jar paper.jar nogui\n")
	write(t, filepath.Join(root, "paper-world-defaults.yml"), "keep-spawn-loaded: true\n")
	info, findings := Analyze(root, model.SystemInfo{TotalMemoryMB: 4096}, model.JavaInfo{Found: true, Major: 21}, model.MinecraftInfo{Properties: map[string]string{"view-distance": "16", "simulation-distance": "12", "max-tick-time": "0", "network-compression-threshold": "-1"}}, nil, model.LogInfo{LongTickCount: 2})
	if info.HeapMaxMB != 3900 {
		t.Fatalf("ожидался Xmx 3900 МБ, получено %d", info.HeapMaxMB)
	}
	if len(info.Risks) == 0 {
		t.Fatal("ожидались риски производительности")
	}
	if len(findings) == 0 {
		t.Fatal("ожидались findings производительности")
	}
	want := map[string]bool{"performance.metric.view.distance": false, "performance.jvm.xmx.too_high": false, "performance.logs.long_tick": false}
	for _, f := range findings {
		if _, ok := want[f.ID]; ok {
			want[f.ID] = true
		}
	}
	for id, ok := range want {
		if !ok {
			t.Fatalf("ожидалась находка %s, findings=%v", id, findings)
		}
	}
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}
