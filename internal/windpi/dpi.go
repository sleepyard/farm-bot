// Package windpi 提供与原版一致的物理像素 DPI 上下文。
//
// DPI 未感知进程里，GetClientRect / ClientToScreen / SetCursorPos 使用虚拟坐标，
// 而桌面 BitBlt 按物理像素取样，会导致「匹配点」和「鼠标点击」错位。
package windpi

import "golang.org/x/sys/windows"

const dpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3) // (DPI_AWARENESS_CONTEXT)(-4)

var (
	modUser32                         = windows.NewLazySystemDLL("user32.dll")
	procSetThreadDpiAwarenessContext  = modUser32.NewProc("SetThreadDpiAwarenessContext")
	procSetProcessDpiAwarenessContext = modUser32.NewProc("SetProcessDpiAwarenessContext")
)

// EnableProcessPhysical 进程级启用 Per-Monitor V2（启动时调用一次）。
func EnableProcessPhysical() {
	if procSetProcessDpiAwarenessContext.Find() == nil {
		_, _, _ = procSetProcessDpiAwarenessContext.Call(dpiAwarenessContextPerMonitorAwareV2)
	}
}

// WithPhysical 在 Per-Monitor V2 下执行 fn，结束后恢复原上下文。
func WithPhysical(fn func()) {
	if procSetThreadDpiAwarenessContext.Find() != nil {
		fn()
		return
	}
	prev, _, _ := procSetThreadDpiAwarenessContext.Call(dpiAwarenessContextPerMonitorAwareV2)
	defer procSetThreadDpiAwarenessContext.Call(prev)
	fn()
}
