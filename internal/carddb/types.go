// Package carddb 从本机 MTGA Raw_CardDatabase*.mtga（SQLite）导出并加载卡牌索引。
package carddb

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Card 是导出后的精简卡牌记录。
type Card struct {
	GrpID    int      `json:"grpId"`
	TitleID  int      `json:"titleId,omitempty"`
	Name     string   `json:"name,omitempty"` // Localizations_enUS
	ManaCost string   `json:"manaCost,omitempty"`
	Colors   []string `json:"colors,omitempty"`
	Types    []string `json:"types,omitempty"`
	SetCode  string   `json:"setCode,omitempty"`
	Rarity   string   `json:"rarity,omitempty"`
}

// DB 是 grpId → Card 的内存索引。
type DB struct {
	byGrp map[int]Card
}

// Get 按 grpId 查找卡牌。
func (d *DB) Get(grpID int) (Card, bool) {
	if d == nil || d.byGrp == nil {
		return Card{}, false
	}
	c, ok := d.byGrp[grpID]
	return c, ok
}

// NameByGrp 按 grpId 取英文牌名；未知则空串。
func NameByGrp(grpID int) string {
	if grpID <= 0 {
		return ""
	}
	db := Global()
	if db == nil {
		return ""
	}
	c, ok := db.Get(grpID)
	if !ok {
		return ""
	}
	return c.Name
}

// FormatInstance 统一日志片段：instance=219||name=Plains
func FormatInstance(instanceID, grpID int) string {
	name := NameByGrp(grpID)
	if name == "" {
		name = "?"
	}
	return fmt.Sprintf("instance=%d||name=%s", instanceID, name)
}

// Len 返回已加载卡牌数量。
func (d *DB) Len() int {
	if d == nil || d.byGrp == nil {
		return 0
	}
	return len(d.byGrp)
}

var (
	globalMu sync.RWMutex
	globalDB *DB
)

// Global 返回当前进程内已加载的卡牌库（可能为 nil）。
func Global() *DB {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalDB
}

func setGlobal(db *DB) {
	globalMu.Lock()
	globalDB = db
	globalMu.Unlock()
}

// LoadResult 是一次加载/刷新的结果，供 API 返回。
type LoadResult struct {
	OK       bool   `json:"ok"`
	Count    int    `json:"count"`
	Skipped  bool   `json:"skipped"` // 缓存未变，跳过重导
	RawDir   string `json:"raw_dir,omitempty"`
	Source   string `json:"source,omitempty"`
	CacheDir string `json:"cache_dir,omitempty"`
	Message  string `json:"message"`
}

// DefaultCacheDir 返回 exe 旁的缓存路径（./runtime/cache）。
func DefaultCacheDir() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "runtime", "cache")
	}
	return filepath.Join("runtime", "cache")
}
