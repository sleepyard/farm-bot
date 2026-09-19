package decisioner

// unsupportedCast 等牌表策略占位（原版 CardPolicy / LegendRule）。
func unsupportedCast(grpID int) bool {
	_ = grpID
	return false
}
