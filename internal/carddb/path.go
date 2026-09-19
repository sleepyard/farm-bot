package carddb

import (
	"os"
	"path/filepath"
	"strings"
)

// EnvRawDir 可覆盖 MTGA Raw 数据目录。
const EnvRawDir = "MTGA_BOT_DATA_DIR"

// ResolveRawDir 按优先级查找含 Raw_CardDatabase*.mtga / data_cards*.mtga 的目录。
// 环境变量 → 页面里记住的目录 → 常见安装路径。
func ResolveRawDir() (string, error) {
	if override := strings.TrimSpace(os.Getenv(EnvRawDir)); override != "" {
		if looksLikeRawDir(override) {
			return override, nil
		}
		if n, err := NormalizePickedRawDir(override); err == nil {
			return n, nil
		}
	}
	if saved := LoadSavedRawDir(); saved != "" {
		if looksLikeRawDir(saved) {
			return saved, nil
		}
		if n, err := NormalizePickedRawDir(saved); err == nil {
			return n, nil
		}
	}
	for _, c := range defaultRawCandidates() {
		if looksLikeRawDir(c) {
			return c, nil
		}
	}
	return "", os.ErrNotExist
}

// NormalizePickedRawDir 把用户选的文件夹收成真正的 Raw 目录。
// 允许选 MTGA 根、MTGA_Data、Downloads，或已经是 Raw。
func NormalizePickedRawDir(picked string) (string, error) {
	picked = strings.TrimSpace(picked)
	if picked == "" {
		return "", os.ErrNotExist
	}
	cands := []string{
		picked,
		filepath.Join(picked, "Raw"),
		filepath.Join(picked, "Downloads", "Raw"),
		filepath.Join(picked, "MTGA_Data", "Downloads", "Raw"),
		filepath.Join(picked, "MTGA", "MTGA_Data", "Downloads", "Raw"),
	}
	for _, c := range cands {
		if looksLikeRawDir(c) {
			return c, nil
		}
	}
	return "", os.ErrNotExist
}

func defaultRawCandidates() []string {
	var out []string
	fixed := []string{
		`D:\Steam\steamapps\common\MTGA\MTGA_Data\Downloads\Raw`,
		`C:\Program Files (x86)\Steam\steamapps\common\MTGA\MTGA_Data\Downloads\Raw`,
		`C:\Program Files\Steam\steamapps\common\MTGA\MTGA_Data\Downloads\Raw`,
		`C:\Program Files\Wizards of the Coast\MTGA\MTGA_Data\Downloads\Raw`,
	}
	out = append(out, fixed...)
	for _, drive := range []string{"C", "D", "E", "F"} {
		out = append(out,
			drive+`:\Steam\steamapps\common\MTGA\MTGA_Data\Downloads\Raw`,
			drive+`:\Program Files (x86)\Steam\steamapps\common\MTGA\MTGA_Data\Downloads\Raw`,
			drive+`:\Program Files\Steam\steamapps\common\MTGA\MTGA_Data\Downloads\Raw`,
		)
	}
	return uniqueStrings(out)
}

func looksLikeRawDir(dir string) bool {
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return false
	}
	p, err := FindLatestSource(dir)
	return err == nil && p != ""
}

// FindLatestSource 在 Raw 目录中选最新的卡牌库文件。
func FindLatestSource(rawDir string) (string, error) {
	patterns := []string{"Raw_CardDatabase*.mtga", "data_cards*.mtga"}
	var best string
	var bestMtime int64
	var bestName string
	for _, pat := range patterns {
		matches, err := filepath.Glob(filepath.Join(rawDir, pat))
		if err != nil {
			continue
		}
		for _, m := range matches {
			st, err := os.Stat(m)
			if err != nil || st.IsDir() {
				continue
			}
			mt := st.ModTime().UnixNano()
			name := filepath.Base(m)
			if best == "" || mt > bestMtime || (mt == bestMtime && name > bestName) {
				best = m
				bestMtime = mt
				bestName = name
			}
		}
	}
	if best == "" {
		return "", os.ErrNotExist
	}
	return best, nil
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
