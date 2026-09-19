package window

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// 参考空间固定为 1280x720（模板与点击坐标都按此空间表达）；
// 实际客户区接受任意 ≈16:9 且不小于 1024x576 的尺寸，截图在捕获时归一化、
// 点击在发送时换算，因此不再强制窗口恰好是 1280x720。
const RequiredLiveWidth = 1280
const RequiredLiveHeight = 720

const (
	minLiveWidth  = 1024
	minLiveHeight = 576
	// aspectTolerancePct 宽高比容差（%）：|w*9-h*16| ≤ h*16*pct%，
	// 让 1366x768 这类近似 16:9 通过。
	aspectTolerancePct = 3
)

const (
	processQueryLimitedInformation       = 0x1000
	dpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3) // (DPI_AWARENESS_CONTEXT)(-4)
)

var (
	modUser32   = windows.NewLazySystemDLL("user32.dll")
	modKernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procEnumWindows                  = modUser32.NewProc("EnumWindows")
	procIsWindowVisible              = modUser32.NewProc("IsWindowVisible")
	procIsIconic                     = modUser32.NewProc("IsIconic")
	procGetWindowTextLengthW         = modUser32.NewProc("GetWindowTextLengthW")
	procGetWindowTextW               = modUser32.NewProc("GetWindowTextW")
	procGetWindowThreadProcessId     = modUser32.NewProc("GetWindowThreadProcessId")
	procGetClientRect                = modUser32.NewProc("GetClientRect")
	procGetWindowRect                = modUser32.NewProc("GetWindowRect")
	procClientToScreen               = modUser32.NewProc("ClientToScreen")
	procSetThreadDpiAwarenessContext = modUser32.NewProc("SetThreadDpiAwarenessContext")

	procOpenProcess                = modKernel32.NewProc("OpenProcess")
	procQueryFullProcessImageNameW = modKernel32.NewProc("QueryFullProcessImageNameW")
	procCloseHandle                = modKernel32.NewProc("CloseHandle")
)

// Candidate 是一次窗口枚举得到的候选。
type Candidate struct {
	HWND       uintptr
	Title      string
	ClientRect Rect  // 客户区：后续截图 / 点击都以它为准
	WindowRect *Rect // 含边框的整个窗口矩形，仅作诊断
	Score      float64
}

// CaptureResult 是「捕获 MTGA」按钮的业务结果。
type CaptureResult struct {
	OK         bool
	Candidate  *Candidate
	Message    string
	Candidates []Candidate
}

// CaptureMTGA 查找并选中最合适的 MTGA 窗口。
// 成功条件：找到可见的 mtga.exe 窗口，且客户区为 ≈16:9 且不小于 1024x576。
func CaptureMTGA() CaptureResult {
	candidates, err := listMTGAWindows()
	if err != nil {
		return CaptureResult{
			OK:      false,
			Message: fmt.Sprintf("枚举窗口失败: %v", err),
		}
	}
	if len(candidates) == 0 {
		return CaptureResult{
			OK: false,
			Message: fmt.Sprintf(
				"未找到 MTGA 窗口。请以可见的窗口模式 %dx%d 打开 MTGA。",
				RequiredLiveWidth, RequiredLiveHeight,
			),
		}
	}

	best := pickBestCandidate(candidates, RequiredLiveWidth, RequiredLiveHeight)
	if !isSupportedLiveSize(best.ClientRect.W, best.ClientRect.H) {
		return CaptureResult{
			OK:         false,
			Candidate:  &best,
			Candidates: candidates,
			Message: fmt.Sprintf(
				"已找到 MTGA 窗口，当前分辨率为 %s，坐标为 %s。请使用 16:9 窗口分辨率（≥%dx%d），推荐 %dx%d。",
				best.ClientRect.Size(),
				best.ClientRect.Position(),
				minLiveWidth,
				minLiveHeight,
				RequiredLiveWidth,
				RequiredLiveHeight,
			),
		}
	}

	return CaptureResult{
		OK:         true,
		Candidate:  &best,
		Candidates: candidates,
		Message: fmt.Sprintf(
			"捕获成功，目标窗口分辨率为 %s，坐标为 %s",
			best.ClientRect.Size(),
			best.ClientRect.Position(),
		),
	}
}

// isSupportedLiveSize 判断客户区是否为受支持的直播尺寸：
// 宽高比 ≈16:9（容差 aspectTolerancePct%）且不小于 1024x576。
func isSupportedLiveSize(w, h int) bool {
	if w < minLiveWidth || h < minLiveHeight {
		return false
	}
	return abs(w*9-h*16)*100 <= h*16*aspectTolerancePct
}

