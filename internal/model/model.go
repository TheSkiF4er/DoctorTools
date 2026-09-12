package model

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type Severity string

const (
	SeverityInfo     Severity = "INFO"
	SeverityWarn     Severity = "WARN"
	SeverityDanger   Severity = "DANGER"
	SeverityCritical Severity = "CRITICAL"
)

func (s Severity) Rank() int {
	switch s {
	case SeverityCritical:
		return 4
	case SeverityDanger:
		return 3
	case SeverityWarn:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 0
	}
}

type Finding struct {
	ID             string   `json:"id"`
	Severity       Severity `json:"severity"`
	Category       string   `json:"category"`
	Title          string   `json:"title"`
	Message        string   `json:"message"`
	Recommendation string   `json:"recommendation,omitempty"`
	File           string   `json:"file,omitempty"`
}

type SystemInfo struct {
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	CPUs            int    `json:"cpus"`
	TotalMemoryMB   uint64 `json:"total_memory_mb,omitempty"`
	FreeDiskMB      uint64 `json:"free_disk_mb,omitempty"`
	TotalDiskMB     uint64 `json:"total_disk_mb,omitempty"`
	WritableRoot    bool   `json:"writable_root"`
	WritablePlugins bool   `json:"writable_plugins"`
}

type JavaInfo struct {
	Found      bool   `json:"found"`
	Version    string `json:"version,omitempty"`
	Major      int    `json:"major,omitempty"`
	Executable string `json:"executable,omitempty"`
	Raw        string `json:"raw,omitempty"`
}

type MinecraftInfo struct {
	CoreType       string            `json:"core_type"`
	CoreJar        string            `json:"core_jar,omitempty"`
	ServerVersion  string            `json:"server_version,omitempty"`
	EULAAccepted   bool              `json:"eula_accepted"`
	Properties     map[string]string `json:"properties,omitempty"`
	ConfigFiles    []string          `json:"config_files,omitempty"`
	Worlds         []string          `json:"worlds,omitempty"`
	PluginsDir     string            `json:"plugins_dir,omitempty"`
	LogsDir        string            `json:"logs_dir,omitempty"`
	CrashReports   int               `json:"crash_reports"`
	ProxyForwarded bool              `json:"proxy_forwarded"`
}

type PluginInfo struct {
	Name              string   `json:"name"`
	Version           string   `json:"version,omitempty"`
	Main              string   `json:"main,omitempty"`
	APIVersion        string   `json:"api_version,omitempty"`
	FoliaSupported    *bool    `json:"folia_supported,omitempty"`
	Bootstrapper      string   `json:"bootstrapper,omitempty"`
	Loader            string   `json:"loader,omitempty"`
	JarFile           string   `json:"jar_file"`
	Metadata          string   `json:"metadata,omitempty"`
	Authors           []string `json:"authors,omitempty"`
	Description       string   `json:"description,omitempty"`
	Website           string   `json:"website,omitempty"`
	Depends           []string `json:"depends,omitempty"`
	SoftDepends       []string `json:"soft_depends,omitempty"`
	LoadBefore        []string `json:"load_before,omitempty"`
	Provides          []string `json:"provides,omitempty"`
	Libraries         []string `json:"libraries,omitempty"`
	PaperDependencies []string `json:"paper_dependencies,omitempty"`
	Commands          []string `json:"commands,omitempty"`
	Permissions       []string `json:"permissions,omitempty"`
	Categories        []string `json:"categories,omitempty"`
	ClassCount        int      `json:"class_count,omitempty"`
	MainClassPresent  bool     `json:"main_class_present,omitempty"`
	PackageRoots      []string `json:"package_roots,omitempty"`
	ShadedLibraries   []string `json:"shaded_libraries,omitempty"`
	DuplicateClasses  []string `json:"duplicate_classes,omitempty"`
	Valid             bool     `json:"valid"`
	Problem           string   `json:"problem,omitempty"`
}

type PluginDependencyEdge struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Kind    string `json:"kind"`
	Present bool   `json:"present"`
	File    string `json:"file,omitempty"`
}

type PluginCycle struct {
	Plugins []string `json:"plugins"`
}

