package report

import (
	"strings"
	"testing"
	"time"

	"gitflic.ru/skif4er/doctortools/internal/model"
)

func TestMarkdownContainsPriorityBlock(t *testing.T) {
	rep := &model.Report{
		ToolVersion: "test",
		GeneratedAt: time.Unix(0, 0),
		TargetPath:  "/tmp/server",
		Findings: []model.Finding{
			{ID: "info", Severity: model.SeverityInfo, Category: "test", Title: "Информация"},
			{ID: "critical", Severity: model.SeverityCritical, Category: "test", Title: "Критичная проблема", Recommendation: "Исправить"},
		},
	}
	rep.Finalize()
	out := Markdown(rep)
	if !strings.Contains(out, "## Что исправить первым") {
		t.Fatalf("markdown report does not contain priority block:\n%s", out)
	}
	if !strings.Contains(out, "[CRITICAL] Критичная проблема") {
		t.Fatalf("markdown report does not contain critical finding in priority block:\n%s", out)
	}
}

func TestHTMLContainsPriorityBlock(t *testing.T) {
	rep := &model.Report{
		ToolVersion: "test",
		GeneratedAt: time.Unix(0, 0),
		TargetPath:  "/tmp/server",
		Findings: []model.Finding{
			{ID: "danger", Severity: model.SeverityDanger, Category: "test", Title: "Опасная проблема", Recommendation: "Исправить"},
		},
	}
	rep.Finalize()
	out := HTML(rep)
	if !strings.Contains(out, "Что исправить первым") {
		t.Fatalf("html report does not contain priority block:\n%s", out)
	}
	if !strings.Contains(out, "Опасная проблема") {
		t.Fatalf("html report does not contain finding title:\n%s", out)
	}
}

func TestReportsContainPerformanceDoctor(t *testing.T) {
	rep := &model.Report{
		ToolVersion: "test",
		GeneratedAt: time.Unix(0, 0),
		TargetPath:  "/tmp/server",
		Performance: model.PerformanceInfo{
			Status:    model.SeverityWarn,
			HeapMinMB: 1024,
			HeapMaxMB: 4096,
			Metrics:   []model.PerformanceMetric{{Key: "view-distance", Value: "16", Source: "server.properties", Status: model.SeverityWarn}},
			Risks:     []model.PerformanceRisk{{ID: "performance.logs.long_tick", Severity: model.SeverityWarn, Area: "логи", Message: "Long tick"}},
		},
	}
	rep.Finalize()
	md := Markdown(rep)
	if !strings.Contains(md, "## Performance Doctor") {
		t.Fatalf("markdown report does not contain Performance Doctor block:\n%s", md)
	}
	html := HTML(rep)
	if !strings.Contains(html, "Performance Doctor") {
		t.Fatalf("html report does not contain Performance Doctor block:\n%s", html)
	}
}

func TestHTMLReportV3ContainsDashboardSearchPrintAndRiskMatrix(t *testing.T) {
	rep := &model.Report{
		ToolVersion: "test",
		GeneratedAt: time.Unix(0, 0),
		TargetPath:  "/tmp/server",
		Findings: []model.Finding{
			{ID: "critical", Severity: model.SeverityCritical, Category: "безопасность", Title: "Критичная проблема", Recommendation: "Исправить"},
			{ID: "warn", Severity: model.SeverityWarn, Category: "производительность", Title: "Предупреждение"},
		},
	}
	rep.Finalize()
	out := HTML(rep)
	for _, needle := range []string{
		"HTML Report 3.0",
		"Doctor-модули",
		"Сводка по категориям",
		"Команды для повторной проверки",
		"data-filter=\"CRITICAL\"",
		"data-severity=\"CRITICAL\"",
		"score-ring",
		"findingSearch",
		"Печать / PDF",
		"Score breakdown",
		"Risk matrix",
		"Находки по категориям",
		"Экспорт и передача отчёта",
	} {
		if !strings.Contains(out, needle) {
			t.Fatalf("html report v3 does not contain %q:\n%s", needle, out)
		}
	}
}
