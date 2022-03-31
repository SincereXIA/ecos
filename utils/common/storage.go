package common

import (
	"ecos/utils/logger"
	"errors"
	"os"
	"path"
)

// InitPath create path as an empty dir
func InitPath(path string) error {
	s, err := os.Stat(path) // path 是否存在
	if err != nil {
		if os.IsExist(err) { //
			return err // path 存在，且不是目录
		}
		logger.Infof("Path: %v not exist, create it", path)
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

func InitAndClearPath(path string) error {
	err := InitPath(path)
	if err != nil {
		return err
	}
	err = os.RemoveAll(path)
	return err
}

// InitParentPath create file parent path if not exist
func InitParentPath(filePath string) error {
	parentPath := path.Dir(filePath)
	return InitPath(parentPath)
}

// PathExists Check if a directory exists, return true means Exits, return false means Not Exits.
func PathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	return false
}
