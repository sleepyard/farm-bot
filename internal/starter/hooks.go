package starter

import (
	"image"
	"time"
)

// Hooks 供 UI / 服务端注入：日志、预览图、停止检查。
type Hooks struct {
	Log        func(string)
	Preview    func(templateRel string, frame *image.RGBA)
	ShouldStop func() bool
}

func (n *navigator) stopped() bool {
	return n.hooks.ShouldStop != nil && n.hooks.ShouldStop()
}

func (n *navigator) publishPreview(rel string, frame *image.RGBA) {
	if n.hooks.Preview != nil && frame != nil {
		n.hooks.Preview(rel, frame)
	}
}

// sleep 可中断休眠；返回 true 表示被停止。
func (n *navigator) sleep(d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if n.stopped() {
			return true
		}
		left := time.Until(deadline)
		if left > 80*time.Millisecond {
			left = 80 * time.Millisecond
		}
		time.Sleep(left)
	}
	return n.stopped()
}
