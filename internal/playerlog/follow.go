package playerlog

import (
	"context"
	"time"
)

// FollowHooks 跟随日志时的回调。
type FollowHooks struct {
	OnChunk func(chunk string)
	Log     func(string)
	Stop    func() bool
}

// Follow 从 startOffset 起轮询追加内容，直到 ctx 取消或 Stop。
// 返回最后读到的文件字节 offset。
func (r *Reader) Follow(ctx context.Context, startOffset int64, poll time.Duration, hooks FollowHooks) (int64, error) {
	if poll <= 0 {
		poll = 400 * time.Millisecond
	}
	offset := startOffset
	if offset < 0 {
		if sz, err := r.Size(); err == nil {
			offset = sz
		} else {
			offset = 0
		}
	}
	ticker := time.NewTicker(poll)
	defer ticker.Stop()

	for {
		if hooks.Stop != nil && hooks.Stop() {
			return offset, nil
		}
		select {
		case <-ctx.Done():
			return offset, ctx.Err()
		case <-ticker.C:
			sz, err := r.Size()
			if err != nil {
				return offset, err
			}
			if sz < offset {
				// 日志轮转
				offset = 0
			}
			if sz <= offset {
				continue
			}
			chunk, err := r.Since(offset, FloorMaxBytes, false)
			if err != nil {
				return offset, err
			}
			advanced := sz - offset
			if advanced > FloorMaxBytes {
				advanced = FloorMaxBytes
			}
			offset += advanced
			if chunk != "" && hooks.OnChunk != nil {
				hooks.OnChunk(chunk)
			}
		}
	}
}
