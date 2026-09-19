package carddb

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var savedMu sync.Mutex

func savedRawDirFile() string {
	return filepath.Join(filepath.Dir(DefaultCacheDir()), "mtga_data_dir.txt")
}

// LoadSavedRawDir 读取用户上次在页面里选的 Raw 目录。
func LoadSavedRawDir() string {
	savedMu.Lock()
	defer savedMu.Unlock()
	b, err := os.ReadFile(savedRawDirFile())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// SaveRawDir 记住用户选择的卡牌数据目录（下次启动自动用）。
func SaveRawDir(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return os.ErrInvalid
	}
	path := savedRawDirFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	savedMu.Lock()
	defer savedMu.Unlock()
	return os.WriteFile(path, []byte(dir+"\n"), 0o644)
}