type PluginCategoryInfo struct {
	Category string   `json:"category"`
	Plugins  []string `json:"plugins"`
}

type PluginDuplicateClassInfo struct {
	Class   string   `json:"class"`
	Plugins []string `json:"plugins"`
}

type PluginConflictInfo struct {
	Kind           string   `json:"kind"`
	Plugins        []string `json:"plugins"`
	Reason         string   `json:"reason"`
	Recommendation string   `json:"recommendation,omitempty"`
}

type PluginGraphInfo struct {
	Nodes []string               `json:"nodes,omitempty"`
	Edges []PluginDependencyEdge `json:"edges,omitempty"`
}

type PluginAuditInfo struct {
	Total               int                        `json:"total"`
	Valid               int                        `json:"valid"`
	Invalid             int                        `json:"invalid"`
	BrokenJars          []string                   `json:"broken_jars,omitempty"`
	DuplicateNames      map[string][]string        `json:"duplicate_names,omitempty"`
	Categories          []PluginCategoryInfo       `json:"categories,omitempty"`
	DependencyEdges     []PluginDependencyEdge     `json:"dependency_edges,omitempty"`
	MissingDependencies []PluginDependencyEdge     `json:"missing_dependencies,omitempty"`
	Cycles              []PluginCycle              `json:"cycles,omitempty"`
	DuplicateClasses    []PluginDuplicateClassInfo `json:"duplicate_classes,omitempty"`
	Conflicts           []PluginConflictInfo       `json:"conflicts,omitempty"`
	Graph               PluginGraphInfo            `json:"graph,omitempty"`
}

type LogIssue struct {
	Type           string   `json:"type"`
	Severity       Severity `json:"severity"`
	Component      string   `json:"component,omitempty"`
	Line           int      `json:"line,omitempty"`
	Count          int      `json:"count"`
	Message        string   `json:"message"`
	Evidence       string   `json:"evidence,omitempty"`
	Recommendation string   `json:"recommendation,omitempty"`
}

type LogRootCause struct {
	ID             string   `json:"id"`
	Severity       Severity `json:"severity"`
	Component      string   `json:"component,omitempty"`
	Title          string   `json:"title"`
	Message        string   `json:"message"`
	Recommendation string   `json:"recommendation,omitempty"`
	Count          int      `json:"count"`
}

type LogComponentSummary struct {
	Component string `json:"component"`
	Critical  int    `json:"critical"`
	Errors    int    `json:"errors"`
	Warnings  int    `json:"warnings"`
}

// LogSession описывает обнаруженную сессию запуска сервера внутри лог-файла.
type LogSession struct {
	Index     int    `json:"index"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	StartTime string `json:"start_time,omitempty"`
	EndTime   string `json:"end_time,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// LogStackTraceGroup группирует одинаковые stack trace по fingerprint.
type LogStackTraceGroup struct {
	Fingerprint string   `json:"fingerprint"`
	Severity    Severity `json:"severity"`
	Component   string   `json:"component,omitempty"`
	Exception   string   `json:"exception,omitempty"`
	FirstLine   int      `json:"first_line,omitempty"`
	LastLine    int      `json:"last_line,omitempty"`
	Count       int      `json:"count"`
	Message     string   `json:"message,omitempty"`
	Cause       string   `json:"cause,omitempty"`
	Sample      []string `json:"sample,omitempty"`
}

// LogIssueTypeSummary показывает частотность типов проблем в логах.
type LogIssueTypeSummary struct {
	Type     string   `json:"type"`
	Severity Severity `json:"severity"`
	Count    int      `json:"count"`
}

// LogAnalysisOptions фиксирует режим анализа логов, выбранный пользователем.
type LogAnalysisOptions struct {
	LastRun bool   `json:"last_run"`
	Since   string `json:"since,omitempty"`
}

type LogInfo struct {
	LatestLogFound  bool                  `json:"latest_log_found"`
	AnalyzedFile    string                `json:"analyzed_file,omitempty"`
	LinesAnalyzed   int                   `json:"lines_analyzed,omitempty"`
	WarnCount       int                   `json:"warn_count"`
	ErrorCount      int                   `json:"error_count"`
	ExceptionCount  int                   `json:"exception_count"`
	LongTickCount   int                   `json:"long_tick_count"`
	Samples         []string              `json:"samples,omitempty"`
	Issues          []LogIssue            `json:"issues,omitempty"`
	RootCauses      []LogRootCause        `json:"root_causes,omitempty"`
	Components      []LogComponentSummary `json:"components,omitempty"`
	Sessions        []LogSession          `json:"sessions,omitempty"`
	SelectedSession *LogSession           `json:"selected_session,omitempty"`
	StackTraces     []LogStackTraceGroup  `json:"stack_traces,omitempty"`
	IssueTypes      []LogIssueTypeSummary `json:"issue_types,omitempty"`
	MainRootCause   *LogRootCause         `json:"main_root_cause,omitempty"`
	Options         LogAnalysisOptions    `json:"options,omitempty"`
}

type ProxyServer struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Host    string `json:"host,omitempty"`
	Port    string `json:"port,omitempty"`
}

