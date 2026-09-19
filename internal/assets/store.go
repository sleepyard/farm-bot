// Package assets 管理视觉模板图片（锚点 / 按钮截图）。
//
// 目录约定（优先 exe 同级，其次上一级，再回退工作目录）：
//
//	assets/assert/...   ← 对应原版 assets/assert
//	assets/Buttons/...  ← 对应原版 Buttons
package assets

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RootDir 返回资源根目录。分发包把 assets 放在 exe 旁边即可。
func RootDir() string {
	var cands []string
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		cands = append(cands, filepath.Join(dir, "assets"), filepath.Join(dir, "..", "assets"))
	}
	if wd, err := os.Getwd(); err == nil {
		cands = append(cands, filepath.Join(wd, "assets"))
	}
	cands = append(cands, "assets")
	for _, c := range cands {
		if looksLikeAssets(c) {
			return c
		}
	}
	return "assets"
}

func looksLikeAssets(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, "assert", "home_anchor.png"))
	return err == nil && !st.IsDir()
}

// Item 是清单中某一项的状态。
type Item struct {
	Path     string `json:"path"` // 相对 assets 的路径，使用 /
	Exists   bool   `json:"exists"`
	Size     int64  `json:"size,omitempty"`
	Modified string `json:"modified,omitempty"` // RFC3339
}

// ListStatus 返回清单中每项是否已上传。
func ListStatus() ([]Item, error) {
	root := RootDir()
	out := make([]Item, 0, len(Required))
	for _, rel := range Required {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		it := Item{Path: rel}
		st, err := os.Stat(abs)
		if err == nil && !st.IsDir() {
			it.Exists = true
			it.Size = st.Size()
			it.Modified = st.ModTime().UTC().Format(time.RFC3339)
		}
		out = append(out, it)
	}
	return out, nil
}

// SaveUpload 将上传内容写入清单中的相对路径（仅允许 Required 内路径）。
func SaveUpload(relPath string, r io.Reader) error {
	relPath = normalizeRel(relPath)
	canon := canonicalPath(relPath)
	if canon == "" {
		return fmt.Errorf("不在资源清单中: %s", relPath)
	}
	abs := filepath.Join(RootDir(), filepath.FromSlash(canon))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	tmp := abs + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	return os.Rename(tmp, abs)
}

func normalizeRel(p string) string {
	p = strings.ReplaceAll(p, `\`, `/`)
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimPrefix(p, "assets/")
	return filepath.ToSlash(filepath.Clean(p))
}

// canonicalPath 返回清单中的规范路径（保留原大小写）；未找到返回空串。
// 清单项的中文变体（如 keep_hand.cn.png）同样放行，返回变体规范路径。
func canonicalPath(rel string) string {
	for _, r := range Required {
		if strings.EqualFold(r, rel) {
			return r
		}
		if v := LangVariant(r, "cn"); strings.EqualFold(v, rel) {
			return v
		}
	}
	return ""
}
