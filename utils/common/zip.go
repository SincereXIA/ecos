package common

import (
	"archive/zip"
	"ecos/utils/logger"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func Zip(src_dir string, zip_file_name string) error {

	// 预防：旧文件无法覆盖
	err := os.RemoveAll(zip_file_name)
	if err != nil {
		logger.Errorf("remove old zip file error: %v", err)
		return err
	}

	// 创建：zip文件
	zipfile, err := os.Create(zip_file_name)
	if err != nil {
		logger.Errorf("create zip file error: %v", err)
		return err
	}
	defer zipfile.Close()

	// 打开：zip文件
	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	// 遍历路径信息
	filepath.Walk(src_dir, func(path string, info os.FileInfo, _ error) error {

		// 如果是源路径，提前进行下一个遍历
		if path == src_dir {
			return nil
		}

		// 获取：文件头信息
		header, _ := zip.FileInfoHeader(info)
		header.Name = strings.TrimPrefix(path, src_dir+`\`)

		// 判断：文件是不是文件夹
		if info.IsDir() {
			header.Name += `/`
		} else {
			// 设置：zip的文件压缩算法
			header.Method = zip.Deflate
		}

		// 创建：压缩包头部信息
		writer, err := archive.CreateHeader(header)
		if err != nil {
			logger.Errorf("create zip header error: %v", err)
			return err
		}
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				logger.Errorf("open file error: %v", err)
				return err
			}
			defer file.Close()
			io.Copy(writer, file)
		}
		return nil
	})
	return nil
}

func UnZip(zipFile, dir string) error {
	r, err := zip.OpenReader(zipFile)
	if err != nil {
		log.Fatalf("Open zip file1 failed: %s\n", err.Error())
	}
	defer r.Close()

	for _, f := range r.File {
		func() {
			path := dir + string(filepath.Separator) + f.Name
			os.MkdirAll(filepath.Dir(path), 0755)
			fDest, err := os.Create(path)
			if err != nil {
				logger.Errorf("Create failed: %s\n", err.Error())
				return
			}
			defer fDest.Close()

			fSrc, err := f.Open()
			if err != nil {
				log.Printf("Open failed: %s\n", err.Error())
				return
			}
			defer fSrc.Close()
			_, err = io.Copy(fDest, fSrc)
			if err != nil {
				log.Printf("Copy failed: %s\n", err.Error())
				return
			}
		}()
	}
	return nil
}