type ProxyForcedHost struct {
	Host    string   `json:"host"`
	Servers []string `json:"servers"`
}

type ProxyConfig struct {
	Type           string            `json:"type"`
	File           string            `json:"file"`
	Bind           string            `json:"bind,omitempty"`
	ForwardingMode string            `json:"forwarding_mode,omitempty"`
	SecretFile     string            `json:"secret_file,omitempty"`
	SecretPresent  bool              `json:"secret_present"`
	OnlineMode     string            `json:"online_mode,omitempty"`
	IPForward      string            `json:"ip_forward,omitempty"`
	Servers        []ProxyServer     `json:"servers,omitempty"`
	ForcedHosts    []ProxyForcedHost `json:"forced_hosts,omitempty"`
	RawSettings    map[string]string `json:"raw_settings,omitempty"`
}

type ProxyNode struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Address string `json:"address,omitempty"`
	File    string `json:"file,omitempty"`
}

type ProxyEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

type ProxyCheck struct {
	ID             string   `json:"id"`
	Severity       Severity `json:"severity"`
	Area           string   `json:"area"`
	Target         string   `json:"target,omitempty"`
	Status         string   `json:"status"`
	Message        string   `json:"message"`
	Recommendation string   `json:"recommendation,omitempty"`
	File           string   `json:"file,omitempty"`
}

type ProxyInfo struct {
	Status          Severity      `json:"status"`
	Detected        bool          `json:"detected"`
	Types           []string      `json:"types,omitempty"`
	ConfigFiles     []string      `json:"config_files,omitempty"`
	Configs         []ProxyConfig `json:"configs,omitempty"`
	Nodes           []ProxyNode   `json:"nodes,omitempty"`
	Edges           []ProxyEdge   `json:"edges,omitempty"`
	Checks          []ProxyCheck  `json:"checks,omitempty"`
	Recommendations []string      `json:"recommendations,omitempty"`
}

type PerformanceMetric struct {
	Key            string   `json:"key"`
	Value          string   `json:"value"`
	Source         string   `json:"source,omitempty"`
	Status         Severity `json:"status"`
	Recommendation string   `json:"recommendation,omitempty"`
}

type PerformanceRisk struct {
	ID             string   `json:"id"`
	Severity       Severity `json:"severity"`
	Area           string   `json:"area"`
	Message        string   `json:"message"`
	Recommendation string   `json:"recommendation,omitempty"`
}

// PerformanceOptions фиксирует пользовательские параметры Performance Doctor.
type PerformanceOptions struct {
	Players int    `json:"players,omitempty"`
	Target  string `json:"target,omitempty"`
}

// PerformanceProfile описывает импортированный spark/timings/performance-отчёт.
type PerformanceProfile struct {
	Source  string   `json:"source"`
	Kind    string   `json:"kind"`
	TPS     float64  `json:"tps,omitempty"`
	MSPTAvg float64  `json:"mspt_avg,omitempty"`
	MSPTP95 float64  `json:"mspt_p95,omitempty"`
	MSPTMax float64  `json:"mspt_max,omitempty"`
	Samples int      `json:"samples,omitempty"`
	Notes   []string `json:"notes,omitempty"`
}

