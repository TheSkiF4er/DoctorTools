package version

import "testing"

func TestValidateRegistry(t *testing.T) {
	checks := ValidateRegistry()
	if len(checks) != 8 {
		t.Fatalf("ожидалось 8 проверок реестра, получено %d", len(checks))
	}
	for _, check := range checks {
		if check.Status != "ok" {
			t.Fatalf("registry check failed: %+v", check)
		}
	}
}
