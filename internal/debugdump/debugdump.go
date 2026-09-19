// Package debugdump 把导航搜图的 miss/click 截图写到 runtime/debug。
// 分发给用户的 zip 默认关闭；在本仓库里跑（能看到 go.mod）时自动打开。
package debugdump

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

const modulePath = "github.com/flourbrain/mtga-farm-bot"

// Enabled 是否落盘调试截图。
// MTGA_BOT_DEBUG=1 强制开，=0 强制关；未设置时仅开发树（旁侧有本模块 go.mod）开启。
func Enabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MTGA_BOT_DEBUG"))) {
	case "1", "true", "on", "yes":
		return true
	case "0", "false", "off", "no":
		return false
	}
	return looksLikeDevTree()
}

func looksLikeDevTree() bool {
	var dirs []string
	if wd, err := os.Getwd(); err == nil {
		dirs = append(dirs, wd)
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		dirs = append(dirs, dir, filepath.Dir(dir))
	}
	for _, dir := range dirs {
		b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err != nil {
			continue
		}
		if strings.Contains(string(b), "module "+modulePath) {
			return true
		}
	}
	return false
}

func safeStem(rel string) string {
	safe := strings.ReplaceAll(rel, "/", "_")
	safe = strings.ReplaceAll(safe, "\\", "_")
	return strings.TrimSuffix(safe, filepath.Ext(safe))
}

// SavePNG 在调试开启时写入 runtime/debug/<name>，返回路径；关闭时返回空串。
func SavePNG(img image.Image, name string) string {
	if !Enabled() || img == nil || name == "" {
		return ""
	}
	dir := filepath.Join("runtime", "debug")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	out := filepath.Join(dir, name)
	f, err := os.Create(out)
	if err != nil {
		return ""
	}
	defer f.Close()
	if png.Encode(f, img) != nil {
		return ""
	}
	return out
}

// MissName / ClickName 生成 miss_*.png / click_*.png 文件名。
func MissName(rel string) string  { return "miss_" + safeStem(rel) + ".png" }
func ClickName(rel string) string { return "click_" + safeStem(rel) + ".png" }
