package decisioner

// ShouldKeep 起手是否保留。首切片与原版 DummyAI.generate_keep 一致：始终保留。
func ShouldKeep(_ []int) bool {
	return true
}

// KeepMove 返回保留手牌指令。
func KeepMove() Move {
	return Move{Kind: KindKeep, Reason: "起手保留"}
}
