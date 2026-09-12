package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"gitflic.ru/skif4er/doctortools/internal/model"
)

func Render(rep *model.Report, format string) ([]byte, string, error) {
	switch strings.ToLower(format) {
	case "json":
		b, err := json.MarshalIndent(rep, "", "  ")
		return b, "json", err
	case "markdown", "md":
		return []byte(Markdown(rep)), "md", nil
	case "html", "":
		return []byte(HTML(rep)), "html", nil
	default:
		return nil, "", fmt.Errorf("неподдерживаемый формат отчёта: %s", format)
	}
}

func Markdown(rep *model.Report) string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "# Отчёт CraftDoctor\n\n")
	fmt.Fprintf(&b, "**Версия инструмента:** %s  \n", rep.ToolVersion)
	fmt.Fprintf(&b, "**Дата:** %s  \n", rep.GeneratedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "**Путь:** `%s`  \n", rep.TargetPath)
	fmt.Fprintf(&b, "**Статус:** `%s`  \n\n", rep.Status)

	fmt.Fprintf(&b, "## Сводка\n\n")
	fmt.Fprintf(&b, "| Уровень | Количество |\n|---|---:|\n")
	fmt.Fprintf(&b, "| CRITICAL | %d |\n", rep.Summary.Critical)
	fmt.Fprintf(&b, "| DANGER | %d |\n", rep.Summary.Danger)
	fmt.Fprintf(&b, "| WARN | %d |\n", rep.Summary.Warn)
	fmt.Fprintf(&b, "| INFO | %d |\n\n", rep.Summary.Info)

	writeMarkdownRules(&b, rep)
	writeMarkdownPriority(&b, rep)

	fmt.Fprintf(&b, "## Серверная машина\n\n")
	fmt.Fprintf(&b, "- ОС: `%s`\n", rep.System.OS)
	fmt.Fprintf(&b, "- Архитектура: `%s`\n", rep.System.Arch)
	fmt.Fprintf(&b, "- CPU-потоки: `%d`\n", rep.System.CPUs)
	fmt.Fprintf(&b, "- RAM: `%d МБ`\n", rep.System.TotalMemoryMB)
	fmt.Fprintf(&b, "- Диск: свободно `%d МБ` из `%d МБ`\n\n", rep.System.FreeDiskMB, rep.System.TotalDiskMB)

	fmt.Fprintf(&b, "## Java\n\n")
	if rep.Java.Found {
		fmt.Fprintf(&b, "- Java: `%s`\n", rep.Java.Version)
		fmt.Fprintf(&b, "- Major: `%d`\n", rep.Java.Major)
		fmt.Fprintf(&b, "- Исполняемый файл: `%s`\n\n", rep.Java.Executable)
	} else {
		fmt.Fprintf(&b, "- Java не найдена.\n\n")
	}

	fmt.Fprintf(&b, "## Minecraft\n\n")
	fmt.Fprintf(&b, "- Ядро: `%s`\n", rep.Minecraft.CoreType)
	fmt.Fprintf(&b, "- Jar: `%s`\n", rep.Minecraft.CoreJar)
	fmt.Fprintf(&b, "- EULA принята: `%t`\n", rep.Minecraft.EULAAccepted)
	fmt.Fprintf(&b, "- Proxy forwarding признаки: `%t`\n", rep.Minecraft.ProxyForwarded)
	fmt.Fprintf(&b, "- Миры: `%s`\n", strings.Join(rep.Minecraft.Worlds, ", "))
	fmt.Fprintf(&b, "- Crash reports: `%d`\n\n", rep.Minecraft.CrashReports)

	writeMarkdownConfigDoctor(&b, rep)

	fmt.Fprintf(&b, "## Плагины\n\n")
	if len(rep.Plugins) == 0 {
		fmt.Fprintf(&b, "Плагины не найдены.\n\n")
	} else {
		fmt.Fprintf(&b, "| Название | Версия | Jar | Категории | Зависимости | Softdepend |\n|---|---|---|---|---|---|\n")
		for _, p := range rep.Plugins {
			fmt.Fprintf(&b, "| %s | %s | `%s` | %s | %s | %s |\n", safeMD(p.Name), safeMD(p.Version), p.JarFile, safeMD(strings.Join(p.Categories, ", ")), safeMD(strings.Join(p.Depends, ", ")), safeMD(strings.Join(p.SoftDepends, ", ")))
		}
		fmt.Fprintf(&b, "\n")
	}
	writeMarkdownPluginAudit(&b, rep)

	fmt.Fprintf(&b, "## Логи\n\n")
	fmt.Fprintf(&b, "- latest.log найден: `%t`\n", rep.Logs.LatestLogFound)
	fmt.Fprintf(&b, "- WARN: `%d`\n", rep.Logs.WarnCount)
	fmt.Fprintf(&b, "- ERROR: `%d`\n", rep.Logs.ErrorCount)
	fmt.Fprintf(&b, "- Exception/Caused by: `%d`\n", rep.Logs.ExceptionCount)
	fmt.Fprintf(&b, "- Long tick: `%d`\n", rep.Logs.LongTickCount)
	fmt.Fprintf(&b, "- Сессий запуска: `%d`\n", len(rep.Logs.Sessions))
	fmt.Fprintf(&b, "- Stack trace fingerprint: `%d`\n\n", len(rep.Logs.StackTraces))
	if len(rep.Logs.Samples) > 0 {
		fmt.Fprintf(&b, "### Примеры строк из логов\n\n")
		for _, s := range rep.Logs.Samples {
			fmt.Fprintf(&b, "```text\n%s\n```\n", s)
		}
	}
	writeMarkdownLogDoctor(&b, rep)
	writeMarkdownPerformanceDoctor(&b, rep)
	writeMarkdownProxyDoctor(&b, rep)
	writeMarkdownSecurityDoctor(&b, rep)
	writeMarkdownProductionDoctor(&b, rep)

	fmt.Fprintf(&b, "## Найденные проблемы и рекомендации\n\n")
	for _, f := range rep.Findings {
		fmt.Fprintf(&b, "### [%s] %s\n\n", f.Severity, f.Title)
		fmt.Fprintf(&b, "- ID: `%s`\n", f.ID)
		fmt.Fprintf(&b, "- Категория: `%s`\n", f.Category)
		if f.File != "" {
			fmt.Fprintf(&b, "- Файл: `%s`\n", f.File)
		}
		fmt.Fprintf(&b, "- Описание: %s\n", f.Message)
		if f.Recommendation != "" {
			fmt.Fprintf(&b, "- Рекомендация: %s\n", f.Recommendation)
		}
		fmt.Fprintf(&b, "\n")
	}
	return b.String()
}

