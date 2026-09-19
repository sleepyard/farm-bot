package carddb

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type metadata struct {
	SourceFilename string  `json:"source_filename"`
	SourceMTime    float64 `json:"source_mtime"`
	SourceSHA256   string  `json:"source_sha256"`
	SchemaVersion  int     `json:"schema_version"`
}

// exportSchema 缓存 schema：2 = 含英文牌名 (name)。
const exportSchema = 2

// ExportIfNeeded 若源文件变更则重新导出 cards.json；否则跳过。
// 返回 (卡牌列表或从缓存加载的列表, skipped, sourcePath, error)。
func ExportIfNeeded(rawDir, cacheDir string) (cards []Card, skipped bool, source string, err error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, false, "", fmt.Errorf("创建缓存目录失败: %w", err)
	}
	source, err = FindLatestSource(rawDir)
	if err != nil {
		return nil, false, "", fmt.Errorf("未找到卡牌库文件: %w", err)
	}

	st, err := os.Stat(source)
	if err != nil {
		return nil, false, source, err
	}
	mtime := float64(st.ModTime().UnixNano()) / 1e9
	hash, err := fileSHA256(source)
	if err != nil {
		return nil, false, source, err
	}

	cardsPath := filepath.Join(cacheDir, "cards.json")
	metaPath := filepath.Join(cacheDir, "cards_metadata.json")
	if meta, ok := loadMetadata(metaPath); ok {
		if meta.SchemaVersion == exportSchema &&
			meta.SourceFilename == filepath.Base(source) &&
			almostEqual(meta.SourceMTime, mtime) &&
			meta.SourceSHA256 == hash {
			if _, err := os.Stat(cardsPath); err == nil {
				cards, err := loadCardsJSON(cardsPath)
				if err == nil && cardsHaveNames(cards) {
					return cards, true, source, nil
				}
			}
		}
	}

	cards, err = ParseSource(source)
	if err != nil {
		return nil, false, source, err
	}
	if err := writeCardsJSON(cardsPath, cards); err != nil {
		return nil, false, source, err
	}
	meta := metadata{
		SourceFilename: filepath.Base(source),
		SourceMTime:    mtime,
		SourceSHA256:   hash,
		SchemaVersion:  exportSchema,
	}
	if err := writeMetadata(metaPath, meta); err != nil {
		return nil, false, source, err
	}
	return cards, false, source, nil
}

func cardsHaveNames(cards []Card) bool {
	named := 0
	for _, c := range cards {
		if c.Name != "" {
			named++
			if named >= 10 {
				return true
			}
		}
	}
	return named > 0
}

// LoadFromCache 仅从 cards.json 载入（不访问 MTGA Raw）。
func LoadFromCache(cacheDir string) ([]Card, error) {
	return loadCardsJSON(filepath.Join(cacheDir, "cards.json"))
}

// RefreshAndLoad 导出（如需要）并载入全局内存索引。
func RefreshAndLoad(rawDir, cacheDir string) LoadResult {
	if cacheDir == "" {
		cacheDir = DefaultCacheDir()
	}
	if rawDir == "" {
		var err error
		rawDir, err = ResolveRawDir()
		if err != nil {
			return LoadResult{
				OK:       false,
				CacheDir: cacheDir,
				Message:  "未找到 MTGA 卡牌数据目录（应含 Raw_CardDatabase*.mtga）。请在页面点击「选择卡牌数据目录」，选到游戏安装目录下的 MTGA_Data\\Downloads\\Raw。",
			}
		}
	}

	cards, skipped, source, err := ExportIfNeeded(rawDir, cacheDir)
	if err != nil {
		return LoadResult{
			OK:       false,
			RawDir:   rawDir,
			Source:   source,
			CacheDir: cacheDir,
			Message:  err.Error(),
		}
	}

	db := &DB{byGrp: make(map[int]Card, len(cards))}
	for _, c := range cards {
		if c.GrpID != 0 {
			db.byGrp[c.GrpID] = c
		}
	}
	setGlobal(db)

	msg := fmt.Sprintf("卡牌数据已加载：%d 张", db.Len())
	if skipped {
		msg = fmt.Sprintf("卡牌缓存未变更，已载入 %d 张", db.Len())
	}
	return LoadResult{
		OK:       true,
		Count:    db.Len(),
		Skipped:  skipped,
		RawDir:   rawDir,
		Source:   filepath.Base(source),
		CacheDir: cacheDir,
		Message:  msg,
	}
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func loadMetadata(path string) (metadata, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return metadata{}, false
	}
	var m metadata
	if err := json.Unmarshal(b, &m); err != nil {
		return metadata{}, false
	}
	return m, true
}

func writeMetadata(path string, m metadata) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func loadCardsJSON(path string) ([]Card, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cards []Card
	if err := json.Unmarshal(b, &cards); err != nil {
		return nil, err
	}
	return cards, nil
}

func writeCardsJSON(path string, cards []Card) error {
	tmp := path + ".tmp"
	b, err := json.MarshalIndent(cards, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func almostEqual(a, b float64) bool {
	const eps = 1e-6
	if a > b {
		return a-b < eps
	}
	return b-a < eps
}
