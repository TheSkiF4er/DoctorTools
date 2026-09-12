package systeminfo

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"gitflic.ru/skif4er/doctortools/internal/model"
)

func Analyze(root string) (model.SystemInfo, []model.Finding) {
	info := model.SystemInfo{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
		CPUs: runtime.NumCPU(),
	}
	var findings []model.Finding

	if runtime.GOOS == "linux" {
		info.TotalMemoryMB = readLinuxMemTotalMB()
	}
	info.TotalDiskMB, info.FreeDiskMB = diskSpaceMB(root)
	info.WritableRoot = canWriteByMode(root)
	info.WritablePlugins = canWriteByMode(filepath.Join(root, "plugins"))

	if info.CPUs > 0 && info.CPUs < 2 {
		findings = append(findings, model.Finding{
			ID:             "system.cpu.low",
			Severity:       model.SeverityWarn,
			Category:       "серверная машина",
			Title:          "Мало CPU-потоков",
			Message:        "На машине обнаружено менее 2 CPU-потоков. Для публичного Minecraft-сервера это может быть ограничением.",
			Recommendation: "Для production-сервера использовать машину минимум с 2 CPU-потоками, а для нагруженного сервера — больше.",
		})
	}
	if info.TotalMemoryMB > 0 && info.TotalMemoryMB < 2048 {
		findings = append(findings, model.Finding{
			ID:             "system.memory.low",
			Severity:       model.SeverityWarn,
			Category:       "серверная машина",
			Title:          "Мало оперативной памяти",
			Message:        "На машине обнаружено менее 2 ГБ RAM.",
			Recommendation: "Для Paper/Purpur/Folia-сервера обычно требуется больше памяти, особенно при плагинах и нескольких мирах.",
		})
	}
	if info.FreeDiskMB > 0 && info.FreeDiskMB < 5120 {
		findings = append(findings, model.Finding{
			ID:             "system.disk.critical_free_space",
			Severity:       model.SeverityDanger,
			Category:       "серверная машина",
			Title:          "Критически мало свободного места",
			Message:        "На диске меньше 5 ГБ свободного места.",
			Recommendation: "Освободить место до запуска сервера, настройки бэкапов или обновления ядра/плагинов.",
		})
	} else if info.FreeDiskMB > 0 && info.FreeDiskMB < 10240 {
		findings = append(findings, model.Finding{
			ID:             "system.disk.low_free_space",
			Severity:       model.SeverityWarn,
			Category:       "серверная машина",
			Title:          "Мало свободного места",
			Message:        "На диске меньше 10 ГБ свободного места.",
			Recommendation: "Проверить логи, старые бэкапы, crash reports и размер миров.",
		})
	}
	if !info.WritableRoot {
		findings = append(findings, model.Finding{
			ID:             "system.permissions.root_not_writable",
			Severity:       model.SeverityDanger,
			Category:       "файловая система",
			Title:          "Нет прав на запись в корневую директорию сервера",
			Message:        "Текущий пользователь, вероятно, не сможет корректно обновлять конфиги, логи или runtime-файлы сервера.",
			Recommendation: "Проверить владельца файлов, права директории и пользователя, от которого запускается сервер.",
		})
	}

	return info, findings
}

func readLinuxMemTotalMB() uint64 {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseUint(fields[1], 10, 64)
				return kb / 1024
			}
		}
	}
	return 0
}

func canWriteByMode(path string) bool {
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	mode := st.Mode().Perm()
	return mode&0200 != 0 || mode&0020 != 0 || mode&0002 != 0
}
