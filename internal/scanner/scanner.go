package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gitflic.ru/skif4er/doctortools/internal/analyzers/config"
	"gitflic.ru/skif4er/doctortools/internal/analyzers/craftadvanced"
	"gitflic.ru/skif4er/doctortools/internal/analyzers/java"
	"gitflic.ru/skif4er/doctortools/internal/analyzers/logs"
	"gitflic.ru/skif4er/doctortools/internal/analyzers/minecraft"
	"gitflic.ru/skif4er/doctortools/internal/analyzers/performance"
	"gitflic.ru/skif4er/doctortools/internal/analyzers/plugins"
	"gitflic.ru/skif4er/doctortools/internal/analyzers/production"
	"gitflic.ru/skif4er/doctortools/internal/analyzers/proxy"
	"gitflic.ru/skif4er/doctortools/internal/analyzers/security"
	"gitflic.ru/skif4er/doctortools/internal/analyzers/systeminfo"
	"gitflic.ru/skif4er/doctortools/internal/model"
	"gitflic.ru/skif4er/doctortools/internal/rules"
	"gitflic.ru/skif4er/doctortools/internal/version"
)

type Options struct {
	RulesFile              string
	IgnoreFile             string
	NoIgnore               bool
	PerformancePlayers     int
	PerformanceTarget      string
	ProductionSystemdDir   string
	ProductionLogrotateDir string
	ProductionBackupMaxAge string
}

func Scan(target string) (*model.Report, error) {
	return ScanWithOptions(target, Options{})
}

func ScanWithOptions(target string, opts Options) (*model.Report, error) {
	if target == "" {
		return nil, fmt.Errorf("не указан путь к Minecraft-серверу")
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть путь %s: %w", abs, err)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("путь %s не является директорией", abs)
	}

	catalog, err := rules.LoadCatalog(opts.RulesFile)
	if err != nil {
		return nil, err
	}
	ignoreMatcher, err := rules.LoadIgnore(abs, opts.IgnoreFile, opts.NoIgnore)
	if err != nil {
		return nil, err
	}

	rep := &model.Report{
		ToolVersion: version.Version,
		GeneratedAt: time.Now(),
		TargetPath:  abs,
		Rules: model.RulesInfo{
			CatalogVersion:     catalog.Version,
			BuiltinRules:       len(rules.BuiltinCatalog().Rules),
			CustomRulesFile:    opts.RulesFile,
			IgnoreFile:         ignoreMatcher.File,
			IgnorePatterns:     ignoreMatcher.Patterns,
			SuppressedFindings: 0,
		},
	}

	sysInfo, sysFindings := systeminfo.Analyze(abs)
	rep.System = sysInfo
	appendFindings(rep, sysFindings)

	javaInfo, javaFindings := java.Analyze()
	rep.Java = javaInfo
	appendFindings(rep, javaFindings)

	mcInfo, mcFindings := minecraft.Analyze(abs)
	rep.Minecraft = mcInfo
	appendFindings(rep, mcFindings)

	configInfo, configFindings := config.Analyze(abs, mcInfo)
	rep.Config = configInfo
	appendFindings(rep, configFindings)

	pluginList, pluginAudit, pluginFindings := plugins.AnalyzeDetailed(abs)
	rep.Plugins = pluginList
	rep.PluginAudit = pluginAudit
	appendFindings(rep, pluginFindings)

	logInfo, logFindings := logs.Analyze(abs)
	rep.Logs = logInfo
	appendFindings(rep, logFindings)

	perfInfo, perfFindings := performance.AnalyzeWithOptions(abs, sysInfo, javaInfo, mcInfo, pluginList, logInfo, model.PerformanceOptions{Players: opts.PerformancePlayers, Target: opts.PerformanceTarget})
	rep.Performance = perfInfo
	appendFindings(rep, perfFindings)

	proxyInfo, proxyFindings := proxy.Analyze(abs, mcInfo)
	rep.Proxy = proxyInfo
	appendFindings(rep, proxyFindings)

	securityInfo, securityFindings := security.Analyze(abs, sysInfo, mcInfo, pluginList)
	rep.Security = securityInfo
	appendFindings(rep, securityFindings)

	productionInfo, productionFindings := production.AnalyzeWithOptions(abs, sysInfo, mcInfo, model.ProductionOptions{SystemdDir: opts.ProductionSystemdDir, LogrotateDir: opts.ProductionLogrotateDir, BackupMaxAge: opts.ProductionBackupMaxAge})
	rep.Production = productionInfo
	appendFindings(rep, productionFindings)

	craftAdvancedInfo, craftAdvancedFindings := craftadvanced.Analyze(abs, mcInfo, perfInfo, proxyInfo, productionInfo)
	rep.CraftAdvanced = craftAdvancedInfo
	appendFindings(rep, craftAdvancedFindings)

	rep.Findings = catalog.EnrichFindings(rep.Findings)
	rep.Findings, rep.Suppressed = ignoreMatcher.Filter(rep.Findings)
	rep.Rules.SuppressedFindings = len(rep.Suppressed)

	rep.Finalize()
	return rep, nil
}

func appendFindings(rep *model.Report, findings []model.Finding) {
	for _, f := range findings {
		rep.AddFinding(f)
	}
}
