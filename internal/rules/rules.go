package rules

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gitflic.ru/skif4er/doctortools/internal/model"
)

const CatalogVersion = "1.9.0"

type Rule struct {
	ID             string         `json:"id"`
	Severity       model.Severity `json:"severity"`
	Category       string         `json:"category"`
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	Recommendation string         `json:"recommendation,omitempty"`
}

type Catalog struct {
	Version string `json:"version"`
	Module  string `json:"module,omitempty"`
	Rules   []Rule `json:"rules"`
}

type CatalogIndex struct {
	Version    string   `json:"version"`
	Modules    []string `json:"modules"`
	RulesCount int      `json:"rules_count"`
}

type ValidationIssue struct {
	Severity string `json:"severity"`
	File     string `json:"file,omitempty"`
	RuleID   string `json:"rule_id,omitempty"`
	Message  string `json:"message"`
}

type ValidationReport struct {
	Version string            `json:"version"`
	Files   []string          `json:"files,omitempty"`
	Modules map[string]int    `json:"modules,omitempty"`
	Rules   int               `json:"rules"`
	Issues  []ValidationIssue `json:"issues,omitempty"`
	Valid   bool              `json:"valid"`
}

type IgnoreMatcher struct {
	File     string   `json:"file,omitempty"`
	Patterns []string `json:"patterns,omitempty"`
}

func LoadCatalog(extraFile string) (Catalog, error) {
	catalog, err := EmbeddedCatalog()
	if err != nil {
		return Catalog{}, err
	}
	if strings.TrimSpace(extraFile) == "" {
		return catalog, nil
	}
	extra, err := LoadCatalogFile(extraFile)
	if err != nil {
		return Catalog{}, err
	}
	catalog.Rules = append(catalog.Rules, extra.Rules...)
	catalog.Rules = deduplicateRules(catalog.Rules)
	return catalog, nil
}

func LoadCatalogFile(path string) (Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Catalog{}, fmt.Errorf("не удалось прочитать файл правил %s: %w", path, err)
	}
	catalog, err := parseCatalogJSON(data, path)
	if err != nil {
		return Catalog{}, err
	}
	if report := ValidateCatalog(catalog); !report.Valid {
		return Catalog{}, fmt.Errorf("файл правил %s не прошёл проверку: %s", path, firstValidationMessage(report.Issues))
	}
	return catalog, nil
}

func parseCatalogJSON(data []byte, source string) (Catalog, error) {
	var wrapped Catalog
	if err := json.Unmarshal(data, &wrapped); err == nil && len(wrapped.Rules) > 0 {
		if wrapped.Version == "" {
			wrapped.Version = "custom"
		}
		return wrapped, nil
	}
	var list []Rule
	if err := json.Unmarshal(data, &list); err != nil {
		return Catalog{}, fmt.Errorf("файл правил %s должен быть JSON-массивом правил или объектом с полем rules: %w", source, err)
	}
	return Catalog{Version: "custom", Rules: list}, nil
}

func (c Catalog) SortedRules() []Rule {
	out := append([]Rule(nil), c.Rules...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Category == out[j].Category {
			return out[i].ID < out[j].ID
		}
		return out[i].Category < out[j].Category
	})
	return out
}

func (c Catalog) Find(id string) (Rule, bool) {
	for _, rule := range c.Rules {
		if rule.ID == id {
			return rule, true
		}
	}
	for _, rule := range c.Rules {
		if matchPattern(rule.ID, id) {
			return rule, true
		}
	}
	return Rule{}, false
}

func (c Catalog) EnrichFindings(findings []model.Finding) []model.Finding {
	out := make([]model.Finding, 0, len(findings))
	for _, f := range findings {
		if rule, ok := c.Find(f.ID); ok {
			if f.Severity == "" {
				f.Severity = rule.Severity
			}
			if f.Category == "" {
				f.Category = rule.Category
			}
			if f.Title == "" {
				f.Title = rule.Title
			}
			if f.Message == "" {
				f.Message = rule.Description
			}
			if f.Recommendation == "" {
				f.Recommendation = rule.Recommendation
			}
		}
		out = append(out, f)
	}
	return out
}

func ValidateCatalog(c Catalog) ValidationReport {
	report := ValidationReport{Version: c.Version, Rules: len(c.Rules), Modules: map[string]int{}, Valid: true}
	if strings.TrimSpace(c.Version) == "" {
		report.addIssue("ERROR", "", "", "не указана версия каталога правил")
	}
	if len(c.Rules) == 0 {
		report.addIssue("ERROR", "", "", "каталог не содержит правил")
	}
	seen := map[string]struct{}{}
	validSeverity := map[model.Severity]bool{
		model.SeverityInfo:     true,
		model.SeverityWarn:     true,
		model.SeverityDanger:   true,
		model.SeverityCritical: true,
	}
	for _, rule := range c.Rules {
		id := strings.TrimSpace(rule.ID)
		if id == "" {
			report.addIssue("ERROR", "", "", "найдено правило без id")
			continue
		}
		if _, ok := seen[id]; ok {
			report.addIssue("ERROR", "", id, "дублирующийся id правила")
		}
		seen[id] = struct{}{}
		if !validSeverity[rule.Severity] {
			report.addIssue("ERROR", "", id, "некорректный уровень серьёзности")
		}
		if strings.TrimSpace(rule.Category) == "" {
			report.addIssue("ERROR", "", id, "не указана категория правила")
		}
		if strings.TrimSpace(rule.Title) == "" {
			report.addIssue("ERROR", "", id, "не указано название правила")
		}
		if strings.TrimSpace(rule.Description) == "" {
			report.addIssue("ERROR", "", id, "не указано описание правила")
		}
		module := strings.SplitN(id, ".", 2)[0]
		if module == "" {
			module = "unknown"
		}
		report.Modules[module]++
	}
	report.Valid = len(report.Issues) == 0
	return report
}

