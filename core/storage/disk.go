package storage

import (
	"io"
	"os"
	"path/filepath"
)

// diskBackend 本地磁盘实现：直接包装 os / filepath，行为与原生调用一致
type diskBackend struct{}

func (diskBackend) Walk(root string, fn WalkFunc) error {
	return filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		return fn(p, FileInfo{Name: info.Name(), Size: info.Size(), ModTime: info.ModTime()})
	})
}

func (diskBackend) ReadFile(loc string) ([]byte, error) {
	return os.ReadFile(loc)
}

func (diskBackend) Open(loc string) (io.ReadCloser, error) {
	return os.Open(loc)
}

func (diskBackend) WriteFile(loc string, data []byte) error {
	if dir := filepath.Dir(loc); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(loc, data, 0644)
}

func (diskBackend) Stat(loc string) (FileInfo, error) {
	info, err := os.Stat(loc)
	if err != nil {
		return FileInfo{}, err
	}
	return FileInfo{Name: info.Name(), Size: info.Size(), ModTime: info.ModTime()}, nil
}

func (diskBackend) Exists(loc string) (bool, error) {
	_, err := os.Stat(loc)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
