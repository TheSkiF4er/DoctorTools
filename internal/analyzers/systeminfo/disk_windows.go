//go:build windows

package systeminfo

func diskSpaceMB(path string) (uint64, uint64) {
	// В Windows-сборке v1.x CraftDoctor оставляет дисковые метрики пустыми,
	// чтобы CLI оставался кроссплатформенным без внешних зависимостей.
	return 0, 0
}