func listMTGAWindows() ([]Candidate, error) {
	// 注意：EnumWindows 的回调跑在系统调用栈上，不要在回调里做复杂逻辑或 append 业务对象。
	// 先收集 hwnd，回到 Go 常规调用栈后再过滤/读矩形。
	var hwnds []uintptr
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		hwnds = append(hwnds, hwnd)
		return 1
	})

	r, _, err := procEnumWindows.Call(cb, 0)
	if r == 0 {
		return nil, fmt.Errorf("EnumWindows failed: %w", err)
	}

	out := make([]Candidate, 0)
	for _, hwnd := range hwnds {
		visible, _, _ := procIsWindowVisible.Call(hwnd)
		if visible == 0 {
			continue
		}
		iconic, _, _ := procIsIconic.Call(hwnd)
		if iconic != 0 {
			continue
		}

		title := windowTitle(hwnd)
		if title == "" || !isMTGATitle(title) {
			continue
		}

		// 标题像 MTGA，但进程不是游戏本体（浏览器 / Discord 等）——直接跳过。
		if exe, ok := processExeName(hwnd); ok && !strings.EqualFold(exe, "mtga.exe") {
			continue
		}

		client, ok := clientRectPhysical(hwnd)
		if !ok {
			continue
		}

		c := Candidate{
			HWND:       hwnd,
			Title:      title,
			ClientRect: client,
		}
		if winRect, ok := windowRectPhysical(hwnd); ok {
			r := winRect
			c.WindowRect = &r
		}
		out = append(out, c)
	}
	return out, nil
}

func isMTGATitle(title string) bool {
	low := strings.ToLower(title)
	return strings.Contains(low, "mtga") ||
		strings.Contains(low, "magic: the gathering arena") ||
		strings.Contains(low, "magic the gathering arena")
}

func pickBestCandidate(candidates []Candidate, expectW, expectH int) Candidate {
	bestIdx := 0
	bestScore := scoreCandidate(candidates[0], expectW, expectH)
	candidates[0].Score = bestScore

	for i := 1; i < len(candidates); i++ {
		s := scoreCandidate(candidates[i], expectW, expectH)
		candidates[i].Score = s
		if s > bestScore {
			bestScore = s
			bestIdx = i
		}
	}
	return candidates[bestIdx]
}

// scoreCandidate 与原版 _pick_best_windows_candidate 同一套启发式：
// 完整游戏标题 > 简称；精确 1280x720 额外加分；尺寸越接近越好。
func scoreCandidate(c Candidate, expectW, expectH int) float64 {
	title := strings.ToLower(c.Title)
	sizePenalty := abs(c.ClientRect.W-expectW) + abs(c.ClientRect.H-expectH)

	titleBonus := 0.0
	switch {
	case strings.Contains(title, "magic: the gathering arena"):
		titleBonus = 20
	case strings.Contains(title, "magic the gathering arena"):
		titleBonus = 18
	case strings.Contains(title, "mtga"):
		titleBonus = 12
	}

	exactBonus := 0.0
	if c.ClientRect.W == expectW && c.ClientRect.H == expectH {
		exactBonus = 30
	}
	closeness := max(0.0, 25.0-float64(sizePenalty)/10.0)
	return titleBonus + exactBonus + closeness
}

func windowTitle(hwnd uintptr) string {
	length, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if length == 0 {
		return ""
	}
	buf := make([]uint16, length+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return strings.TrimSpace(windows.UTF16ToString(buf))
}

func processExeName(hwnd uintptr) (string, bool) {
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return "", false
	}

	handle, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if handle == 0 {
		return "", false
	}
	defer procCloseHandle.Call(handle)

	buf := make([]uint16, 260)
	size := uint32(len(buf))
	ok, _, _ := procQueryFullProcessImageNameW.Call(
		handle,
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ok == 0 {
		return "", false
	}
	full := windows.UTF16ToString(buf[:size])
	return filepath.Base(full), true
}

type winPoint struct {
	X, Y int32
}

type winRect struct {
	Left, Top, Right, Bottom int32
}

func clientRectPhysical(hwnd uintptr) (Rect, bool) {
	var out Rect
	var ok bool
	withPhysicalDPI(func() {
		var rc winRect
		r, _, _ := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		if r == 0 {
			return
		}
		tl := winPoint{X: rc.Left, Y: rc.Top}
		br := winPoint{X: rc.Right, Y: rc.Bottom}
		if r, _, _ := procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&tl))); r == 0 {
			return
		}
		if r, _, _ := procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&br))); r == 0 {
			return
		}
		w := int(br.X - tl.X)
		h := int(br.Y - tl.Y)
		if w <= 0 || h <= 0 {
			return
		}
		out = Rect{X: int(tl.X), Y: int(tl.Y), W: w, H: h}
		ok = true
	})
	return out, ok
}

func windowRectPhysical(hwnd uintptr) (Rect, bool) {
	var out Rect
	var ok bool
	withPhysicalDPI(func() {
		var rc winRect
		r, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		if r == 0 {
			return
		}
		w := int(rc.Right - rc.Left)
		h := int(rc.Bottom - rc.Top)
		if w <= 0 || h <= 0 {
			return
		}
		out = Rect{X: int(rc.Left), Y: int(rc.Top), W: w, H: h}
		ok = true
	})
	return out, ok
}

// withPhysicalDPI 临时将当前线程切到 Per-Monitor DPI Aware V2，
// 使 GetClientRect / ClientToScreen 返回物理像素（与截图、鼠标坐标一致），
// 然后恢复原 DPI 上下文，避免影响 UI 框架自身的布局。
func withPhysicalDPI(fn func()) {
	if procSetThreadDpiAwarenessContext.Find() != nil {
		fn()
		return
	}
	prev, _, _ := procSetThreadDpiAwarenessContext.Call(dpiAwarenessContextPerMonitorAwareV2)
	defer procSetThreadDpiAwarenessContext.Call(prev)
	fn()
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
