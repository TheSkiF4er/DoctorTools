package java

import (
	"bytes"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"gitflic.ru/skif4er/doctortools/internal/model"
)

func Analyze() (model.JavaInfo, []model.Finding) {
	info := model.JavaInfo{}
	var findings []model.Finding

	path, err := exec.LookPath("java")
	if err != nil {
		findings = append(findings, model.Finding{
			ID:             "java.not_found",
			Severity:       model.SeverityWarn,
			Category:       "java",
			Title:          "Java не найдена в PATH",
			Message:        "CraftDoctor не смог выполнить java -version.",
			Recommendation: "Установить Java 17/21 и убедиться, что команда java доступна пользователю сервера.",
		})
		return info, findings
	}

	cmd := exec.Command(path, "-version")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	_ = cmd.Run()
	raw := strings.TrimSpace(out.String())
	info.Found = true
	info.Executable = path
	info.Raw = raw
	info.Version, info.Major = parseJavaVersion(raw)

	if info.Major > 0 && info.Major < 17 {
		findings = append(findings, model.Finding{
			ID:             "java.version.too_old",
			Severity:       model.SeverityCritical,
			Category:       "java",
			Title:          "Java слишком старая",
			Message:        "Обнаружена Java " + info.Version + ". Современные Minecraft-серверы требуют Java 17 или Java 21.",
			Recommendation: "Установить Java 21 для актуальных версий Paper/Purpur/Folia или Java 17 для старых поддерживаемых веток.",
		})
	} else if info.Major == 17 {
		findings = append(findings, model.Finding{
			ID:             "java.version.17",
			Severity:       model.SeverityInfo,
			Category:       "java",
			Title:          "Обнаружена Java 17",
			Message:        "Java 17 подходит для многих версий Minecraft, но для новых веток может потребоваться Java 21.",
			Recommendation: "Если сервер ориентирован на Minecraft 1.20.5+ / 1.21+, проверить требования выбранного ядра.",
		})
	} else if info.Major >= 21 {
		findings = append(findings, model.Finding{
			ID:       "java.version.modern",
			Severity: model.SeverityInfo,
			Category: "java",
			Title:    "Обнаружена современная Java",
			Message:  "Java " + info.Version + " подходит для актуальных Minecraft-серверов.",
		})
	}

	return info, findings
}

func parseJavaVersion(raw string) (string, int) {
	re := regexp.MustCompile(`version "([^"]+)"`)
	match := re.FindStringSubmatch(raw)
	if len(match) < 2 {
		return "", 0
	}
	version := match[1]
	major := 0
	parts := strings.Split(version, ".")
	if len(parts) >= 2 && parts[0] == "1" {
		major, _ = strconv.Atoi(parts[1])
	} else if len(parts) >= 1 {
		num := regexp.MustCompile(`^\d+`).FindString(parts[0])
		major, _ = strconv.Atoi(num)
	}
	return version, major
}