func HTML(rep *model.Report) string {
	var b bytes.Buffer
	score := reportScore(rep)
	statusClass := severityClass(rep.Status)
	fmt.Fprintf(&b, "<!doctype html><html lang=\"ru\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>Отчёт CraftDoctor</title>")
	b.WriteString(`<style>
:root{--bg:#0b1020;--panel:#111827;--panel2:#172033;--border:#2b3447;--text:#e5e7eb;--muted:#9ca3af;--info:#60a5fa;--warn:#facc15;--danger:#f59e0b;--critical:#ef4444;--ok:#34d399}*{box-sizing:border-box}html{scroll-behavior:smooth}body{font-family:Inter,Arial,sans-serif;margin:0;background:radial-gradient(circle at top left,#172033 0,#0b1020 34%,#080c16 100%);color:var(--text);line-height:1.55}.wrap{max-width:1280px;margin:0 auto;padding:28px}.hero{display:grid;grid-template-columns:minmax(0,1.7fr) 310px;gap:18px;align-items:stretch}.card{background:linear-gradient(180deg,rgba(31,41,55,.96),rgba(17,24,39,.96));border:1px solid var(--border);border-radius:18px;padding:20px;margin:16px 0;box-shadow:0 14px 40px rgba(0,0,0,.22)}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(210px,1fr));gap:14px}.kpi{background:#0d1425;border:1px solid var(--border);border-radius:14px;padding:14px}.kpi strong{font-size:22px}.muted{color:var(--muted)}.badge{display:inline-block;border-radius:999px;padding:4px 10px;font-weight:800;font-size:12px;letter-spacing:.02em}.critical{background:#7f1d1d;color:#fecaca}.danger{background:#78350f;color:#fde68a}.warn{background:#713f12;color:#fef3c7}.info{background:#1e3a8a;color:#bfdbfe}.ok{background:#064e3b;color:#bbf7d0}.score{display:flex;align-items:center;justify-content:center;min-height:210px}.score-ring{width:172px;height:172px;border-radius:50%;display:grid;place-items:center;background:conic-gradient(var(--ok) calc(var(--score)*1%),#263247 0);box-shadow:inset 0 0 0 16px #0d1425}.score-ring span{font-size:42px;font-weight:900}.nav{position:sticky;top:0;z-index:10;background:rgba(8,12,22,.92);backdrop-filter:blur(10px);border-bottom:1px solid var(--border);padding:10px 0;margin:0 0 16px}.nav-inner{max-width:1280px;margin:0 auto;padding:0 28px;display:flex;flex-wrap:wrap;gap:8px}.nav a,.filter{border:1px solid var(--border);background:#111827;color:var(--text);text-decoration:none;border-radius:999px;padding:7px 11px;font-size:13px;cursor:pointer}.filter.active{outline:2px solid #93c5fd}table{width:100%;border-collapse:collapse}td,th{border-bottom:1px solid var(--border);padding:10px;text-align:left;vertical-align:top}th{color:#cbd5e1;background:#0d1425;position:sticky;top:46px}code,pre{background:#080d19;border:1px solid var(--border);border-radius:8px;padding:2px 5px;color:#d1d5db}pre{padding:12px;overflow:auto}.finding{border-left:5px solid #6b7280}.finding.critical{border-left-color:var(--critical);background:#1f2937}.finding.danger{border-left-color:var(--danger);background:#1f2937}.finding.warn{border-left-color:var(--warn);background:#1f2937}.finding.info{border-left-color:var(--info);background:#1f2937}a{color:#93c5fd}.module{display:flex;justify-content:space-between;gap:12px;align-items:flex-start}.module strong{font-size:18px}.copycmd{display:flex;gap:10px;align-items:center;justify-content:space-between;overflow:auto}.copycmd code{white-space:nowrap}.small{font-size:13px}.hidden{display:none!important}.toolbar{display:flex;flex-wrap:wrap;gap:8px;align-items:center}.search{min-width:260px;flex:1;border:1px solid var(--border);background:#0d1425;color:var(--text);border-radius:999px;padding:8px 12px}.print-btn{border:1px solid #3b82f6;background:#1e3a8a;color:#dbeafe;border-radius:999px;padding:7px 11px;cursor:pointer}.riskbar{display:grid;grid-template-columns:repeat(4,1fr);gap:8px}.riskbar .kpi{min-height:92px}.penalty{font-size:13px;color:#fca5a5}.section-title{display:flex;justify-content:space-between;gap:12px;align-items:center}.table-wrap{overflow:auto;border:1px solid var(--border);border-radius:14px}.table-wrap table{min-width:760px}details{border:1px solid var(--border);border-radius:14px;padding:12px;background:#0d1425;margin:10px 0}summary{cursor:pointer;font-weight:800}@media(max-width:760px){.wrap{padding:18px}.hero{grid-template-columns:1fr}.nav-inner{padding:0 18px}.score{min-height:150px}.score-ring{width:132px;height:132px}.score-ring span{font-size:32px}th{position:static}.search{min-width:100%}}@media print{body{background:#fff;color:#111}.nav,.filter,.print-btn,.search,script{display:none!important}.wrap{max-width:none;padding:0}.card,.kpi,details{box-shadow:none;border-color:#bbb;background:#fff;color:#111;break-inside:avoid}.muted{color:#555}a{color:#111}.badge{border:1px solid #777}.critical,.danger,.warn,.info,.ok{background:#fff;color:#111}.score-ring{background:#fff;border:8px solid #111}.finding{background:#fff!important}code,pre{background:#f6f6f6;color:#111;border-color:#ccc}}
</style>`)
	fmt.Fprintf(&b, "</head><body><nav class=\"nav\"><div class=\"nav-inner toolbar\"><a href=\"#summary\">Сводка</a><a href=\"#priority\">Что исправить первым</a><a href=\"#modules\">Doctor-модули</a><a href=\"#risk\">Риски</a><a href=\"#findings-by-category\">Категории</a><a href=\"#plugins\">Плагины</a><a href=\"#logs\">Логи</a><a href=\"#findings\">Проблемы</a><input class=\"search\" id=\"findingSearch\" type=\"search\" placeholder=\"Поиск по проблемам, ID, файлам и рекомендациям\"><button class=\"filter active\" data-filter=\"all\">Все</button><button class=\"filter\" data-filter=\"CRITICAL\">CRITICAL</button><button class=\"filter\" data-filter=\"DANGER\">DANGER</button><button class=\"filter\" data-filter=\"WARN\">WARN</button><button class=\"filter\" data-filter=\"INFO\">INFO</button><button class=\"print-btn\" type=\"button\" onclick=\"window.print()\">Печать / PDF</button></div></nav><div class=\"wrap\">")
	fmt.Fprintf(&b, "<section class=\"hero\"><div class=\"card\" id=\"summary\"><h1>Отчёт CraftDoctor</h1><p class=\"muted\">Сгенерировано %s. Путь: <code>%s</code></p><p>HTML Report 3.0: dashboard, score breakdown, risk matrix, поиск по находкам, печатный режим, группировка по категориям и приоритетный план исправлений.</p>", html.EscapeString(rep.GeneratedAt.Format("2006-01-02 15:04:05")), html.EscapeString(rep.TargetPath))
	fmt.Fprintf(&b, "<p>Статус: <span class=\"badge %s\">%s</span></p><div class=\"grid\">", statusClass, html.EscapeString(string(rep.Status)))
	kpi(&b, "CRITICAL", rep.Summary.Critical)
	kpi(&b, "DANGER", rep.Summary.Danger)
	kpi(&b, "WARN", rep.Summary.Warn)
	kpi(&b, "INFO", rep.Summary.Info)
	fmt.Fprintf(&b, "</div></div><div class=\"card score\"><div class=\"score-ring\" style=\"--score:%d\"><span>%d</span><div class=\"muted\">/100</div></div></div></section>", score, score)

	writeHTMLTopFixesV2(&b, rep)
	writeHTMLScoreBreakdown(&b, rep, score)
	writeHTMLCategorySummary(&b, rep)
	writeHTMLRiskMatrix(&b, rep)
	writeHTMLQuickCommands(&b, rep)
	writeHTMLExportCommands(&b, rep)
	writeHTMLRules(&b, rep)
	writeHTMLModuleDashboard(&b, rep)
	writeHTMLFindingsByCategory(&b, rep)

	fmt.Fprintf(&b, "<div class=\"card\" id=\"system\"><h2>Серверная машина</h2><div class=\"grid\">")
	kv(&b, "ОС", rep.System.OS)
	kv(&b, "Архитектура", rep.System.Arch)
	kv(&b, "CPU-потоки", fmt.Sprint(rep.System.CPUs))
	kv(&b, "RAM", fmt.Sprintf("%d МБ", rep.System.TotalMemoryMB))
	kv(&b, "Свободно на диске", fmt.Sprintf("%d МБ", rep.System.FreeDiskMB))
	kv(&b, "Всего диск", fmt.Sprintf("%d МБ", rep.System.TotalDiskMB))
	fmt.Fprintf(&b, "</div></div>")

	fmt.Fprintf(&b, "<div class=\"card\" id=\"minecraft\"><h2>Java и Minecraft</h2><div class=\"grid\">")
	if rep.Java.Found {
		kv(&b, "Java", rep.Java.Version)
	} else {
		kv(&b, "Java", "не найдена")
	}
	kv(&b, "Ядро", rep.Minecraft.CoreType)
	kv(&b, "Jar", rep.Minecraft.CoreJar)
	kv(&b, "EULA", fmt.Sprint(rep.Minecraft.EULAAccepted))
	kv(&b, "Proxy forwarding", fmt.Sprint(rep.Minecraft.ProxyForwarded))
	kv(&b, "Crash reports", fmt.Sprint(rep.Minecraft.CrashReports))
	fmt.Fprintf(&b, "</div></div>")

	writeHTMLConfigDoctor(&b, rep)

	fmt.Fprintf(&b, "<div class=\"card\" id=\"plugins\"><h2>Плагины</h2>")
	if len(rep.Plugins) == 0 {
		fmt.Fprintf(&b, "<p class=\"muted\">Плагины не найдены.</p>")
	} else {
		fmt.Fprintf(&b, "<table><thead><tr><th>Название</th><th>Версия</th><th>API</th><th>Folia</th><th>Классов</th><th>Jar</th><th>Категории</th><th>Зависимости</th><th>Shaded</th></tr></thead><tbody>")
		for _, p := range rep.Plugins {
			fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%d</td><td><code>%s</code></td><td>%s</td><td>%s</td><td>%s</td></tr>", html.EscapeString(p.Name), html.EscapeString(p.Version), html.EscapeString(p.APIVersion), html.EscapeString(foliaLabel(p.FoliaSupported)), p.ClassCount, html.EscapeString(p.JarFile), html.EscapeString(strings.Join(p.Categories, ", ")), html.EscapeString(strings.Join(p.Depends, ", ")), html.EscapeString(strings.Join(p.ShadedLibraries, ", ")))
		}
		fmt.Fprintf(&b, "</tbody></table>")
	}
	fmt.Fprintf(&b, "</div>")
	writeHTMLPluginAudit(&b, rep)

	fmt.Fprintf(&b, "<div class=\"card\" id=\"logs\"><h2>Логи</h2><div class=\"grid\">")
	kv(&b, "latest.log", fmt.Sprint(rep.Logs.LatestLogFound))
	kv(&b, "WARN", fmt.Sprint(rep.Logs.WarnCount))
	kv(&b, "ERROR", fmt.Sprint(rep.Logs.ErrorCount))
	kv(&b, "Exception", fmt.Sprint(rep.Logs.ExceptionCount))
	kv(&b, "Long tick", fmt.Sprint(rep.Logs.LongTickCount))
	kv(&b, "Сессии", fmt.Sprint(len(rep.Logs.Sessions)))
	kv(&b, "Stack fingerprints", fmt.Sprint(len(rep.Logs.StackTraces)))
	fmt.Fprintf(&b, "</div>")
	if len(rep.Logs.Samples) > 0 {
		fmt.Fprintf(&b, "<details><summary>Примеры строк из логов</summary>")
		for _, s := range rep.Logs.Samples {
			fmt.Fprintf(&b, "<pre>%s</pre>", html.EscapeString(s))
		}
		fmt.Fprintf(&b, "</details>")
	}
	fmt.Fprintf(&b, "</div>")
	writeHTMLLogDoctor(&b, rep)
	writeHTMLPerformanceDoctor(&b, rep)
	writeHTMLProxyDoctor(&b, rep)
	writeHTMLSecurityDoctor(&b, rep)
	writeHTMLProductionDoctor(&b, rep)

	fmt.Fprintf(&b, "<div class=\"card\" id=\"findings\"><h2>Проблемы и рекомендации</h2><p class=\"muted\">Фильтры сверху скрывают/показывают проблемы по уровню серьёзности.</p>")
	for _, f := range rep.Findings {
		class := severityClass(f.Severity)
		fmt.Fprintf(&b, "<div class=\"card finding %s\" data-severity=\"%s\" data-category=\"%s\" data-search=\"%s\"><h3><span class=\"badge %s\">%s</span> %s</h3>", class, html.EscapeString(string(f.Severity)), html.EscapeString(f.Category), html.EscapeString(strings.ToLower(f.ID+" "+f.Category+" "+f.Title+" "+f.Message+" "+f.Recommendation+" "+f.File)), class, html.EscapeString(string(f.Severity)), html.EscapeString(f.Title))
		fmt.Fprintf(&b, "<p class=\"muted\">ID: <code>%s</code> · Категория: <code>%s</code></p>", html.EscapeString(f.ID), html.EscapeString(f.Category))
		if f.File != "" {
			fmt.Fprintf(&b, "<p>Файл: <code>%s</code></p>", html.EscapeString(f.File))
		}
		fmt.Fprintf(&b, "<p>%s</p>", html.EscapeString(f.Message))
		if f.Recommendation != "" {
			fmt.Fprintf(&b, "<p><strong>Рекомендация:</strong> %s</p>", html.EscapeString(f.Recommendation))
		}
		fmt.Fprintf(&b, "</div>")
	}
	fmt.Fprintf(&b, "</div><p class=\"muted\">CraftDoctor — личный open-source проект Дмитрия &quot;SkiF4er&quot; Ефимова.</p></div>")
	b.WriteString(`<script>
(function(){
  var activeSeverity = 'all';
  var search = '';
  function applyFilters(){
    document.querySelectorAll('[data-severity]').forEach(function(el){
      var bySeverity = activeSeverity === 'all' || el.dataset.severity === activeSeverity;
      var haystack = el.dataset.search || el.textContent.toLowerCase();
      var bySearch = !search || haystack.indexOf(search) !== -1;
      el.classList.toggle('hidden', !(bySeverity && bySearch));
    });
  }
  document.querySelectorAll('.filter').forEach(function(btn){btn.addEventListener('click',function(){activeSeverity=btn.dataset.filter;document.querySelectorAll('.filter').forEach(function(x){x.classList.remove('active')});btn.classList.add('active');applyFilters();})});
  var input=document.getElementById('findingSearch');
  if(input){input.addEventListener('input',function(){search=input.value.trim().toLowerCase();applyFilters();});}
})();
</script></body></html>`)
	return b.String()
}

func severityClass(s model.Severity) string {
	switch s {
	case model.SeverityCritical:
		return "critical"
	case model.SeverityDanger:
		return "danger"
	case model.SeverityWarn:
		return "warn"
	case model.SeverityInfo:
		return "info"
	default:
		return "info"
	}
}