// PerformanceCapacity содержит ориентиры под указанный онлайн и тип сервера.
type PerformanceCapacity struct {
	Players                       int      `json:"players,omitempty"`
	Target                        string   `json:"target,omitempty"`
	RecommendedHeapMB             int      `json:"recommended_heap_mb,omitempty"`
	RecommendedViewDistance       int      `json:"recommended_view_distance,omitempty"`
	RecommendedSimulationDistance int      `json:"recommended_simulation_distance,omitempty"`
	Notes                         []string `json:"notes,omitempty"`
}

type PerformanceInfo struct {
	Status          Severity             `json:"status"`
	Options         PerformanceOptions   `json:"options,omitempty"`
	Capacity        PerformanceCapacity  `json:"capacity,omitempty"`
	HeapMinMB       int                  `json:"heap_min_mb,omitempty"`
	HeapMaxMB       int                  `json:"heap_max_mb,omitempty"`
	JVMFlags        []string             `json:"jvm_flags,omitempty"`
	JVMFlagSources  []string             `json:"jvm_flag_sources,omitempty"`
	HasSpark        bool                 `json:"has_spark"`
	SparkReports    []string             `json:"spark_reports,omitempty"`
	Profiles        []PerformanceProfile `json:"profiles,omitempty"`
	Metrics         []PerformanceMetric  `json:"metrics,omitempty"`
	Risks           []PerformanceRisk    `json:"risks,omitempty"`
	Recommendations []string             `json:"recommendations,omitempty"`
}

type SecurityCheck struct {
	ID             string   `json:"id"`
	Severity       Severity `json:"severity"`
	Area           string   `json:"area"`
	Target         string   `json:"target,omitempty"`
	Status         string   `json:"status"`
	Message        string   `json:"message"`
	Recommendation string   `json:"recommendation,omitempty"`
	File           string   `json:"file,omitempty"`
}

type SecuritySecret struct {
	Kind           string   `json:"kind"`
	Severity       Severity `json:"severity"`
	File           string   `json:"file"`
	Line           int      `json:"line,omitempty"`
	Evidence       string   `json:"evidence,omitempty"`
	Recommendation string   `json:"recommendation,omitempty"`
}

// SecuritySensitiveFile описывает файл, который содержит чувствительные настройки
// и должен иметь строгие права доступа.
type SecuritySensitiveFile struct {
	Path           string `json:"path"`
	Kind           string `json:"kind"`
	Mode           string `json:"mode,omitempty"`
	Recommendation string `json:"recommendation,omitempty"`
}

// SecurityPluginSignal фиксирует найденный плагин/модуль, влияющий на безопасность.
type SecurityPluginSignal struct {
	Name           string   `json:"name"`
	Kind           string   `json:"kind"`
	Severity       Severity `json:"severity"`
	Recommendation string   `json:"recommendation,omitempty"`
}

type SecurityInfo struct {
	Status                Severity                `json:"status"`
	Score                 int                     `json:"score"`
	Checks                []SecurityCheck         `json:"checks,omitempty"`
	Secrets               []SecuritySecret        `json:"secrets,omitempty"`
	SecretsIgnoreFile     string                  `json:"secrets_ignore_file,omitempty"`
	SecretsIgnorePatterns []string                `json:"secrets_ignore_patterns,omitempty"`
	SecretFilesScanned    int                     `json:"secret_files_scanned,omitempty"`
	SensitiveFiles        []SecuritySensitiveFile `json:"sensitive_files,omitempty"`
	PluginSignals         []SecurityPluginSignal  `json:"plugin_signals,omitempty"`
	Recommendations       []string                `json:"recommendations,omitempty"`
}

type ProductionCheck struct {
	ID             string   `json:"id"`
	Severity       Severity `json:"severity"`
	Area           string   `json:"area"`
	Target         string   `json:"target,omitempty"`
	Status         string   `json:"status"`
	Message        string   `json:"message"`
	Recommendation string   `json:"recommendation,omitempty"`
	File           string   `json:"file,omitempty"`
}

