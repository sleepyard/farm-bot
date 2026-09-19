package playerlog

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolvePath 按原版顺序定位 Player.log：
//  1. 环境变量 MTGA_BOT_LOG_PATH
//  2. 在 USERPROFILE / home 下扫描 LocalLow 路径，取 mtime 最新者
//  3. 回退到默认 LocalLow 路径（即使文件尚不存在）
func ResolvePath() (string, error) {
	if override := os.Getenv(EnvLogPath); override != "" {
		if st, err := os.Stat(override); err == nil && !st.IsDir() {
			return override, nil
		}
		return "", fmt.Errorf("MTGA_BOT_LOG_PATH 指向的文件不存在: %s", override)
	}

	candidates := detectCandidates()
	if len(candidates) > 0 {
		return candidates[0], nil
	}
	return defaultPlayerLogPath(), nil
}

func defaultPlayerLogPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = os.Getenv("USERPROFILE")
	}
	return filepath.Join(home, "AppData", "LocalLow", "Wizards Of The Coast", "MTGA", "Player.log")
}

// detectCandidates 返回存在的 Player.log，按 mtime 降序。
func detectCandidates() []string {
	var roots []string
	if up := os.Getenv("USERPROFILE"); up != "" {
		roots = append(roots, up)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dup := false
		for _, r := range roots {
			if r == home {
				dup = true
				break
			}
		}
		if !dup {
			roots = append(roots, home)
		}
	}

	type item struct {
		path  string
		mtime int64
	}
	var found []item
	for _, root := range roots {
		p := filepath.Join(root, "AppData", "LocalLow", "Wizards Of The Coast", "MTGA", "Player.log")
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			continue
		}
		found = append(found, item{path: p, mtime: st.ModTime().UnixNano()})
	}
	// 简单选择排序：mtime 降序
	for i := 0; i < len(found); i++ {
		best := i
		for j := i + 1; j < len(found); j++ {
			if found[j].mtime > found[best].mtime {
				best = j
			}
		}
		found[i], found[best] = found[best], found[i]
	}
	out := make([]string, len(found))
	for i, it := range found {
		out[i] = it.path
	}
	return out
}
