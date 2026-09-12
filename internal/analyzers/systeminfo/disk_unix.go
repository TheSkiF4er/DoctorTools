//go:build linux || darwin || freebsd || openbsd || netbsd

package systeminfo

import "syscall"

func diskSpaceMB(path string) (uint64, uint64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0
	}
	total := stat.Blocks * uint64(stat.Bsize) / 1024 / 1024
	free := stat.Bavail * uint64(stat.Bsize) / 1024 / 1024
	return total, free
}