func ValidateCatalogPath(path string) (ValidationReport, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return ValidateEmbeddedCatalog()
	}
	info, err := os.Stat(path)
	if err != nil {
		return ValidationReport{}, fmt.Errorf("не удалось открыть путь каталога правил %s: %w", path, err)
	}
	if !info.IsDir() {
		catalog, err := loadCatalogFileWithoutValidation(path)
		if err != nil {
			return ValidationReport{}, err
		}
		report := ValidateCatalog(catalog)
		report.Files = []string{path}
		return report, nil
	}
	files, err := filepath.Glob(filepath.Join(path, "*.json"))
	if err != nil {
		return ValidationReport{}, err
	}
	sort.Strings(files)
	merged := Catalog{Version: CatalogVersion}
	report := ValidationReport{Version: CatalogVersion, Modules: map[string]int{}, Valid: true}
	for _, file := range files {
		if filepath.Base(file) == "catalog.json" {
			continue
		}
		catalog, err := loadCatalogFileWithoutValidation(file)
		if err != nil {
			report.addIssue("ERROR", file, "", err.Error())
			continue
		}
		report.Files = append(report.Files, file)
		merged.Rules = append(merged.Rules, catalog.Rules...)
	}
	validation := ValidateCatalog(merged)
	report.Rules = validation.Rules
	for module, count := range validation.Modules {
		report.Modules[module] = count
	}
	report.Issues = append(report.Issues, validation.Issues...)
	report.Valid = len(report.Issues) == 0
	return report, nil
}

func loadCatalogFileWithoutValidation(path string) (Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Catalog{}, fmt.Errorf("не удалось прочитать файл правил %s: %w", path, err)
	}
	return parseCatalogJSON(data, path)
}

func (r *ValidationReport) addIssue(severity, file, ruleID, message string) {
	r.Issues = append(r.Issues, ValidationIssue{Severity: severity, File: file, RuleID: ruleID, Message: message})
	r.Valid = false
}

func firstValidationMessage(issues []ValidationIssue) string {
	if len(issues) == 0 {
		return "неизвестная ошибка"
	}
	if issues[0].RuleID != "" {
		return issues[0].RuleID + ": " + issues[0].Message
	}
	return issues[0].Message
}

func LoadIgnore(root, explicitPath string, disabled bool) (IgnoreMatcher, error) {
	if disabled {
		return IgnoreMatcher{}, nil
	}
	path := strings.TrimSpace(explicitPath)
	if path == "" {
		path = filepath.Join(root, ".craftdoctorignore")
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return IgnoreMatcher{}, nil
		}
		return IgnoreMatcher{}, fmt.Errorf("не удалось прочитать файл игнорирования %s: %w", path, err)
	}
	defer file.Close()
	matcher := IgnoreMatcher{File: path}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		matcher.Patterns = append(matcher.Patterns, line)
	}
	if err := scanner.Err(); err != nil {
		return IgnoreMatcher{}, err
	}
	return matcher, nil
}

func (m IgnoreMatcher) Filter(findings []model.Finding) ([]model.Finding, []model.Finding) {
	if len(m.Patterns) == 0 {
		return findings, nil
	}
	kept := make([]model.Finding, 0, len(findings))
	ignored := make([]model.Finding, 0)
	for _, f := range findings {
		if m.Match(f) {
			ignored = append(ignored, f)
			continue
		}
		kept = append(kept, f)
	}
	return kept, ignored
}

func (m IgnoreMatcher) Match(f model.Finding) bool {
	for _, pattern := range m.Patterns {
		p := strings.TrimSpace(pattern)
		if p == "" || strings.HasPrefix(p, "#") {
			continue
		}
		lower := strings.ToLower(p)
		switch {
		case strings.HasPrefix(lower, "category:"):
			want := strings.TrimSpace(strings.TrimPrefix(p, "category:"))
			if strings.EqualFold(want, f.Category) {
				return true
			}
		case strings.HasPrefix(lower, "severity:"):
			want := strings.TrimSpace(strings.TrimPrefix(p, "severity:"))
			if strings.EqualFold(want, string(f.Severity)) {
				return true
			}
		case matchPattern(p, f.ID):
			return true
		}
	}
	return false
}

func matchPattern(pattern, value string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	if pattern == value || pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") && strings.HasPrefix(value, strings.TrimSuffix(pattern, "*")) {
		return true
	}
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(value, strings.TrimPrefix(pattern, "*")) {
		return true
	}
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		pos := 0
		for _, part := range parts {
			if part == "" {
				continue
			}
			idx := strings.Index(value[pos:], part)
			if idx < 0 {
				return false
			}
			pos += idx + len(part)
		}
		return true
	}
	return false
}

func deduplicateRules(rules []Rule) []Rule {
	seen := map[string]Rule{}
	for _, rule := range rules {
		if strings.TrimSpace(rule.ID) == "" {
			continue
		}
		seen[rule.ID] = rule
	}
	out := make([]Rule, 0, len(seen))
	for _, rule := range seen {
		out = append(out, rule)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func BuiltinCatalog() Catalog {
	catalog, err := EmbeddedCatalog()
	if err != nil {
		return Catalog{Version: CatalogVersion}
	}
	return catalog
}
