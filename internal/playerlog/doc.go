// Package playerlog 负责定位并按需解析 MTGA 的 Player.log。
//
// 当前只做任务快照：QuestGetQuests（含 canSwap）。
// 不解析账户名 / 金币；不含 GRE 对局实时跟随。
package playerlog

// DefaultTailBytes 普通尾部读取窗口（与原版 Controller._read_log_tail 对齐）。
// QuestGetQuests 通常写在文件末尾，用这个窗口即可。
const DefaultTailBytes int64 = 600_000

// FloorMaxBytes 带 floor 时的读取上限（与原版 prefer_newest 窗口对齐）。
const FloorMaxBytes int64 = 2_000_000

// EnvLogPath 环境变量覆盖 Player.log 路径。
const EnvLogPath = "MTGA_BOT_LOG_PATH"
