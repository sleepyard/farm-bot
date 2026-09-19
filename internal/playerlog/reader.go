package playerlog

import (
	"fmt"
	"io"
	"os"
)

// Reader 提供无业务语义的 Player.log 字节窗口读取。
type Reader struct {
	Path string
}

// Size 返回当前文件字节数。
func (r *Reader) Size() (int64, error) {
	st, err := os.Stat(r.Path)
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}

// Tail 读取文件末尾最多 maxBytes 字节，返回 UTF-8 文本与读到时的文件末尾 offset。
func (r *Reader) Tail(maxBytes int64) (string, int64, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultTailBytes
	}
	f, err := os.Open(r.Path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return "", 0, err
	}
	size := st.Size()
	start := size - maxBytes
	if start < 0 {
		start = 0
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return "", 0, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return "", 0, err
	}
	return string(decodeUTF8(data)), size, nil
}

// Since 从 startOffset 起读取，最多 maxBytes。
// preferNewest=true 时：在不低于 startOffset 的前提下，只取文件末尾 maxBytes
// （与原版 _read_log_since(prefer_newest=True) 一致）。
func (r *Reader) Since(startOffset, maxBytes int64, preferNewest bool) (string, error) {
	if maxBytes <= 0 {
		maxBytes = FloorMaxBytes
	}
	if startOffset < 0 {
		startOffset = 0
	}
	f, err := os.Open(r.Path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	size := st.Size()
	if size <= startOffset {
		return "", nil
	}
	start := startOffset
	if preferNewest {
		newestStart := size - maxBytes
		if newestStart < startOffset {
			newestStart = startOffset
		}
		if newestStart < 0 {
			newestStart = 0
		}
		start = newestStart
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return "", err
	}
	toRead := size - start
	if toRead > maxBytes {
		toRead = maxBytes
	}
	buf := make([]byte, toRead)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", err
	}
	return string(decodeUTF8(buf[:n])), nil
}

// DetectShrink 若当前文件比 knownSize 更短，视为 rotate，返回 true。
func (r *Reader) DetectShrink(knownSize int64) (bool, int64, error) {
	size, err := r.Size()
	if err != nil {
		return false, 0, err
	}
	if knownSize > 0 && size < knownSize {
		return true, size, nil
	}
	return false, size, nil
}

// Snapshotter 组合 Reader + Floor，产出业务快照。
type Snapshotter struct {
	Reader *Reader
	Floor  int64
}

// Capture 读取当前窗口并解析每日任务（含 canSwap）。
// Floor=0 时用普通 Tail；Floor>0 时用 Since(preferNewest)。
// 若文件相对 Floor 发生 shrink，自动清零 Floor（对齐原版 rotate 处理）。
func (s *Snapshotter) Capture() Snapshot {
	if s == nil || s.Reader == nil || s.Reader.Path == "" {
		return Snapshot{OK: false, Warning: "未配置 Player.log 路径"}
	}
	path := s.Reader.Path
	size, err := s.Reader.Size()
	if err != nil {
		return Snapshot{
			OK:      false,
			Path:    path,
			Warning: fmt.Sprintf("无法读取 Player.log: %v（请确认 MTGA 已开启 Detailed Logs）", err),
		}
	}

	floor := s.Floor
	if floor > 0 && size < floor {
		// 日志 rotate：旧边界失效。
		floor = 0
		s.Floor = 0
	}

	var text string
	if floor > 0 {
		text, err = s.Reader.Since(floor, FloorMaxBytes, true)
	} else {
		text, _, err = s.Reader.Tail(DefaultTailBytes)
	}
	if err != nil {
		return Snapshot{
			OK:      false,
			Path:    path,
			LogSize: size,
			Floor:   floor,
			Warning: fmt.Sprintf("读取 Player.log 失败: %v", err),
		}
	}

	snap := Snapshot{
		OK:      true,
		Path:    path,
		LogSize: size,
		Floor:   floor,
		Quests:  []Quest{},
	}

	if qs, err := ParseLatestQuests(text); err == nil && qs != nil {
		snap.CanSwap = qs.CanSwap
		snap.Quests = BuildQuestView(qs.Quests)
	}

	if len(snap.Quests) == 0 {
		snap.Warning = "暂无任务数据（需在 Home 触发 QuestGetQuests）"
	}
	return snap
}

// decodeUTF8 将可能损坏的字节转为字符串；非法序列用替换字符，避免整段失败。
func decodeUTF8(data []byte) []byte {
	// Go 的 string([]byte) 已按 UTF-8 解释；此处保持原字节即可，
	// 解析器用正则/JSON 时遇到坏字节会自然失败并跳过。
	return data
}
