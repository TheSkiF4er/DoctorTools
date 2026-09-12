package minecraft

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadProperties(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.properties")
	data := "# comment\nonline-mode=false\nview-distance=8\n"
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	props, err := ReadProperties(path)
	if err != nil {
		t.Fatal(err)
	}
	if props["online-mode"] != "false" {
		t.Fatalf("unexpected online-mode: %q", props["online-mode"])
	}
	if props["view-distance"] != "8" {
		t.Fatalf("unexpected view-distance: %q", props["view-distance"])
	}
}
