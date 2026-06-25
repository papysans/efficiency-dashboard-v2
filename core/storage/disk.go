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
	dir := filepath.Dir(loc)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	// 原子写：先写同目录临时文件 + fsync，再 rename 覆盖目标。
	// 避免进程中断/写盘失败留半截文件——正文卸载要求「对象要么完整存在、要么不存在」，
	// 否则回读到截断 JSON 会复现工具事件解析退化（与硬截断同源）。rename 同目录是原子操作。
	tmp, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(loc)+"-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // rename 成功后为 no-op；任何中途失败则清理临时文件
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0644); err != nil {
		return err
	}
	return os.Rename(tmpName, loc)
}

func (diskBackend) Stat(loc string) (FileInfo, error) {
	info, err := os.Stat(loc)
	if err != nil {
		return FileInfo{}, err
	}
	return FileInfo{Name: info.Name(), Size: info.Size(), ModTime: info.ModTime()}, nil
}

func (diskBackend) Remove(loc string) error {
	if err := os.Remove(loc); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
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