type BackupCandidate struct {
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	Modified  string `json:"modified,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
}

// ProductionOptions фиксирует параметры Production Doctor 2.0.
type ProductionOptions struct {
	SystemdDir   string `json:"systemd_dir,omitempty"`
	LogrotateDir string `json:"logrotate_dir,omitempty"`
	BackupMaxAge string `json:"backup_max_age,omitempty"`
}

// DockerInfo описывает найденную Docker/docker-compose структуру.
type DockerInfo struct {
	Detected         bool     `json:"detected"`
	ComposeFiles     []string `json:"compose_files,omitempty"`
	Dockerfiles      []string `json:"dockerfiles,omitempty"`
	HasVolumeHints   bool     `json:"has_volume_hints,omitempty"`
	HasRestartPolicy bool     `json:"has_restart_policy,omitempty"`
}

// StartupCommand описывает извлечённую команду запуска из скрипта или service-файла.
type StartupCommand struct {
	Source  string `json:"source"`
	Command string `json:"command"`
}

// ReleaseLayoutInfo описывает current/previous/releases структуру проекта.
type ReleaseLayoutInfo struct {
	Current  string   `json:"current,omitempty"`
	Previous string   `json:"previous,omitempty"`
	Releases []string `json:"releases,omitempty"`
}

type ProductionInfo struct {
	Status             Severity          `json:"status"`
	Options            ProductionOptions `json:"options,omitempty"`
	Checks             []ProductionCheck `json:"checks,omitempty"`
	BackupCandidates   []BackupCandidate `json:"backup_candidates,omitempty"`
	ServiceFiles       []string          `json:"service_files,omitempty"`
	LogrotateFiles     []string          `json:"logrotate_files,omitempty"`
	StartupScripts     []string          `json:"startup_scripts,omitempty"`
	StartupCommands    []StartupCommand  `json:"startup_commands,omitempty"`
	Docker             DockerInfo        `json:"docker"`
	RestoreChecklist   []string          `json:"restore_checklist,omitempty"`
	ReleaseLayout      ReleaseLayoutInfo `json:"release_layout,omitempty"`
	StagingCandidates  []string          `json:"staging_candidates,omitempty"`
	RollbackCandidates []string          `json:"rollback_candidates,omitempty"`
	Recommendations    []string          `json:"recommendations,omitempty"`
}

type ConfigFileInfo struct {
	Path           string   `json:"path"`
	Kind           string   `json:"kind"`
	Valid          bool     `json:"valid"`
	KeyCount       int      `json:"key_count"`
	Keys           []string `json:"keys,omitempty"`
	DuplicateKeys  []string `json:"duplicate_keys,omitempty"`
	UnknownKeys    []string `json:"unknown_keys,omitempty"`
	DeprecatedKeys []string `json:"deprecated_keys,omitempty"`
	Error          string   `json:"error,omitempty"`
}

type ConfigCheck struct {
	ID             string   `json:"id"`
	Severity       Severity `json:"severity"`
	Area           string   `json:"area"`
	Target         string   `json:"target,omitempty"`
	Status         string   `json:"status"`
	Message        string   `json:"message"`
	Recommendation string   `json:"recommendation,omitempty"`
	File           string   `json:"file,omitempty"`
}

type ConfigInfo struct {
	Status          Severity         `json:"status"`
	Files           []ConfigFileInfo `json:"files,omitempty"`
	Checks          []ConfigCheck    `json:"checks,omitempty"`
	Recommendations []string         `json:"recommendations,omitempty"`
}

// CraftWorldInfo описывает проверку мира Minecraft в расширенном CraftDoctor-аудите.
type CraftWorldInfo struct {
	Name            string `json:"name"`
	Path            string `json:"path"`
	Kind            string `json:"kind"`
	Exists          bool   `json:"exists"`
	HasLevelDat     bool   `json:"has_level_dat"`
	HasSessionLock  bool   `json:"has_session_lock"`
	HasUIDDat       bool   `json:"has_uid_dat"`
	RegionFiles     int    `json:"region_files,omitempty"`
	RegionSizeMB    int64  `json:"region_size_mb,omitempty"`
	EntitiesSizeMB  int64  `json:"entities_size_mb,omitempty"`
	PlayerDataFiles int    `json:"playerdata_files,omitempty"`
	DataPacks       int    `json:"datapacks,omitempty"`
	LastModified    string `json:"last_modified,omitempty"`
}

// CraftJavaFlagCheck фиксирует результат ревизии JVM-флагов Minecraft-сервера.
type CraftJavaFlagCheck struct {
	ID             string   `json:"id"`
	Severity       Severity `json:"severity"`
	Status         string   `json:"status"`
	Message        string   `json:"message"`
	Recommendation string   `json:"recommendation,omitempty"`
}

// CraftGateCheck описывает production-gate проверку CraftDoctor.
type CraftGateCheck struct {
	ID             string   `json:"id"`
	Severity       Severity `json:"severity"`
	Area           string   `json:"area"`
	Status         string   `json:"status"`
	Message        string   `json:"message"`
	Recommendation string   `json:"recommendation,omitempty"`
}

// CraftAdvancedInfo хранит расширенный Minecraft/server аудит CraftDoctor 3.0.1.
type CraftAdvancedInfo struct {
	Status          Severity             `json:"status"`
	Worlds          []CraftWorldInfo     `json:"worlds,omitempty"`
	JavaFlagChecks  []CraftJavaFlagCheck `json:"java_flag_checks,omitempty"`
	GateChecks      []CraftGateCheck     `json:"gate_checks,omitempty"`
	Recommendations []string             `json:"recommendations,omitempty"`
}

type Summary struct {
	Info     int `json:"info"`
	Warn     int `json:"warn"`
	Danger   int `json:"danger"`
	Critical int `json:"critical"`
}

type RulesInfo struct {
	CatalogVersion     string   `json:"catalog_version,omitempty"`
	BuiltinRules       int      `json:"builtin_rules"`
	CustomRulesFile    string   `json:"custom_rules_file,omitempty"`
	IgnoreFile         string   `json:"ignore_file,omitempty"`
	IgnorePatterns     []string `json:"ignore_patterns,omitempty"`
	SuppressedFindings int      `json:"suppressed_findings"`
}

type Report struct {
	ToolVersion   string            `json:"tool_version"`
	GeneratedAt   time.Time         `json:"generated_at"`
	TargetPath    string            `json:"target_path"`
	Status        Severity          `json:"status"`
	Summary       Summary           `json:"summary"`
	System        SystemInfo        `json:"system"`
	Java          JavaInfo          `json:"java"`
	Minecraft     MinecraftInfo     `json:"minecraft"`
	Config        ConfigInfo        `json:"config"`
	Plugins       []PluginInfo      `json:"plugins"`
	PluginAudit   PluginAuditInfo   `json:"plugin_audit"`
	Logs          LogInfo           `json:"logs"`
	Performance   PerformanceInfo   `json:"performance"`
	Proxy         ProxyInfo         `json:"proxy"`
	Security      SecurityInfo      `json:"security"`
	Production    ProductionInfo    `json:"production"`
	CraftAdvanced CraftAdvancedInfo `json:"craft_advanced"`
	Rules         RulesInfo         `json:"rules"`
	Findings      []Finding         `json:"findings"`
	Suppressed    []Finding         `json:"suppressed_findings,omitempty"`
}

func (r *Report) AddFinding(f Finding) {
	if f.ID == "" {
		f.ID = fmt.Sprintf("finding.%d", len(r.Findings)+1)
	}
	r.Findings = append(r.Findings, f)
}

func (r *Report) Finalize() {
	summary := Summary{}
	status := SeverityInfo
	for _, f := range r.Findings {
		switch f.Severity {
		case SeverityCritical:
			summary.Critical++
		case SeverityDanger:
			summary.Danger++
		case SeverityWarn:
			summary.Warn++
		default:
			summary.Info++
		}
		if f.Severity.Rank() > status.Rank() {
			status = f.Severity
		}
	}
	if len(r.Findings) == 0 {
		status = SeverityInfo
	}
	sort.SliceStable(r.Findings, func(i, j int) bool {
		if r.Findings[i].Severity.Rank() == r.Findings[j].Severity.Rank() {
			return strings.Compare(r.Findings[i].ID, r.Findings[j].ID) < 0
		}
		return r.Findings[i].Severity.Rank() > r.Findings[j].Severity.Rank()
	})
	r.Summary = summary
	r.Status = status
}
