package common

import (
	"errors"
	"os"
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

// InitPath create path as an empty dir
func InitPath(path string) error {
	s, err := os.Stat(path) // path 是否存在
	if err != nil {
		if os.IsExist(err) { //
			return err // path 存在，且不是目录
		}
		err = os.MkdirAll(path, 0777) // 目录不存在，创建空目录
		if err != nil {
			return err
		}
		return nil
	}
	if !s.IsDir() {
		return errors.New("path exist and not a dir")
	}
	return nil
}
