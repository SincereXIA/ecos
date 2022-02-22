//go:build linux || darwin
// +build linux darwin

package common

import (
	"syscall"
)

// GetAvailStorage get available storage (byte) in path
func GetAvailStorage(path string) uint64 {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return 0
	}

	return stat.Bavail * uint64(stat.Bsize)
}