func reportScore(rep *model.Report) int {
	score := 100 - rep.Summary.Critical*25 - rep.Summary.Danger*12 - rep.Summary.Warn*4
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func writeHTMLTopFixesV2(b *bytes.Buffer, rep *model.Report) {
	items := priorityFindings(rep, 7)
	fmt.Fprintf(b, "<div class=\"card\" id=\"priority\"><h2>Что исправить первым</h2>")
	if len(items) == 0 {
		fmt.Fprintf(b, "<p class=\"muted\">Критичных и предупреждающих находок нет.</p></div>")
		return
	}
	fmt.Fprintf(b, "<table><thead><tr><th>#</th><th>Уровень</th><th>Проблема</th><th>Файл</th><th>Рекомендация</th></tr></thead><tbody>")
	for i, f := range items {
		class := severityClass(f.Severity)
		fmt.Fprintf(b, "<tr data-severity=\"%s\"><td>%d</td><td><span class=\"badge %s\">%s</span></td><td>%s<br><span class=\"muted small\"><code>%s</code></span></td><td><code>%s</code></td><td>%s</td></tr>", html.EscapeString(string(f.Severity)), i+1, class, html.EscapeString(string(f.Severity)), html.EscapeString(f.Title), html.EscapeString(f.ID), html.EscapeString(f.File), html.EscapeString(f.Recommendation))
	}
	fmt.Fprintf(b, "</tbody></table></div>")
}

func writeHTMLCategorySummary(b *bytes.Buffer, rep *model.Report) {
	if len(rep.Findings) == 0 {
		return
	}
	type counts struct{ critical, danger, warn, info int }
	byCategory := map[string]*counts{}
	categories := make([]string, 0)
	for _, f := range rep.Findings {
		if _, ok := byCategory[f.Category]; !ok {
			byCategory[f.Category] = &counts{}
			categories = append(categories, f.Category)
		}
		c := byCategory[f.Category]
		switch f.Severity {
		case model.SeverityCritical:
			c.critical++
		case model.SeverityDanger:
			c.danger++
		case model.SeverityWarn:
			c.warn++
		default:
			c.info++
		}
	}
	fmt.Fprintf(b, "<div class=\"card\"><h2>Сводка по категориям</h2><table><thead><tr><th>Категория</th><th>CRITICAL</th><th>DANGER</th><th>WARN</th><th>INFO</th></tr></thead><tbody>")
	for _, cat := range categories {
		c := byCategory[cat]
		fmt.Fprintf(b, "<tr><td>%s</td><td>%d</td><td>%d</td><td>%d</td><td>%d</td></tr>", html.EscapeString(cat), c.critical, c.danger, c.warn, c.info)
	}
	fmt.Fprintf(b, "</tbody></table></div>")
}

func writeHTMLQuickCommands(b *bytes.Buffer, rep *model.Report) {
	target := html.EscapeString(rep.TargetPath)
	fmt.Fprintf(b, "<div class=\"card\"><h2>Команды для повторной проверки</h2>")
	cmds := []string{
		fmt.Sprintf("craftdoctor scan %s --format html --output report.html", rep.TargetPath),
		fmt.Sprintf("craftdoctor scan %s --format json --output report.json --fail-on-critical", rep.TargetPath),
		fmt.Sprintf("craftdoctor plugins audit %s", rep.TargetPath),
		fmt.Sprintf("craftdoctor logs analyze %s", rep.TargetPath),
	}
	_ = target
	for _, cmd := range cmds {
		fmt.Fprintf(b, "<div class=\"copycmd\"><code>%s</code></div>", html.EscapeString(cmd))
	}
	fmt.Fprintf(b, "</div>")
}

func writeHTMLModuleDashboard(b *bytes.Buffer, rep *model.Report) {
	fmt.Fprintf(b, "<div class=\"card\" id=\"modules\"><h2>Doctor-модули</h2><div class=\"grid\">")
	moduleCard(b, "Config Doctor", rep.Config.Status, fmt.Sprintf("файлов: %d · проверок: %d", len(rep.Config.Files), len(rep.Config.Checks)), "#config")
	moduleCard(b, "Plugin Doctor", statusByFindings(rep.Findings, "plugins.", "плагины"), fmt.Sprintf("плагинов: %d · зависимостей: %d · классов-дублей: %d", rep.PluginAudit.Total, len(rep.PluginAudit.DependencyEdges), len(rep.PluginAudit.DuplicateClasses)), "#plugins")
	moduleCard(b, "Log Doctor 2.0", statusByFindings(rep.Findings, "logs.", "логи"), fmt.Sprintf("WARN: %d · ERROR: %d · root-cause: %d · stack: %d", rep.Logs.WarnCount, rep.Logs.ErrorCount, len(rep.Logs.RootCauses), len(rep.Logs.StackTraces)), "#logs")
	moduleCard(b, "Performance Doctor 2.0", rep.Performance.Status, fmt.Sprintf("метрик: %d · рисков: %d · профилей: %d", len(rep.Performance.Metrics), len(rep.Performance.Risks), len(rep.Performance.Profiles)), "#performance")
	moduleCard(b, "Proxy / Network Doctor", rep.Proxy.Status, fmt.Sprintf("proxy: %t · проверок: %d", rep.Proxy.Detected, len(rep.Proxy.Checks)), "#proxy")
	moduleCard(b, "Security Doctor", rep.Security.Status, fmt.Sprintf("проверок: %d · секретов: %d", len(rep.Security.Checks), len(rep.Security.Secrets)), "#security")
	moduleCard(b, "Production Doctor", rep.Production.Status, fmt.Sprintf("проверок: %d · бэкапов: %d · Docker: %t", len(rep.Production.Checks), len(rep.Production.BackupCandidates), rep.Production.Docker.Detected), "#production")
	fmt.Fprintf(b, "</div></div>")
}

func moduleCard(b *bytes.Buffer, title string, status model.Severity, summary string, anchor string) {
	class := severityClass(status)
	fmt.Fprintf(b, "<a class=\"kpi module\" href=\"%s\"><span><strong>%s</strong><br><span class=\"muted\">%s</span></span><span class=\"badge %s\">%s</span></a>", html.EscapeString(anchor), html.EscapeString(title), html.EscapeString(summary), class, html.EscapeString(string(status)))
}

func statusByFindings(findings []model.Finding, prefix, category string) model.Severity {
	status := model.SeverityInfo
	for _, f := range findings {
		if strings.HasPrefix(f.ID, prefix) || f.Category == category {
			if f.Severity.Rank() > status.Rank() {
				status = f.Severity
			}
		}
	}
	return status
}

func writeHTMLScoreBreakdown(b *bytes.Buffer, rep *model.Report, score int) {
	fmt.Fprintf(b, "<div class=\"card\" id=\"score-breakdown\"><div class=\"section-title\"><h2>Score breakdown</h2><span class=\"badge %s\">%d/100</span></div>", severityClass(rep.Status), score)
	fmt.Fprintf(b, "<p class=\"muted\">Оценка начинается со 100 и снижается за находки: CRITICAL −25, DANGER −12, WARN −4. INFO не снижает score.</p>")
	fmt.Fprintf(b, "<div class=\"riskbar\">")
	scoreKPI(b, "CRITICAL", rep.Summary.Critical, rep.Summary.Critical*25, "critical")
	scoreKPI(b, "DANGER", rep.Summary.Danger, rep.Summary.Danger*12, "danger")
	scoreKPI(b, "WARN", rep.Summary.Warn, rep.Summary.Warn*4, "warn")
	scoreKPI(b, "INFO", rep.Summary.Info, 0, "info")
	fmt.Fprintf(b, "</div></div>")
}

func scoreKPI(b *bytes.Buffer, title string, count int, penalty int, class string) {
	fmt.Fprintf(b, "<div class=\"kpi\"><span class=\"badge %s\">%s</span><br><strong>%d</strong><div class=\"penalty\">штраф: −%d</div></div>", class, html.EscapeString(title), count, penalty)
}

func writeHTMLRiskMatrix(b *bytes.Buffer, rep *model.Report) {
	if len(rep.Findings) == 0 {
		return
	}
	type counts struct{ critical, danger, warn, info int }
	byCategory := map[string]*counts{}
	categories := make([]string, 0)
	for _, f := range rep.Findings {
		if _, ok := byCategory[f.Category]; !ok {
			byCategory[f.Category] = &counts{}
			categories = append(categories, f.Category)
		}
		c := byCategory[f.Category]
		switch f.Severity {
		case model.SeverityCritical:
			c.critical++
		case model.SeverityDanger:
			c.danger++
		case model.SeverityWarn:
			c.warn++
		default:
			c.info++
		}
	}
	fmt.Fprintf(b, "<div class=\"card\" id=\"risk\"><h2>Risk matrix</h2><p class=\"muted\">Матрица показывает, какие подсистемы создают основной риск проекта.</p><div class=\"table-wrap\"><table><thead><tr><th>Категория</th><th>CRITICAL</th><th>DANGER</th><th>WARN</th><th>INFO</th><th>Risk score</th></tr></thead><tbody>")
	for _, cat := range categories {
		c := byCategory[cat]
		risk := c.critical*25 + c.danger*12 + c.warn*4
		fmt.Fprintf(b, "<tr><td>%s</td><td>%d</td><td>%d</td><td>%d</td><td>%d</td><td><strong>%d</strong></td></tr>", html.EscapeString(cat), c.critical, c.danger, c.warn, c.info, risk)
	}
	fmt.Fprintf(b, "</tbody></table></div></div>")
}

func writeHTMLExportCommands(b *bytes.Buffer, rep *model.Report) {
	target := rep.TargetPath
	fmt.Fprintf(b, "<div class=\"card\" id=\"export\"><h2>Экспорт и передача отчёта</h2><p class=\"muted\">HTML удобно читать в браузере, Markdown — прикладывать в GitFlic Issue, JSON — использовать в CI/CD.</p>")
	cmds := []string{fmt.Sprintf("craftdoctor scan %s --format html --output craftdoctor-report.html", target), fmt.Sprintf("craftdoctor scan %s --format markdown --output craftdoctor-report.md", target), fmt.Sprintf("craftdoctor scan %s --format json --output craftdoctor-report.json", target), fmt.Sprintf("craftdoctor ci %s --output craftdoctor-ci-report.json", target)}
	for _, cmd := range cmds {
		fmt.Fprintf(b, "<div class=\"copycmd\"><code>%s</code></div>", html.EscapeString(cmd))
	}
	fmt.Fprintf(b, "</div>")
}

func writeHTMLFindingsByCategory(b *bytes.Buffer, rep *model.Report) {
	if len(rep.Findings) == 0 {
		return
	}
	groups := map[string][]model.Finding{}
	order := make([]string, 0)
	for _, f := range rep.Findings {
		if _, ok := groups[f.Category]; !ok {
			order = append(order, f.Category)
		}
		groups[f.Category] = append(groups[f.Category], f)
	}
	fmt.Fprintf(b, "<div class=\"card\" id=\"findings-by-category\"><h2>Находки по категориям</h2><p class=\"muted\">Группировка помогает быстро передать задачу нужному исполнителю: DevOps, разработчику плагина, администратору или владельцу проекта.</p>")
	for _, cat := range order {
		items := groups[cat]
		fmt.Fprintf(b, "<details open><summary>%s · %d</summary><table><thead><tr><th>Уровень</th><th>ID</th><th>Проблема</th><th>Файл</th></tr></thead><tbody>", html.EscapeString(cat), len(items))
		for _, f := range items {
			class := severityClass(f.Severity)
			fmt.Fprintf(b, "<tr data-severity=\"%s\" data-search=\"%s\"><td><span class=\"badge %s\">%s</span></td><td><code>%s</code></td><td>%s</td><td><code>%s</code></td></tr>", html.EscapeString(string(f.Severity)), html.EscapeString(strings.ToLower(f.ID+" "+f.Category+" "+f.Title+" "+f.File)), class, html.EscapeString(string(f.Severity)), html.EscapeString(f.ID), html.EscapeString(f.Title), html.EscapeString(f.File))
		}
		fmt.Fprintf(b, "</tbody></table></details>")
	}
	fmt.Fprintf(b, "</div>")
}

func writeMarkdownConfigDoctor(b *bytes.Buffer, rep *model.Report) {
	if len(rep.Config.Files) == 0 && len(rep.Config.Checks) == 0 {
		return
	}
	fmt.Fprintf(b, "## Config Doctor\n\n")
	fmt.Fprintf(b, "- Статус: `%s`\n", rep.Config.Status)
	fmt.Fprintf(b, "- Файлов конфигурации: `%d`\n", len(rep.Config.Files))
	fmt.Fprintf(b, "- Проверок: `%d`\n\n", len(rep.Config.Checks))
	if len(rep.Config.Files) > 0 {
		fmt.Fprintf(b, "### Файлы конфигурации\n\n")
		fmt.Fprintf(b, "| Файл | Тип | Валиден | Ключей | Дубли | Неизвестные | Устаревшие |\n|---|---|---:|---:|---|---|---|\n")
		for _, f := range rep.Config.Files {
			fmt.Fprintf(b, "| `%s` | %s | %t | %d | %s | %s | %s |\n", safeMD(f.Path), safeMD(f.Kind), f.Valid, f.KeyCount, safeMD(strings.Join(f.DuplicateKeys, ", ")), safeMD(strings.Join(f.UnknownKeys, ", ")), safeMD(strings.Join(f.DeprecatedKeys, ", ")))
		}
		fmt.Fprintf(b, "\n")
	}
	if len(rep.Config.Checks) > 0 {
		fmt.Fprintf(b, "### Проверки конфигурации\n\n")
		fmt.Fprintf(b, "| Уровень | Область | Цель | Статус | Сообщение |\n|---|---|---|---|---|\n")
		for _, c := range rep.Config.Checks {
			fmt.Fprintf(b, "| %s | %s | `%s` | %s | %s |\n", c.Severity, safeMD(c.Area), safeMD(c.Target), safeMD(c.Status), safeMD(c.Message))
		}
		fmt.Fprintf(b, "\n")
	}
}

func writeHTMLConfigDoctor(b *bytes.Buffer, rep *model.Report) {
	if len(rep.Config.Files) == 0 && len(rep.Config.Checks) == 0 {
		return
	}
	fmt.Fprintf(b, "<div class=\"card\" id=\"config\"><h2>Config Doctor</h2><div class=\"grid\">")
	kv(b, "Статус", string(rep.Config.Status))
	kv(b, "Файлов", fmt.Sprint(len(rep.Config.Files)))
	kv(b, "Проверок", fmt.Sprint(len(rep.Config.Checks)))
	kv(b, "Рекомендаций", fmt.Sprint(len(rep.Config.Recommendations)))
	fmt.Fprintf(b, "</div>")
	if len(rep.Config.Files) > 0 {
		fmt.Fprintf(b, "<h3>Файлы конфигурации</h3><table><thead><tr><th>Файл</th><th>Тип</th><th>Валиден</th><th>Ключей</th><th>Дубли</th><th>Неизвестные</th><th>Устаревшие</th></tr></thead><tbody>")
		for _, f := range rep.Config.Files {
			fmt.Fprintf(b, "<tr><td><code>%s</code></td><td>%s</td><td>%t</td><td>%d</td><td>%s</td><td>%s</td><td>%s</td></tr>", html.EscapeString(f.Path), html.EscapeString(f.Kind), f.Valid, f.KeyCount, html.EscapeString(strings.Join(f.DuplicateKeys, ", ")), html.EscapeString(strings.Join(f.UnknownKeys, ", ")), html.EscapeString(strings.Join(f.DeprecatedKeys, ", ")))
		}
		fmt.Fprintf(b, "</tbody></table>")
	}
	if len(rep.Config.Checks) > 0 {
		fmt.Fprintf(b, "<h3>Проверки</h3><table><thead><tr><th>Уровень</th><th>Область</th><th>Цель</th><th>Статус</th><th>Сообщение</th><th>Рекомендация</th></tr></thead><tbody>")
		for _, c := range rep.Config.Checks {
			class := strings.ToLower(string(c.Severity))
			fmt.Fprintf(b, "<tr><td><span class=\"badge %s\">%s</span></td><td>%s</td><td><code>%s</code></td><td>%s</td><td>%s</td><td>%s</td></tr>", class, html.EscapeString(string(c.Severity)), html.EscapeString(c.Area), html.EscapeString(c.Target), html.EscapeString(c.Status), html.EscapeString(c.Message), html.EscapeString(c.Recommendation))
		}
		fmt.Fprintf(b, "</tbody></table>")
	}
	fmt.Fprintf(b, "</div>")
}

func writeMarkdownProductionDoctor(b *bytes.Buffer, rep *model.Report) {
	if len(rep.Production.Checks) == 0 && len(rep.Production.BackupCandidates) == 0 && len(rep.Production.ServiceFiles) == 0 {
		return
	}
	fmt.Fprintf(b, "## Production Doctor\n\n")
	fmt.Fprintf(b, "- Статус: `%s`\n", rep.Production.Status)
	fmt.Fprintf(b, "- Проверок: `%d`\n", len(rep.Production.Checks))
	fmt.Fprintf(b, "- Backup-кандидатов: `%d`\n", len(rep.Production.BackupCandidates))
	fmt.Fprintf(b, "- Service-файлов: `%d`\n", len(rep.Production.ServiceFiles))
	fmt.Fprintf(b, "- Logrotate-файлов: `%d`\n", len(rep.Production.LogrotateFiles))
	fmt.Fprintf(b, "- Скриптов запуска: `%d`\n", len(rep.Production.StartupScripts))
	fmt.Fprintf(b, "- Команд запуска: `%d`\n", len(rep.Production.StartupCommands))
	fmt.Fprintf(b, "- Docker/Compose: `%t`\n", rep.Production.Docker.Detected)
	fmt.Fprintf(b, "- Restore-инструкций: `%d`\n", len(rep.Production.RestoreChecklist))
	fmt.Fprintf(b, "- Rollback-кандидатов: `%d`\n", len(rep.Production.RollbackCandidates))
	fmt.Fprintf(b, "- Staging-кандидатов: `%d`\n\n", len(rep.Production.StagingCandidates))
	if len(rep.Production.Checks) > 0 {
		fmt.Fprintf(b, "### Проверки production-ready\n\n")
		fmt.Fprintf(b, "| Уровень | Область | Цель | Статус | Сообщение |\n|---|---|---|---|---|\n")
		for _, check := range rep.Production.Checks {
			fmt.Fprintf(b, "| %s | %s | %s | %s | %s |\n", check.Severity, safeMD(check.Area), safeMD(check.Target), safeMD(check.Status), safeMD(check.Message))
		}
		fmt.Fprintf(b, "\n")
	}
	if len(rep.Production.BackupCandidates) > 0 {
		fmt.Fprintf(b, "### Найденные backup-кандидаты\n\n")
		fmt.Fprintf(b, "| Тип | Путь | Размер | Изменён |\n|---|---|---:|---|\n")
		for _, backup := range rep.Production.BackupCandidates {
			fmt.Fprintf(b, "| %s | `%s` | %d | `%s` |\n", safeMD(backup.Kind), safeMD(backup.Path), backup.SizeBytes, safeMD(backup.Modified))
		}
		fmt.Fprintf(b, "\n")
	}

	if len(rep.Production.StartupCommands) > 0 {
		fmt.Fprintf(b, "### Извлечённые команды запуска\n\n")
		fmt.Fprintf(b, "| Источник | Команда |\n|---|---|\n")
		for _, cmd := range rep.Production.StartupCommands {
			fmt.Fprintf(b, "| `%s` | `%s` |\n", safeMD(cmd.Source), safeMD(cmd.Command))
		}
		fmt.Fprintf(b, "\n")
	}
	if rep.Production.Docker.Detected {
		fmt.Fprintf(b, "### Docker / Compose\n\n")
		fmt.Fprintf(b, "- Compose-файлы: `%s`\n", safeMD(strings.Join(rep.Production.Docker.ComposeFiles, ", ")))
		fmt.Fprintf(b, "- Dockerfile: `%s`\n", safeMD(strings.Join(rep.Production.Docker.Dockerfiles, ", ")))
		fmt.Fprintf(b, "- Признаки volumes: `%t`\n", rep.Production.Docker.HasVolumeHints)
		fmt.Fprintf(b, "- Restart policy: `%t`\n\n", rep.Production.Docker.HasRestartPolicy)
	}
	if len(rep.Production.Recommendations) > 0 {
		fmt.Fprintf(b, "### Рекомендации Production Doctor\n\n")
		for _, rec := range rep.Production.Recommendations {
			fmt.Fprintf(b, "- %s\n", safeMD(rec))
		}
		fmt.Fprintf(b, "\n")
	}
}

func writeHTMLProductionDoctor(b *bytes.Buffer, rep *model.Report) {
	if len(rep.Production.Checks) == 0 && len(rep.Production.BackupCandidates) == 0 && len(rep.Production.ServiceFiles) == 0 {
		return
	}
	fmt.Fprintf(b, "<div class=\"card\"><h2>Production Doctor</h2><div class=\"grid\">")
	kv(b, "Статус", string(rep.Production.Status))
	kv(b, "Проверок", fmt.Sprint(len(rep.Production.Checks)))
	kv(b, "Backup-кандидатов", fmt.Sprint(len(rep.Production.BackupCandidates)))
	kv(b, "Service-файлов", fmt.Sprint(len(rep.Production.ServiceFiles)))
	kv(b, "Logrotate-файлов", fmt.Sprint(len(rep.Production.LogrotateFiles)))
	kv(b, "Скриптов запуска", fmt.Sprint(len(rep.Production.StartupScripts)))
	kv(b, "Команд запуска", fmt.Sprint(len(rep.Production.StartupCommands)))
	kv(b, "Docker/Compose", fmt.Sprint(rep.Production.Docker.Detected))
	kv(b, "Restore-инструкций", fmt.Sprint(len(rep.Production.RestoreChecklist)))
	kv(b, "Rollback", fmt.Sprint(len(rep.Production.RollbackCandidates)))
	kv(b, "Staging", fmt.Sprint(len(rep.Production.StagingCandidates)))
	fmt.Fprintf(b, "</div>")
	if len(rep.Production.Checks) > 0 {
		fmt.Fprintf(b, "<h3>Проверки production-ready</h3><table><thead><tr><th>Уровень</th><th>Область</th><th>Цель</th><th>Статус</th><th>Сообщение</th><th>Рекомендация</th></tr></thead><tbody>")
		for _, check := range rep.Production.Checks {
			class := strings.ToLower(string(check.Severity))
			fmt.Fprintf(b, "<tr><td><span class=\"badge %s\">%s</span></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>", class, html.EscapeString(string(check.Severity)), html.EscapeString(check.Area), html.EscapeString(check.Target), html.EscapeString(check.Status), html.EscapeString(check.Message), html.EscapeString(check.Recommendation))
		}
		fmt.Fprintf(b, "</tbody></table>")
	}
	if len(rep.Production.BackupCandidates) > 0 {
		fmt.Fprintf(b, "<h3>Backup-кандидаты</h3><table><thead><tr><th>Тип</th><th>Путь</th><th>Размер</th><th>Изменён</th></tr></thead><tbody>")
		for _, backup := range rep.Production.BackupCandidates {
			fmt.Fprintf(b, "<tr><td>%s</td><td><code>%s</code></td><td>%d</td><td><code>%s</code></td></tr>", html.EscapeString(backup.Kind), html.EscapeString(backup.Path), backup.SizeBytes, html.EscapeString(backup.Modified))
		}
		fmt.Fprintf(b, "</tbody></table>")
	}

	if len(rep.Production.StartupCommands) > 0 {
		fmt.Fprintf(b, "<h3>Извлечённые команды запуска</h3><table><thead><tr><th>Источник</th><th>Команда</th></tr></thead><tbody>")
		for _, cmd := range rep.Production.StartupCommands {
			fmt.Fprintf(b, "<tr><td><code>%s</code></td><td><code>%s</code></td></tr>", html.EscapeString(cmd.Source), html.EscapeString(cmd.Command))
		}
		fmt.Fprintf(b, "</tbody></table>")
	}
	if rep.Production.Docker.Detected {
		fmt.Fprintf(b, "<h3>Docker / Compose</h3><div class=\"grid\">")
		kv(b, "Compose-файлы", strings.Join(rep.Production.Docker.ComposeFiles, ", "))
		kv(b, "Dockerfile", strings.Join(rep.Production.Docker.Dockerfiles, ", "))
		kv(b, "Признаки volumes", fmt.Sprint(rep.Production.Docker.HasVolumeHints))
		kv(b, "Restart policy", fmt.Sprint(rep.Production.Docker.HasRestartPolicy))
		fmt.Fprintf(b, "</div>")
	}
	if len(rep.Production.Recommendations) > 0 {
		fmt.Fprintf(b, "<h3>Рекомендации</h3><ul>")
		for _, rec := range rep.Production.Recommendations {
			fmt.Fprintf(b, "<li>%s</li>", html.EscapeString(rec))
		}
		fmt.Fprintf(b, "</ul>")
	}
	fmt.Fprintf(b, "</div>")
}

func writeMarkdownProxyDoctor(b *bytes.Buffer, rep *model.Report) {
	if len(rep.Proxy.Checks) == 0 && len(rep.Proxy.Configs) == 0 {
		return
	}
	fmt.Fprintf(b, "## Proxy / Network Doctor\n\n")
	fmt.Fprintf(b, "- Статус: `%s`\n", rep.Proxy.Status)
	fmt.Fprintf(b, "- Proxy обнаружен: `%t`\n", rep.Proxy.Detected)
	fmt.Fprintf(b, "- Типы: `%s`\n", safeMD(strings.Join(rep.Proxy.Types, ", ")))
	fmt.Fprintf(b, "- Конфигов: `%d`\n", len(rep.Proxy.ConfigFiles))
	fmt.Fprintf(b, "- Узлов сети: `%d`\n", len(rep.Proxy.Nodes))
	fmt.Fprintf(b, "- Связей сети: `%d`\n\n", len(rep.Proxy.Edges))
	if len(rep.Proxy.Configs) > 0 {
		fmt.Fprintf(b, "### Proxy-конфигурации\n\n")
		fmt.Fprintf(b, "| Тип | Файл | Bind | Forwarding | Secret | Backend-серверов |\n|---|---|---|---|---|---:|\n")
		for _, cfg := range rep.Proxy.Configs {
			fmt.Fprintf(b, "| %s | `%s` | `%s` | `%s` | `%t` | %d |\n", safeMD(cfg.Type), safeMD(cfg.File), safeMD(cfg.Bind), safeMD(cfg.ForwardingMode), cfg.SecretPresent, len(cfg.Servers))
		}
		fmt.Fprintf(b, "\n")
	}
	if len(rep.Proxy.Edges) > 0 {
		fmt.Fprintf(b, "### Карта сети\n\n")
		fmt.Fprintf(b, "| Откуда | Связь | Куда |\n|---|---|---|\n")
		for _, edge := range rep.Proxy.Edges {
			fmt.Fprintf(b, "| `%s` | `%s` | `%s` |\n", safeMD(edge.From), safeMD(edge.Kind), safeMD(edge.To))
		}
		fmt.Fprintf(b, "\n")
	}
	if len(rep.Proxy.Checks) > 0 {
		fmt.Fprintf(b, "### Проверки proxy/network\n\n")
		fmt.Fprintf(b, "| Уровень | Область | Цель | Статус | Сообщение |\n|---|---|---|---|---|\n")
		for _, check := range rep.Proxy.Checks {
			fmt.Fprintf(b, "| %s | %s | %s | %s | %s |\n", check.Severity, safeMD(check.Area), safeMD(check.Target), safeMD(check.Status), safeMD(check.Message))
		}
		fmt.Fprintf(b, "\n")
	}
	if len(rep.Proxy.Recommendations) > 0 {
		fmt.Fprintf(b, "### Рекомендации Proxy / Network Doctor\n\n")
		for _, rec := range rep.Proxy.Recommendations {
			fmt.Fprintf(b, "- %s\n", safeMD(rec))
		}
		fmt.Fprintf(b, "\n")
	}
}

func writeHTMLProxyDoctor(b *bytes.Buffer, rep *model.Report) {
	if len(rep.Proxy.Checks) == 0 && len(rep.Proxy.Configs) == 0 {
		return
	}
	fmt.Fprintf(b, "<div class=\"card\"><h2>Proxy / Network Doctor</h2><div class=\"grid\">")
	kv(b, "Статус", string(rep.Proxy.Status))
	kv(b, "Proxy обнаружен", fmt.Sprint(rep.Proxy.Detected))
	kv(b, "Типы", strings.Join(rep.Proxy.Types, ", "))
	kv(b, "Конфигов", fmt.Sprint(len(rep.Proxy.ConfigFiles)))
	kv(b, "Узлов сети", fmt.Sprint(len(rep.Proxy.Nodes)))
	kv(b, "Связей сети", fmt.Sprint(len(rep.Proxy.Edges)))
	fmt.Fprintf(b, "</div>")
	if len(rep.Proxy.Configs) > 0 {
		fmt.Fprintf(b, "<h3>Proxy-конфигурации</h3><table><thead><tr><th>Тип</th><th>Файл</th><th>Bind</th><th>Forwarding</th><th>Secret</th><th>Backend</th></tr></thead><tbody>")
		for _, cfg := range rep.Proxy.Configs {
			fmt.Fprintf(b, "<tr><td>%s</td><td><code>%s</code></td><td><code>%s</code></td><td><code>%s</code></td><td>%t</td><td>%d</td></tr>", html.EscapeString(cfg.Type), html.EscapeString(cfg.File), html.EscapeString(cfg.Bind), html.EscapeString(cfg.ForwardingMode), cfg.SecretPresent, len(cfg.Servers))
		}
		fmt.Fprintf(b, "</tbody></table>")
	}
	if len(rep.Proxy.Edges) > 0 {
		fmt.Fprintf(b, "<h3>Карта сети</h3><table><thead><tr><th>Откуда</th><th>Связь</th><th>Куда</th></tr></thead><tbody>")
		for _, edge := range rep.Proxy.Edges {
			fmt.Fprintf(b, "<tr><td><code>%s</code></td><td><code>%s</code></td><td><code>%s</code></td></tr>", html.EscapeString(edge.From), html.EscapeString(edge.Kind), html.EscapeString(edge.To))
		}
		fmt.Fprintf(b, "</tbody></table>")
	}
	if len(rep.Proxy.Checks) > 0 {
		fmt.Fprintf(b, "<h3>Проверки proxy/network</h3><table><thead><tr><th>Уровень</th><th>Область</th><th>Цель</th><th>Статус</th><th>Сообщение</th><th>Рекомендация</th></tr></thead><tbody>")
		for _, check := range rep.Proxy.Checks {
			class := strings.ToLower(string(check.Severity))
			fmt.Fprintf(b, "<tr><td><span class=\"badge %s\">%s</span></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>", class, html.EscapeString(string(check.Severity)), html.EscapeString(check.Area), html.EscapeString(check.Target), html.EscapeString(check.Status), html.EscapeString(check.Message), html.EscapeString(check.Recommendation))
		}
		fmt.Fprintf(b, "</tbody></table>")
	}
	if len(rep.Proxy.Recommendations) > 0 {
		fmt.Fprintf(b, "<h3>Рекомендации</h3><ul>")
		for _, rec := range rep.Proxy.Recommendations {
			fmt.Fprintf(b, "<li>%s</li>", html.EscapeString(rec))
		}
		fmt.Fprintf(b, "</ul>")
	}
	fmt.Fprintf(b, "</div>")
}

func writeMarkdownSecurityDoctor(b *bytes.Buffer, rep *model.Report) {
	if len(rep.Security.Checks) == 0 && len(rep.Security.Secrets) == 0 {
		return
	}
	fmt.Fprintf(b, "## Security Doctor 2.0\n\n")
	fmt.Fprintf(b, "- Статус: `%s`\n", rep.Security.Status)
	fmt.Fprintf(b, "- Security score: `%d/100`\n", rep.Security.Score)
	fmt.Fprintf(b, "- Проверок: `%d`\n", len(rep.Security.Checks))
	fmt.Fprintf(b, "- Найденных секретов: `%d`\n", len(rep.Security.Secrets))
	fmt.Fprintf(b, "- Просканировано файлов на секреты: `%d`\n", rep.Security.SecretFilesScanned)
	if rep.Security.SecretsIgnoreFile != "" {
		fmt.Fprintf(b, "- Allowlist секретов: `%s` (`%d` правил)\n", rep.Security.SecretsIgnoreFile, len(rep.Security.SecretsIgnorePatterns))
	}
	fmt.Fprintf(b, "- Чувствительных файлов: `%d`\n", len(rep.Security.SensitiveFiles))
	fmt.Fprintf(b, "- Сигналов по плагинам: `%d`\n\n", len(rep.Security.PluginSignals))
	if len(rep.Security.Checks) > 0 {
		fmt.Fprintf(b, "### Проверки безопасности\n\n")
		fmt.Fprintf(b, "| Уровень | Область | Цель | Статус | Сообщение |\n|---|---|---|---|---|\n")
		for _, check := range rep.Security.Checks {
			fmt.Fprintf(b, "| %s | %s | %s | %s | %s |\n", check.Severity, safeMD(check.Area), safeMD(check.Target), safeMD(check.Status), safeMD(check.Message))
		}
		fmt.Fprintf(b, "\n")
	}
	if len(rep.Security.Secrets) > 0 {
		fmt.Fprintf(b, "### Найденные секреты\n\n")
		fmt.Fprintf(b, "| Уровень | Тип | Файл | Строка | Доказательство |\n|---|---|---|---:|---|\n")
		for _, secret := range rep.Security.Secrets {
			fmt.Fprintf(b, "| %s | %s | `%s` | %d | `%s` |\n", secret.Severity, safeMD(secret.Kind), safeMD(secret.File), secret.Line, safeMD(secret.Evidence))
		}
		fmt.Fprintf(b, "\n")
	}
	if len(rep.Security.SensitiveFiles) > 0 {
		fmt.Fprintf(b, "### Чувствительные файлы\n\n")
		fmt.Fprintf(b, "| Файл | Тип | Права | Рекомендация |\n|---|---|---|---|\n")
		for _, file := range rep.Security.SensitiveFiles {
			fmt.Fprintf(b, "| `%s` | %s | `%s` | %s |\n", safeMD(file.Path), safeMD(file.Kind), safeMD(file.Mode), safeMD(file.Recommendation))
		}
		fmt.Fprintf(b, "\n")
	}
	if len(rep.Security.PluginSignals) > 0 {
		fmt.Fprintf(b, "### Сигналы по плагинам\n\n")
		fmt.Fprintf(b, "| Уровень | Плагин | Тип | Рекомендация |\n|---|---|---|---|\n")
		for _, signal := range rep.Security.PluginSignals {
			fmt.Fprintf(b, "| %s | %s | %s | %s |\n", signal.Severity, safeMD(signal.Name), safeMD(signal.Kind), safeMD(signal.Recommendation))
		}
		fmt.Fprintf(b, "\n")
	}
	if len(rep.Security.Recommendations) > 0 {
		fmt.Fprintf(b, "### Рекомендации Security Doctor\n\n")
		for _, rec := range rep.Security.Recommendations {
			fmt.Fprintf(b, "- %s\n", safeMD(rec))
		}
		fmt.Fprintf(b, "\n")
	}
}

func writeHTMLSecurityDoctor(b *bytes.Buffer, rep *model.Report) {
	if len(rep.Security.Checks) == 0 && len(rep.Security.Secrets) == 0 {
		return
	}
	fmt.Fprintf(b, "<div class=\"card\"><h2>Security Doctor 2.0</h2><div class=\"grid\">")
	kv(b, "Статус", string(rep.Security.Status))
	kv(b, "Security score", fmt.Sprintf("%d/100", rep.Security.Score))
	kv(b, "Проверок", fmt.Sprint(len(rep.Security.Checks)))
	kv(b, "Секретов", fmt.Sprint(len(rep.Security.Secrets)))
	kv(b, "Файлов на секреты", fmt.Sprint(rep.Security.SecretFilesScanned))
	kv(b, "Sensitive files", fmt.Sprint(len(rep.Security.SensitiveFiles)))
	kv(b, "Plugin signals", fmt.Sprint(len(rep.Security.PluginSignals)))
	if rep.Security.SecretsIgnoreFile != "" {
		kv(b, "Allowlist", fmt.Sprintf("%s · %d правил", rep.Security.SecretsIgnoreFile, len(rep.Security.SecretsIgnorePatterns)))
	}
	fmt.Fprintf(b, "</div>")
	if len(rep.Security.Checks) > 0 {
		fmt.Fprintf(b, "<h3>Проверки безопасности</h3><table><thead><tr><th>Уровень</th><th>Область</th><th>Цель</th><th>Статус</th><th>Сообщение</th><th>Рекомендация</th></tr></thead><tbody>")
		for _, check := range rep.Security.Checks {
			class := strings.ToLower(string(check.Severity))
			fmt.Fprintf(b, "<tr><td><span class=\"badge %s\">%s</span></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>", class, html.EscapeString(string(check.Severity)), html.EscapeString(check.Area), html.EscapeString(check.Target), html.EscapeString(check.Status), html.EscapeString(check.Message), html.EscapeString(check.Recommendation))
		}
		fmt.Fprintf(b, "</tbody></table>")
	}
	if len(rep.Security.Secrets) > 0 {
		fmt.Fprintf(b, "<h3>Найденные секреты</h3><table><thead><tr><th>Уровень</th><th>Тип</th><th>Файл</th><th>Строка</th><th>Доказательство</th></tr></thead><tbody>")
		for _, secret := range rep.Security.Secrets {
			class := strings.ToLower(string(secret.Severity))
			fmt.Fprintf(b, "<tr><td><span class=\"badge %s\">%s</span></td><td>%s</td><td><code>%s</code></td><td>%d</td><td><code>%s</code></td></tr>", class, html.EscapeString(string(secret.Severity)), html.EscapeString(secret.Kind), html.EscapeString(secret.File), secret.Line, html.EscapeString(secret.Evidence))
		}
		fmt.Fprintf(b, "</tbody></table>")
	}
	if len(rep.Security.SensitiveFiles) > 0 {
		fmt.Fprintf(b, "<h3>Чувствительные файлы</h3><table><thead><tr><th>Файл</th><th>Тип</th><th>Права</th><th>Рекомендация</th></tr></thead><tbody>")
		for _, file := range rep.Security.SensitiveFiles {
			fmt.Fprintf(b, "<tr><td><code>%s</code></td><td>%s</td><td><code>%s</code></td><td>%s</td></tr>", html.EscapeString(file.Path), html.EscapeString(file.Kind), html.EscapeString(file.Mode), html.EscapeString(file.Recommendation))
		}
		fmt.Fprintf(b, "</tbody></table>")
	}
	if len(rep.Security.PluginSignals) > 0 {
		fmt.Fprintf(b, "<h3>Сигналы по плагинам</h3><table><thead><tr><th>Уровень</th><th>Плагин</th><th>Тип</th><th>Рекомендация</th></tr></thead><tbody>")
		for _, signal := range rep.Security.PluginSignals {
			class := strings.ToLower(string(signal.Severity))
			fmt.Fprintf(b, "<tr><td><span class=\"badge %s\">%s</span></td><td>%s</td><td>%s</td><td>%s</td></tr>", class, html.EscapeString(string(signal.Severity)), html.EscapeString(signal.Name), html.EscapeString(signal.Kind), html.EscapeString(signal.Recommendation))
		}
		fmt.Fprintf(b, "</tbody></table>")
	}
	if len(rep.Security.Recommendations) > 0 {
		fmt.Fprintf(b, "<h3>Рекомендации</h3><ul>")
		for _, rec := range rep.Security.Recommendations {
			fmt.Fprintf(b, "<li>%s</li>", html.EscapeString(rec))
		}
		fmt.Fprintf(b, "</ul>")
	}
	fmt.Fprintf(b, "</div>")
}

func writeMarkdownPerformanceDoctor(b *bytes.Buffer, rep *model.Report) {
	if len(rep.Performance.Metrics) == 0 && len(rep.Performance.Risks) == 0 && len(rep.Performance.JVMFlags) == 0 && len(rep.Performance.Profiles) == 0 {
		return
	}
	fmt.Fprintf(b, "## Performance Doctor 2.0\n\n")
	fmt.Fprintf(b, "- Статус: `%s`\n", rep.Performance.Status)
	if rep.Performance.Options.Players > 0 || rep.Performance.Options.Target != "" {
		fmt.Fprintf(b, "- Профиль нагрузки: `target=%s`, `players=%d`\n", safeMD(rep.Performance.Options.Target), rep.Performance.Options.Players)
	}
	if rep.Performance.HeapMinMB > 0 || rep.Performance.HeapMaxMB > 0 {
		fmt.Fprintf(b, "- Heap: `Xms=%d МБ`, `Xmx=%d МБ`\n", rep.Performance.HeapMinMB, rep.Performance.HeapMaxMB)
	}
	fmt.Fprintf(b, "- spark найден: `%t`\n", rep.Performance.HasSpark)
	fmt.Fprintf(b, "- Импортированных profiler/timings-отчётов: `%d`\n", len(rep.Performance.Profiles))
	if len(rep.Performance.JVMFlagSources) > 0 {
		fmt.Fprintf(b, "- Источники JVM-флагов: `%s`\n", safeMD(strings.Join(rep.Performance.JVMFlagSources, ", ")))
	}
	fmt.Fprintf(b, "- Метрик: `%d`\n", len(rep.Performance.Metrics))
	fmt.Fprintf(b, "- Рисков: `%d`\n\n", len(rep.Performance.Risks))
	if rep.Performance.Capacity.RecommendedHeapMB > 0 {
		fmt.Fprintf(b, "### Capacity-ориентиры\n\n")
		fmt.Fprintf(b, "| Игроки | Тип | Heap | view-distance | simulation-distance |\n|---:|---|---:|---:|---:|\n")
		fmt.Fprintf(b, "| %d | %s | %d МБ | %d | %d |\n\n", rep.Performance.Capacity.Players, safeMD(rep.Performance.Capacity.Target), rep.Performance.Capacity.RecommendedHeapMB, rep.Performance.Capacity.RecommendedViewDistance, rep.Performance.Capacity.RecommendedSimulationDistance)
	}
	if len(rep.Performance.Profiles) > 0 {
		fmt.Fprintf(b, "### Импортированные profiler/timings-отчёты\n\n")
		fmt.Fprintf(b, "| Тип | Источник | TPS | MSPT avg | MSPT p95 | MSPT max |\n|---|---|---:|---:|---:|---:|\n")
		for _, profile := range rep.Performance.Profiles {
			fmt.Fprintf(b, "| %s | `%s` | %.2f | %.2f | %.2f | %.2f |\n", safeMD(profile.Kind), safeMD(profile.Source), profile.TPS, profile.MSPTAvg, profile.MSPTP95, profile.MSPTMax)
		}
		fmt.Fprintf(b, "\n")
	}
	if len(rep.Performance.Risks) > 0 {
		fmt.Fprintf(b, "### Риски производительности\n\n")
		fmt.Fprintf(b, "| Уровень | Область | Сообщение | Рекомендация |\n|---|---|---|---|\n")
		for _, risk := range rep.Performance.Risks {
			fmt.Fprintf(b, "| %s | %s | %s | %s |\n", risk.Severity, safeMD(risk.Area), safeMD(risk.Message), safeMD(risk.Recommendation))
		}
		fmt.Fprintf(b, "\n")
	}
	if len(rep.Performance.Metrics) > 0 {
		fmt.Fprintf(b, "### Метрики и параметры\n\n")
		fmt.Fprintf(b, "| Статус | Параметр | Значение | Источник |\n|---|---|---|---|\n")
		for _, metric := range rep.Performance.Metrics {
			fmt.Fprintf(b, "| %s | %s | %s | %s |\n", metric.Status, safeMD(metric.Key), safeMD(metric.Value), safeMD(metric.Source))
		}
		fmt.Fprintf(b, "\n")
	}
	if len(rep.Performance.Recommendations) > 0 {
		fmt.Fprintf(b, "### Общие рекомендации\n\n")
		for _, rec := range rep.Performance.Recommendations {
			fmt.Fprintf(b, "- %s\n", safeMD(rec))
		}
		fmt.Fprintf(b, "\n")
	}
}

func writeHTMLPerformanceDoctor(b *bytes.Buffer, rep *model.Report) {
	if len(rep.Performance.Metrics) == 0 && len(rep.Performance.Risks) == 0 && len(rep.Performance.JVMFlags) == 0 && len(rep.Performance.Profiles) == 0 {
		return
	}
	fmt.Fprintf(b, "<div class=\"card\"><h2>Performance Doctor 2.0</h2><div class=\"grid\">")
	kv(b, "Статус", string(rep.Performance.Status))
	kv(b, "Xms", fmt.Sprintf("%d МБ", rep.Performance.HeapMinMB))
	kv(b, "Xmx", fmt.Sprintf("%d МБ", rep.Performance.HeapMaxMB))
	kv(b, "spark", fmt.Sprint(rep.Performance.HasSpark))
	kv(b, "Profiler/timings", fmt.Sprint(len(rep.Performance.Profiles)))
	kv(b, "Метрик", fmt.Sprint(len(rep.Performance.Metrics)))
	kv(b, "Рисков", fmt.Sprint(len(rep.Performance.Risks)))
	if rep.Performance.Options.Players > 0 || rep.Performance.Options.Target != "" {
		kv(b, "Профиль", fmt.Sprintf("%s / %d игроков", rep.Performance.Options.Target, rep.Performance.Options.Players))
	}
	fmt.Fprintf(b, "</div>")
	if len(rep.Performance.JVMFlagSources) > 0 {
		fmt.Fprintf(b, "<p class=\"muted\">Источники JVM-флагов: <code>%s</code></p>", html.EscapeString(strings.Join(rep.Performance.JVMFlagSources, ", ")))
	}
	if rep.Performance.Capacity.RecommendedHeapMB > 0 {
		fmt.Fprintf(b, "<h3>Capacity-ориентиры</h3><table><thead><tr><th>Игроки</th><th>Тип</th><th>Heap</th><th>view-distance</th><th>simulation-distance</th></tr></thead><tbody>")
		fmt.Fprintf(b, "<tr><td>%d</td><td>%s</td><td>%d МБ</td><td>%d</td><td>%d</td></tr>", rep.Performance.Capacity.Players, html.EscapeString(rep.Performance.Capacity.Target), rep.Performance.Capacity.RecommendedHeapMB, rep.Performance.Capacity.RecommendedViewDistance, rep.Performance.Capacity.RecommendedSimulationDistance)
		fmt.Fprintf(b, "</tbody></table>")
	}
	if len(rep.Performance.Profiles) > 0 {
		fmt.Fprintf(b, "<h3>Импортированные profiler/timings-отчёты</h3><table><thead><tr><th>Тип</th><th>Источник</th><th>TPS</th><th>MSPT avg</th><th>MSPT p95</th><th>MSPT max</th></tr></thead><tbody>")
		for _, profile := range rep.Performance.Profiles {
			fmt.Fprintf(b, "<tr><td>%s</td><td><code>%s</code></td><td>%.2f</td><td>%.2f</td><td>%.2f</td><td>%.2f</td></tr>", html.EscapeString(profile.Kind), html.EscapeString(profile.Source), profile.TPS, profile.MSPTAvg, profile.MSPTP95, profile.MSPTMax)
		}
		fmt.Fprintf(b, "</tbody></table>")
	}
	if len(rep.Performance.Risks) > 0 {
		fmt.Fprintf(b, "<h3>Риски производительности</h3><table><thead><tr><th>Уровень</th><th>Область</th><th>Сообщение</th><th>Рекомендация</th></tr></thead><tbody>")
		for _, risk := range rep.Performance.Risks {
			class := strings.ToLower(string(risk.Severity))
			fmt.Fprintf(b, "<tr><td><span class=\"badge %s\">%s</span></td><td>%s</td><td>%s</td><td>%s</td></tr>", class, html.EscapeString(string(risk.Severity)), html.EscapeString(risk.Area), html.EscapeString(risk.Message), html.EscapeString(risk.Recommendation))
		}
		fmt.Fprintf(b, "</tbody></table>")
	}
	if len(rep.Performance.Metrics) > 0 {
		fmt.Fprintf(b, "<h3>Метрики и параметры</h3><table><thead><tr><th>Статус</th><th>Параметр</th><th>Значение</th><th>Источник</th></tr></thead><tbody>")
		for _, metric := range rep.Performance.Metrics {
			class := strings.ToLower(string(metric.Status))
			fmt.Fprintf(b, "<tr><td><span class=\"badge %s\">%s</span></td><td>%s</td><td>%s</td><td><code>%s</code></td></tr>", class, html.EscapeString(string(metric.Status)), html.EscapeString(metric.Key), html.EscapeString(metric.Value), html.EscapeString(metric.Source))
		}
		fmt.Fprintf(b, "</tbody></table>")
	}
	if len(rep.Performance.Recommendations) > 0 {
		fmt.Fprintf(b, "<h3>Общие рекомендации</h3><ul>")
		for _, rec := range rep.Performance.Recommendations {
			fmt.Fprintf(b, "<li>%s</li>", html.EscapeString(rec))
		}
		fmt.Fprintf(b, "</ul>")
	}
	fmt.Fprintf(b, "</div>")
}

func writeMarkdownLogDoctor(b *bytes.Buffer, rep *model.Report) {
	if len(rep.Logs.RootCauses) == 0 && len(rep.Logs.Components) == 0 && len(rep.Logs.Issues) == 0 && len(rep.Logs.StackTraces) == 0 && len(rep.Logs.Sessions) == 0 {
		return
	}
	fmt.Fprintf(b, "## Log Doctor 2.0\n\n")
	if rep.Logs.AnalyzedFile != "" {
		fmt.Fprintf(b, "- Файл: `%s`\n", rep.Logs.AnalyzedFile)
	}
	if rep.Logs.LinesAnalyzed > 0 {
		fmt.Fprintf(b, "- Строк проанализировано: `%d`\n", rep.Logs.LinesAnalyzed)
	}
	fmt.Fprintf(b, "- Сессий запуска: `%d`\n", len(rep.Logs.Sessions))
	fmt.Fprintf(b, "- Stack trace fingerprint: `%d`\n", len(rep.Logs.StackTraces))
	if rep.Logs.SelectedSession != nil {
		fmt.Fprintf(b, "- Выбранная сессия: `#%d`, строки `%d-%d`\n", rep.Logs.SelectedSession.Index, rep.Logs.SelectedSession.StartLine, rep.Logs.SelectedSession.EndLine)
	}
	fmt.Fprintf(b, "\n")
	if rep.Logs.MainRootCause != nil {
		fmt.Fprintf(b, "### Главная причина\n\n")
		rc := rep.Logs.MainRootCause
		fmt.Fprintf(b, "- **[%s] %s** · компонент: `%s` · повторов: `%d`\n", rc.Severity, safeMD(rc.Title), safeMD(rc.Component), rc.Count)
		if rc.Recommendation != "" {
			fmt.Fprintf(b, "  - Рекомендация: %s\n", safeMD(rc.Recommendation))
		}
		fmt.Fprintf(b, "\n")
	}
	if len(rep.Logs.StackTraces) > 0 {
		fmt.Fprintf(b, "### Stack trace fingerprint\n\n")
		fmt.Fprintf(b, "| Fingerprint | Exception | Component | Повторов | Строки | Cause |\n|---|---|---|---:|---|---|\n")
		for _, st := range rep.Logs.StackTraces {
			fmt.Fprintf(b, "| `%s` | %s | %s | %d | %d-%d | %s |\n", safeMD(st.Fingerprint), safeMD(st.Exception), safeMD(st.Component), st.Count, st.FirstLine, st.LastLine, safeMD(st.Cause))
		}
		fmt.Fprintf(b, "\n")
	}
	if len(rep.Logs.IssueTypes) > 0 {
		fmt.Fprintf(b, "### Типы проблем\n\n")
		fmt.Fprintf(b, "| Уровень | Тип | Повторов |\n|---|---|---:|\n")
		for _, t := range rep.Logs.IssueTypes {
			fmt.Fprintf(b, "| %s | %s | %d |\n", t.Severity, safeMD(t.Type), t.Count)
		}
		fmt.Fprintf(b, "\n")
	}
	if len(rep.Logs.Sessions) > 0 {
		fmt.Fprintf(b, "### Сессии запуска\n\n")
		fmt.Fprintf(b, "| # | Строки | Причина | Время начала |\n|---:|---|---|---|\n")
		for _, session := range rep.Logs.Sessions {
			fmt.Fprintf(b, "| %d | %d-%d | %s | %s |\n", session.Index, session.StartLine, session.EndLine, safeMD(session.Reason), safeMD(session.StartTime))
		}
		fmt.Fprintf(b, "\n")
	}
	if len(rep.Logs.RootCauses) > 0 {
		fmt.Fprintf(b, "### Предполагаемые root cause\n\n")
		for _, rc := range rep.Logs.RootCauses {
			fmt.Fprintf(b, "- **[%s] %s**", rc.Severity, safeMD(rc.Title))
			if rc.Component != "" && rc.Component != "не определён" {
				fmt.Fprintf(b, " · компонент: `%s`", safeMD(rc.Component))
			}
			fmt.Fprintf(b, " · повторов: `%d`\n", rc.Count)
			if rc.Recommendation != "" {
				fmt.Fprintf(b, "  - Рекомендация: %s\n", safeMD(rc.Recommendation))
			}
		}
		fmt.Fprintf(b, "\n")
	}
	if len(rep.Logs.Components) > 0 {
		fmt.Fprintf(b, "### Компоненты с проблемами\n\n")
		fmt.Fprintf(b, "| Компонент | CRITICAL | ERROR | WARN |\n|---|---:|---:|---:|\n")
		for _, c := range rep.Logs.Components {
			fmt.Fprintf(b, "| %s | %d | %d | %d |\n", safeMD(c.Component), c.Critical, c.Errors, c.Warnings)
		}
		fmt.Fprintf(b, "\n")
	}
	if len(rep.Logs.Issues) > 0 {
		fmt.Fprintf(b, "### Сгруппированные проблемы логов\n\n")
		fmt.Fprintf(b, "| Уровень | Тип | Компонент | Повторов | Первая строка |\n|---|---|---|---:|---:|\n")
		for _, issue := range rep.Logs.Issues {
			fmt.Fprintf(b, "| %s | %s | %s | %d | %d |\n", issue.Severity, safeMD(issue.Type), safeMD(issue.Component), issue.Count, issue.Line)
		}
		fmt.Fprintf(b, "\n")
	}
}

func writeHTMLLogDoctor(b *bytes.Buffer, rep *model.Report) {
	if len(rep.Logs.RootCauses) == 0 && len(rep.Logs.Components) == 0 && len(rep.Logs.Issues) == 0 && len(rep.Logs.StackTraces) == 0 && len(rep.Logs.Sessions) == 0 {
		return
	}
	fmt.Fprintf(b, "<div class=\"card\"><h2>Log Doctor 2.0</h2>")
	fmt.Fprintf(b, "<div class=\"grid\">")
	if rep.Logs.AnalyzedFile != "" {
		kv(b, "Файл", rep.Logs.AnalyzedFile)
	}
	if rep.Logs.LinesAnalyzed > 0 {
		kv(b, "Строк проанализировано", fmt.Sprint(rep.Logs.LinesAnalyzed))
	}
	kv(b, "Root cause", fmt.Sprint(len(rep.Logs.RootCauses)))
	kv(b, "Групп проблем", fmt.Sprint(len(rep.Logs.Issues)))
	kv(b, "Сессии", fmt.Sprint(len(rep.Logs.Sessions)))
	kv(b, "Stack fingerprints", fmt.Sprint(len(rep.Logs.StackTraces)))
	fmt.Fprintf(b, "</div>")
	if rep.Logs.MainRootCause != nil {
		rc := rep.Logs.MainRootCause
		fmt.Fprintf(b, "<h3>Главная причина</h3><p><span class=\"badge %s\">%s</span> <strong>%s</strong> · компонент: <code>%s</code> · повторов: %d</p><p><strong>Рекомендация:</strong> %s</p>", strings.ToLower(string(rc.Severity)), html.EscapeString(string(rc.Severity)), html.EscapeString(rc.Title), html.EscapeString(rc.Component), rc.Count, html.EscapeString(rc.Recommendation))
	}
	if len(rep.Logs.StackTraces) > 0 {
		fmt.Fprintf(b, "<h3>Stack trace fingerprint</h3><table><thead><tr><th>Fingerprint</th><th>Exception</th><th>Component</th><th>Повторов</th><th>Строки</th><th>Cause</th></tr></thead><tbody>")
		for _, st := range rep.Logs.StackTraces {
			fmt.Fprintf(b, "<tr><td><code>%s</code></td><td>%s</td><td>%s</td><td>%d</td><td>%d-%d</td><td>%s</td></tr>", html.EscapeString(st.Fingerprint), html.EscapeString(st.Exception), html.EscapeString(st.Component), st.Count, st.FirstLine, st.LastLine, html.EscapeString(st.Cause))
		}
		fmt.Fprintf(b, "</tbody></table>")
	}
	if len(rep.Logs.IssueTypes) > 0 {
		fmt.Fprintf(b, "<h3>Типы проблем</h3><table><thead><tr><th>Уровень</th><th>Тип</th><th>Повторов</th></tr></thead><tbody>")
		for _, t := range rep.Logs.IssueTypes {
			class := strings.ToLower(string(t.Severity))
			fmt.Fprintf(b, "<tr><td><span class=\"badge %s\">%s</span></td><td>%s</td><td>%d</td></tr>", class, html.EscapeString(string(t.Severity)), html.EscapeString(t.Type), t.Count)
		}
		fmt.Fprintf(b, "</tbody></table>")
	}
	if len(rep.Logs.Sessions) > 0 {
		fmt.Fprintf(b, "<h3>Сессии запуска</h3><table><thead><tr><th>#</th><th>Строки</th><th>Причина</th><th>Время начала</th></tr></thead><tbody>")
		for _, session := range rep.Logs.Sessions {
			fmt.Fprintf(b, "<tr><td>%d</td><td>%d-%d</td><td>%s</td><td>%s</td></tr>", session.Index, session.StartLine, session.EndLine, html.EscapeString(session.Reason), html.EscapeString(session.StartTime))
		}
		fmt.Fprintf(b, "</tbody></table>")
	}
	if len(rep.Logs.RootCauses) > 0 {
		fmt.Fprintf(b, "<h3>Предполагаемые root cause</h3><table><thead><tr><th>Уровень</th><th>Проблема</th><th>Компонент</th><th>Повторов</th><th>Рекомендация</th></tr></thead><tbody>")
		for _, rc := range rep.Logs.RootCauses {
			class := strings.ToLower(string(rc.Severity))
			fmt.Fprintf(b, "<tr><td><span class=\"badge %s\">%s</span></td><td>%s</td><td>%s</td><td>%d</td><td>%s</td></tr>", class, html.EscapeString(string(rc.Severity)), html.EscapeString(rc.Title), html.EscapeString(rc.Component), rc.Count, html.EscapeString(rc.Recommendation))
		}
		fmt.Fprintf(b, "</tbody></table>")
	}
	if len(rep.Logs.Components) > 0 {
		fmt.Fprintf(b, "<h3>Компоненты с проблемами</h3><table><thead><tr><th>Компонент</th><th>CRITICAL</th><th>ERROR</th><th>WARN</th></tr></thead><tbody>")
		for _, c := range rep.Logs.Components {
			fmt.Fprintf(b, "<tr><td>%s</td><td>%d</td><td>%d</td><td>%d</td></tr>", html.EscapeString(c.Component), c.Critical, c.Errors, c.Warnings)
		}
		fmt.Fprintf(b, "</tbody></table>")
	}
	if len(rep.Logs.Issues) > 0 {
		fmt.Fprintf(b, "<h3>Сгруппированные проблемы</h3><table><thead><tr><th>Уровень</th><th>Тип</th><th>Компонент</th><th>Повторов</th><th>Доказательство</th></tr></thead><tbody>")
		for _, issue := range rep.Logs.Issues {
			class := strings.ToLower(string(issue.Severity))
			fmt.Fprintf(b, "<tr><td><span class=\"badge %s\">%s</span></td><td>%s</td><td>%s</td><td>%d</td><td><code>%s</code></td></tr>", class, html.EscapeString(string(issue.Severity)), html.EscapeString(issue.Type), html.EscapeString(issue.Component), issue.Count, html.EscapeString(issue.Evidence))
		}
		fmt.Fprintf(b, "</tbody></table>")
	}
	fmt.Fprintf(b, "</div>")
}

func writeMarkdownPluginAudit(b *bytes.Buffer, rep *model.Report) {
	if rep.PluginAudit.Total == 0 {
		return
	}
	fmt.Fprintf(b, "## Plugin Doctor\n\n")
	fmt.Fprintf(b, "- Всего плагинов: `%d`\n", rep.PluginAudit.Total)
	fmt.Fprintf(b, "- Корректных: `%d`\n", rep.PluginAudit.Valid)
	fmt.Fprintf(b, "- Проблемных: `%d`\n", rep.PluginAudit.Invalid)
	fmt.Fprintf(b, "- Связей зависимостей: `%d`\n", len(rep.PluginAudit.DependencyEdges))
	fmt.Fprintf(b, "- Отсутствующих обязательных зависимостей: `%d`\n", len(rep.PluginAudit.MissingDependencies))
	fmt.Fprintf(b, "- Циклов обязательных зависимостей: `%d`\n", len(rep.PluginAudit.Cycles))
	fmt.Fprintf(b, "- Дублей классов: `%d`\n", len(rep.PluginAudit.DuplicateClasses))
	fmt.Fprintf(b, "- Потенциальных конфликтов: `%d`\n\n", len(rep.PluginAudit.Conflicts))
	if len(rep.Plugins) > 0 {
		fmt.Fprintf(b, "### Сводка Plugin Intelligence\n\n")
		fmt.Fprintf(b, "| Плагин | Версия | API | Folia | Классов | Shaded | Libraries |\n|---|---|---|---|---:|---|---|\n")
		for _, p := range rep.Plugins {
			fmt.Fprintf(b, "| %s | %s | %s | %s | %d | %s | %s |\n", safeMD(p.Name), safeMD(p.Version), safeMD(p.APIVersion), safeMD(foliaLabel(p.FoliaSupported)), p.ClassCount, safeMD(strings.Join(p.ShadedLibraries, ", ")), safeMD(strings.Join(p.Libraries, ", ")))
		}
		fmt.Fprintf(b, "\n")
	}
	if len(rep.PluginAudit.Categories) > 0 {
		fmt.Fprintf(b, "### Категории плагинов\n\n")
		for _, cat := range rep.PluginAudit.Categories {
			fmt.Fprintf(b, "- **%s:** %s\n", safeMD(cat.Category), safeMD(strings.Join(cat.Plugins, ", ")))
		}
		fmt.Fprintf(b, "\n")
	}
	if len(rep.PluginAudit.Cycles) > 0 {
		fmt.Fprintf(b, "### Циклы зависимостей\n\n")
		for _, cycle := range rep.PluginAudit.Cycles {
			fmt.Fprintf(b, "- `%s`\n", safeMD(strings.Join(cycle.Plugins, " -> ")))
		}
		fmt.Fprintf(b, "\n")
	}
}

func writeHTMLPluginAudit(b *bytes.Buffer, rep *model.Report) {
	if rep.PluginAudit.Total == 0 {
		return
	}
	fmt.Fprintf(b, "<div class=\"card\"><h2>Plugin Doctor</h2><div class=\"grid\">")
	kv(b, "Всего плагинов", fmt.Sprint(rep.PluginAudit.Total))
	kv(b, "Корректных", fmt.Sprint(rep.PluginAudit.Valid))
	kv(b, "Проблемных", fmt.Sprint(rep.PluginAudit.Invalid))
	kv(b, "Связей зависимостей", fmt.Sprint(len(rep.PluginAudit.DependencyEdges)))
	kv(b, "Отсутствующих depend", fmt.Sprint(len(rep.PluginAudit.MissingDependencies)))
	kv(b, "Циклов depend", fmt.Sprint(len(rep.PluginAudit.Cycles)))
	kv(b, "Дублей классов", fmt.Sprint(len(rep.PluginAudit.DuplicateClasses)))
	kv(b, "Конфликтов", fmt.Sprint(len(rep.PluginAudit.Conflicts)))
	fmt.Fprintf(b, "</div>")
	if len(rep.PluginAudit.Categories) > 0 {
		fmt.Fprintf(b, "<h3>Категории</h3><table><thead><tr><th>Категория</th><th>Плагины</th></tr></thead><tbody>")
		for _, cat := range rep.PluginAudit.Categories {
			fmt.Fprintf(b, "<tr><td>%s</td><td>%s</td></tr>", html.EscapeString(cat.Category), html.EscapeString(strings.Join(cat.Plugins, ", ")))
		}
		fmt.Fprintf(b, "</tbody></table>")
	}
	if len(rep.PluginAudit.Cycles) > 0 {
		fmt.Fprintf(b, "<h3>Циклы зависимостей</h3><ul>")
		for _, cycle := range rep.PluginAudit.Cycles {
			fmt.Fprintf(b, "<li><code>%s</code></li>", html.EscapeString(strings.Join(cycle.Plugins, " -> ")))
		}
		fmt.Fprintf(b, "</ul>")
	}
	if len(rep.PluginAudit.Conflicts) > 0 {
		fmt.Fprintf(b, "<h3>Потенциальные конфликты</h3><table><thead><tr><th>Тип</th><th>Плагины</th><th>Причина</th><th>Рекомендация</th></tr></thead><tbody>")
		for _, c := range rep.PluginAudit.Conflicts {
			fmt.Fprintf(b, "<tr><td><code>%s</code></td><td>%s</td><td>%s</td><td>%s</td></tr>", html.EscapeString(c.Kind), html.EscapeString(strings.Join(c.Plugins, ", ")), html.EscapeString(c.Reason), html.EscapeString(c.Recommendation))
		}
		fmt.Fprintf(b, "</tbody></table>")
	}
	if len(rep.PluginAudit.DuplicateClasses) > 0 {
		fmt.Fprintf(b, "<h3>Дубли классов</h3><table><thead><tr><th>Класс</th><th>Плагины</th></tr></thead><tbody>")
		for _, d := range rep.PluginAudit.DuplicateClasses {
			fmt.Fprintf(b, "<tr><td><code>%s</code></td><td>%s</td></tr>", html.EscapeString(d.Class), html.EscapeString(strings.Join(d.Plugins, ", ")))
		}
		fmt.Fprintf(b, "</tbody></table>")
	}
	fmt.Fprintf(b, "</div>")
}

func foliaLabel(value *bool) string {
	if value == nil {
		return "не указано"
	}
	if *value {
		return "true"
	}
	return "false"
}

func writeMarkdownRules(b *bytes.Buffer, rep *model.Report) {
	if rep.Rules.CatalogVersion == "" && rep.Rules.BuiltinRules == 0 && rep.Rules.SuppressedFindings == 0 {
		return
	}
	fmt.Fprintf(b, "## Правила диагностики\n\n")
	if rep.Rules.CatalogVersion != "" {
		fmt.Fprintf(b, "- Версия каталога правил: `%s`\n", rep.Rules.CatalogVersion)
	}
	if rep.Rules.BuiltinRules > 0 {
		fmt.Fprintf(b, "- Встроенных правил: `%d`\n", rep.Rules.BuiltinRules)
	}
	if rep.Rules.CustomRulesFile != "" {
		fmt.Fprintf(b, "- Дополнительный файл правил: `%s`\n", rep.Rules.CustomRulesFile)
	}
	if rep.Rules.IgnoreFile != "" {
		fmt.Fprintf(b, "- Файл игнорирования: `%s`\n", rep.Rules.IgnoreFile)
	}
	if len(rep.Rules.IgnorePatterns) > 0 {
		fmt.Fprintf(b, "- Активные ignore-паттерны: `%s`\n", safeMD(strings.Join(rep.Rules.IgnorePatterns, ", ")))
	}
	if rep.Rules.SuppressedFindings > 0 {
		fmt.Fprintf(b, "- Подавлено находок: `%d`\n", rep.Rules.SuppressedFindings)
	}
	fmt.Fprintf(b, "\n")
}

func writeHTMLRules(b *bytes.Buffer, rep *model.Report) {
	if rep.Rules.CatalogVersion == "" && rep.Rules.BuiltinRules == 0 && rep.Rules.SuppressedFindings == 0 {
		return
	}
	fmt.Fprintf(b, "<div class=\"card\"><h2>Правила диагностики</h2><div class=\"grid\">")
	if rep.Rules.CatalogVersion != "" {
		kv(b, "Версия каталога", rep.Rules.CatalogVersion)
	}
	if rep.Rules.BuiltinRules > 0 {
		kv(b, "Встроенных правил", fmt.Sprint(rep.Rules.BuiltinRules))
	}
	if rep.Rules.CustomRulesFile != "" {
		kv(b, "Файл правил", rep.Rules.CustomRulesFile)
	}
	if rep.Rules.IgnoreFile != "" {
		kv(b, "Файл игнорирования", rep.Rules.IgnoreFile)
	}
	if len(rep.Rules.IgnorePatterns) > 0 {
		kv(b, "Ignore-паттерны", strings.Join(rep.Rules.IgnorePatterns, ", "))
	}
	if rep.Rules.SuppressedFindings > 0 {
		kv(b, "Подавлено", fmt.Sprint(rep.Rules.SuppressedFindings))
	}
	fmt.Fprintf(b, "</div></div>")
}

func priorityFindings(rep *model.Report, limit int) []model.Finding {
	var out []model.Finding
	for _, f := range rep.Findings {
		if f.Severity == model.SeverityInfo {
			continue
		}
		out = append(out, f)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func writeMarkdownPriority(b *bytes.Buffer, rep *model.Report) {
	items := priorityFindings(rep, 5)
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "## Что исправить первым\n\n")
	for i, f := range items {
		fmt.Fprintf(b, "%d. **[%s] %s**", i+1, f.Severity, safeMD(f.Title))
		if f.File != "" {
			fmt.Fprintf(b, " — `%s`", f.File)
		}
		fmt.Fprintf(b, "\n")
		if f.Recommendation != "" {
			fmt.Fprintf(b, "   - Рекомендация: %s\n", safeMD(f.Recommendation))
		}
	}
	fmt.Fprintf(b, "\n")
}

func writeHTMLPriority(b *bytes.Buffer, rep *model.Report) {
	items := priorityFindings(rep, 5)
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "<div class=\"card\"><h2>Что исправить первым</h2><ol>")
	for _, f := range items {
		class := strings.ToLower(string(f.Severity))
		fmt.Fprintf(b, "<li><strong><span class=\"badge %s\">%s</span> %s</strong>", class, html.EscapeString(string(f.Severity)), html.EscapeString(f.Title))
		if f.File != "" {
			fmt.Fprintf(b, " <span class=\"muted\">(<code>%s</code>)</span>", html.EscapeString(f.File))
		}
		if f.Recommendation != "" {
			fmt.Fprintf(b, "<br><span class=\"muted\">%s</span>", html.EscapeString(f.Recommendation))
		}
		fmt.Fprintf(b, "</li>")
	}
	fmt.Fprintf(b, "</ol></div>")
}

func kpi(b *bytes.Buffer, label string, value int) {
	fmt.Fprintf(b, "<div class=\"kpi\"><div class=\"muted\">%s</div><strong>%d</strong></div>", html.EscapeString(label), value)
}

func kv(b *bytes.Buffer, key, value string) {
	fmt.Fprintf(b, "<div class=\"kpi\"><div class=\"muted\">%s</div><strong>%s</strong></div>", html.EscapeString(key), html.EscapeString(value))
}

func safeMD(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	if s == "" {
		return "—"
	}
	return s
}
