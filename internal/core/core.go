package core

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityDanger   Severity = "danger"
	SeverityWarn     Severity = "warn"
	SeverityInfo     Severity = "info"
)

type Product struct {
	ID          string   `json:"id"`
	Alias       string   `json:"alias"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Profile     string   `json:"profile"`
	Commands    []string `json:"commands"`
	Status      string   `json:"status"`
}

type Context struct {
	Product   Product
	Command   string
	Target    string
	StartedAt time.Time
}

type Finding struct {
	ID       string   `json:"id"`
	Severity Severity `json:"severity"`
	Title    string   `json:"title"`
	Message  string   `json:"message"`
	Path     string   `json:"path,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

type Report struct {
	Tool       string            `json:"tool"`
	Version    string            `json:"tool_version"`
	Platform   string            `json:"platform"`
	Target     string            `json:"target"`
	Profile    string            `json:"profile"`
	StartedAt  string            `json:"started_at"`
	FinishedAt string            `json:"finished_at"`
	Summary    map[string]int    `json:"summary"`
	Findings   []Finding         `json:"findings"`
	Artifacts  map[string]string `json:"artifacts"`
}

type Analyzer interface {
	ID() string
	Name() string
	Analyze(Context) ([]Finding, error)
}

type AnalyzerFunc struct {
	AnalyzerID   string
	AnalyzerName string
	Fn           func(Context) ([]Finding, error)
}

func (a AnalyzerFunc) ID() string                             { return a.AnalyzerID }
func (a AnalyzerFunc) Name() string                           { return a.AnalyzerName }
func (a AnalyzerFunc) Analyze(ctx Context) ([]Finding, error) { return a.Fn(ctx) }

type Engine struct {
	analyzers []Analyzer
}

func NewEngine(analyzers ...Analyzer) Engine {
	return Engine{analyzers: analyzers}
}

func (e Engine) Run(product Product, command, target string) Report {
	started := time.Now().UTC()
	ctx := Context{Product: product, Command: command, Target: target, StartedAt: started}
	findings := []Finding{{
		ID:       "doctorcore.profile.ready",
		Severity: SeverityInfo,
		Title:    "Профиль диагностики доступен",
		Message:  product.Description,
		Path:     target,
		Tags:     []string{"doctorcore", product.Profile},
	}}
	artifacts := map[string]string{"command": command, "engine": "DoctorCore", "engine_version": "3.0.1"}
	for _, analyzer := range e.analyzers {
		result, err := analyzer.Analyze(ctx)
		artifacts["analyzer."+analyzer.ID()] = analyzer.Name()
		if err != nil {
			findings = append(findings, Finding{
				ID:       "doctorcore.analyzer.error." + analyzer.ID(),
				Severity: SeverityWarn,
				Title:    "Анализатор завершился с предупреждением",
				Message:  err.Error(),
				Path:     target,
				Tags:     []string{"doctorcore", "analyzer"},
			})
			continue
		}
		findings = append(findings, result...)
	}
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Severity == findings[j].Severity {
			return findings[i].ID < findings[j].ID
		}
		return severityRank(findings[i].Severity) < severityRank(findings[j].Severity)
	})
	return Report{
		Tool:       product.Name,
		Version:    product.Version,
		Platform:   "DoctorTools",
		Target:     target,
		Profile:    product.Profile,
		StartedAt:  started.Format(time.RFC3339),
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
		Summary:    Summarize(findings),
		Findings:   findings,
		Artifacts:  artifacts,
	}
}

func severityRank(s Severity) int {
	switch s {
	case SeverityCritical:
		return 0
	case SeverityDanger:
		return 1
	case SeverityWarn:
		return 2
	default:
		return 3
	}
}

func Summarize(findings []Finding) map[string]int {
	summary := map[string]int{"critical": 0, "danger": 0, "warn": 0, "info": 0}
	for _, f := range findings {
		switch f.Severity {
		case SeverityCritical:
			summary["critical"]++
		case SeverityDanger:
			summary["danger"]++
		case SeverityWarn:
			summary["warn"]++
		default:
			summary["info"]++
		}
	}
	return summary
}

func WriteReport(report Report, stdout io.Writer, output string, asJSON bool) error {
	if asJSON || output != "" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		data = append(data, '\n')
		if output == "" {
			_, err = stdout.Write(data)
			return err
		}
		dir := filepath.Dir(output)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}
		}
		if err := os.WriteFile(output, data, 0644); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Отчёт сохранён: %s\n", output)
		return nil
	}
	fmt.Fprintf(stdout, "%s %s: профиль %s, цель %s\n", report.Tool, report.Version, report.Profile, report.Target)
	fmt.Fprintf(stdout, "Итог: critical=%d, danger=%d, warn=%d, info=%d\n", report.Summary["critical"], report.Summary["danger"], report.Summary["warn"], report.Summary["info"])
	for _, f := range report.Findings {
		fmt.Fprintf(stdout, "[%s] %s — %s", f.Severity, f.Title, f.Message)
		if strings.TrimSpace(f.Path) != "" {
			fmt.Fprintf(stdout, " (%s)", f.Path)
		}
		fmt.Fprintln(stdout)
	}
	return nil
}

// WriteJSONFileOrStdout пишет произвольный JSON-совместимый отчёт в stdout или файл.
func WriteJSONFileOrStdout(payload any, stdout io.Writer, output string) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if output == "" {
		_, err = stdout.Write(data)
		return err
	}
	dir := filepath.Dir(output)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(output, data, 0644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Отчёт сохранён: %s\n", output)
	return nil
}

// RecommendedExitCode возвращает стабильный exit code по итогам отчёта.
func RecommendedExitCode(summary map[string]int) int {
	if summary["critical"] > 0 {
		return 4
	}
	if summary["danger"] > 0 {
		return 3
	}
	if summary["warn"] > 0 {
		return 2
	}
	return 0
}
