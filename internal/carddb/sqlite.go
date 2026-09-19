package carddb

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

var colorSymbolMap = map[string]string{
	"White": "W",
	"Blue":  "U",
	"Black": "B",
	"Red":   "R",
	"Green": "G",
}

var rarityMap = map[int]string{
	1: "Common",
	2: "Uncommon",
	3: "Rare",
	4: "Mythic",
	5: "Special",
}

// ParseSource 解析卡牌源文件（当前仅支持 SQLite .mtga）。
func ParseSource(path string) ([]Card, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	header := make([]byte, 16)
	n, _ := f.Read(header)
	_ = f.Close()
	if n >= 15 && string(header[:15]) == "SQLite format 3" {
		return parseSQLite(path)
	}
	return nil, fmt.Errorf("不支持的卡牌文件格式（需要 SQLite）: %s", path)
}

func parseSQLite(path string) ([]Card, error) {
	// 只读打开，避免锁住游戏正在用的文件。
	dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=query_only(1)", filepathToURI(path))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite 失败: %w", err)
	}
	defer db.Close()

	colorMap, err := readEnumTextMap(db, "Color")
	if err != nil {
		return nil, err
	}
	typeMap, err := readEnumTextMap(db, "CardType")
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT c.GrpId, c.TitleId, c.OldSchoolManaText, c.Colors, c.Types, c.ExpansionCode, c.Rarity, l.Loc
		FROM Cards c
		LEFT JOIN Localizations_enUS l ON l.LocId = c.TitleId
	`)
	if err != nil {
		return nil, fmt.Errorf("读取 Cards 表失败: %w", err)
	}
	defer rows.Close()

	out := make([]Card, 0, 30000)
	for rows.Next() {
		var (
			grpID, titleID sql.NullInt64
			manaRaw        sql.NullString
			colorsRaw      sql.NullString
			typesRaw       sql.NullString
			setCode        sql.NullString
			rarityRaw      sql.NullInt64
			nameRaw        sql.NullString
		)
		if err := rows.Scan(&grpID, &titleID, &manaRaw, &colorsRaw, &typesRaw, &setCode, &rarityRaw, &nameRaw); err != nil {
			return nil, err
		}
		var c Card
		if grpID.Valid {
			c.GrpID = int(grpID.Int64)
		}
		if titleID.Valid {
			c.TitleID = int(titleID.Int64)
		}
		if nameRaw.Valid {
			c.Name = strings.TrimSpace(nameRaw.String)
		}
		if m := normalizeManaCost(manaRaw.String); m != "" {
			c.ManaCost = m
		}
		c.Colors = parseIDList(colorsRaw.String, func(id int) string {
			name := colorMap[id]
			if sym, ok := colorSymbolMap[name]; ok {
				return sym
			}
			if name != "" {
				return name
			}
			return strconv.Itoa(id)
		})
		c.Types = parseIDList(typesRaw.String, func(id int) string {
			if t, ok := typeMap[id]; ok {
				return t
			}
			return strconv.Itoa(id)
		})
		if setCode.Valid {
			c.SetCode = setCode.String
		}
		if rarityRaw.Valid {
			if r, ok := rarityMap[int(rarityRaw.Int64)]; ok {
				c.Rarity = r
			} else {
				c.Rarity = strconv.FormatInt(rarityRaw.Int64, 10)
			}
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func readEnumTextMap(db *sql.DB, enumType string) (map[int]string, error) {
	rows, err := db.Query(`
		SELECT e.Value, l.Loc
		FROM Enums e
		JOIN Localizations_enUS l ON l.LocId = e.LocId
		WHERE e.Type = ?
	`, enumType)
	if err != nil {
		return nil, fmt.Errorf("读取枚举 %s 失败: %w", enumType, err)
	}
	defer rows.Close()
	out := make(map[int]string)
	for rows.Next() {
		var value int
		var loc string
		if err := rows.Scan(&value, &loc); err != nil {
			return nil, err
		}
		out[value] = loc
	}
	return out, rows.Err()
}

func normalizeManaCost(mana string) string {
	mana = strings.TrimSpace(mana)
	if mana == "" {
		return ""
	}
	parts := strings.Split(mana, "o")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteByte('{')
		b.WriteString(p)
		b.WriteByte('}')
	}
	return b.String()
}

func parseIDList(raw string, mapID func(int) string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.Atoi(part)
		if err != nil {
			out = append(out, part)
			continue
		}
		out = append(out, mapID(id))
	}
	return out
}

// filepathToURI 把 Windows 路径转成 sqlite URI 可用形式。
func filepathToURI(path string) string {
	// modernc.org/sqlite 接受正斜杠路径。
	p := strings.ReplaceAll(path, `\`, `/`)
	return p
}
