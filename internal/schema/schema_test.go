package schema

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReportJSONSchemaIsValidJSON(t *testing.T) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(ReportJSONSchema), &payload); err != nil {
		t.Fatalf("JSON Schema должна быть валидным JSON: %v", err)
	}
	if payload["title"] != "DoctorTools Report" {
		t.Fatalf("неожиданный title схемы: %v", payload["title"])
	}
	if !strings.Contains(ReportJSONSchema, "suppressed_findings") {
		t.Fatalf("схема должна описывать suppressed_findings")
	}
}
