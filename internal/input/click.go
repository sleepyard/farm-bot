// Package input 提供相对 MTGA 客户区的鼠标点击（Win32 SendInput）。
// 所有公开函数的入参坐标都是 1280×720 参考空间；发送前在函数入口按
// 客户区实际尺寸换算（参考→实际），与 vision 捕获时的归一化互为边界。
package input

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/flourbrain/mtga-farm-bot/internal/windpi"
	"golang.org/x/sys/windows"
)

var (
	modUser32   = windows.NewLazySystemDLL("user32.dll")
	modKernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procSetForegroundWindow      = modUser32.NewProc("SetForegroundWindow")
	procSetCursorPos             = modUser32.NewProc("SetCursorPos")
	procGetCursorPos             = modUser32.NewProc("GetCursorPos")
	procSendInput                = modUser32.NewProc("SendInput")
	procClientToScreen           = modUser32.NewProc("ClientToScreen")
	procGetClientRect            = modUser32.NewProc("GetClientRect")
	procShowWindow               = modUser32.NewProc("ShowWindow")
	procSetWindowPos             = modUser32.NewProc("SetWindowPos")
	procGetForegroundWindow      = modUser32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId = modUser32.NewProc("GetWindowThreadProcessId")
	procAttachThreadInput        = modUser32.NewProc("AttachThreadInput")
	procBringWindowToTop         = modUser32.NewProc("BringWindowToTop")
	procGetSystemMetrics         = modUser32.NewProc("GetSystemMetrics")
	procGetCurrentThreadId       = modKernel32.NewProc("GetCurrentThreadId")
)

const (
	inputMouse            = 0
	mouseEventMove        = 0x0001
	mouseEventLeftDown    = 0x0002
	mouseEventLeftUp      = 0x0004
	mouseEventWheel       = 0x0800
	mouseEventVirtualDesk = 0x4000
	mouseEventAbsolute    = 0x8000
	swRestore             = 9
	hwndTopMost           = ^uintptr(0)
	hwndNoTopMost         = ^uintptr(1)
	swpNoMove             = 0x0002
	swpNoSize             = 0x0001
	swpShowWindow         = 0x0040
	smXVirtualScreen      = 76
	smYVirtualScreen      = 77
	smCXVirtualScreen     = 78
	smCYVirtualScreen     = 79
	smCXScreen            = 0
	smCYScreen            = 1
)

// mouseInput 对齐 64 位 Windows INPUT{type, MOUSEINPUT}。
type mouseInput struct {
	Type      uint32
	_pad      uint32
	dx        int32
	dy        int32
	mouseData uint32
	flags     uint32
	time      uint32
	_pad2     uint32
	extraInfo uintptr
}

type point struct {
	X, Y int32
}

type winRect struct {
	Left, Top, Right, Bottom int32
}

// 参考空间尺寸：全代码库的 ROI / 模板 / 点击坐标统一在此空间表达。
const (
	refWidth  = 1280
	refHeight = 720
)

// refToActual 纯换算：参考空间 (1280×720) → 实际客户区像素（四舍五入）。
func refToActual(x, y, clientW, clientH int) (int, int) {
	if clientW <= 0 || clientH <= 0 {
		return x, y
	}
	return (x*clientW + refWidth/2) / refWidth,
		(y*clientH + refHeight/2) / refHeight
}

// refClientToActual 读 hwnd 客户区实际尺寸并把参考坐标换算成实际坐标。
// 须在物理 DPI 上下文（windpi.WithPhysical）内调用。
func refClientToActual(hwnd uintptr, cx, cy int) (int, int, error) {
	var rc winRect
	r, _, e := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	if r == 0 {
		return 0, 0, fmt.Errorf("GetClientRect: %v", e)
	}
	w := int(rc.Right - rc.Left)
	h := int(rc.Bottom - rc.Top)
	if w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("客户区尺寸无效: %dx%d", w, h)
	}
	ax, ay := refToActual(cx, cy, w, h)
	return ax, ay, nil
}

// Focus 将窗口置于前台。
func Focus(hwnd uintptr) {
	windpi.WithPhysical(func() {
		procShowWindow.Call(hwnd, swRestore)
		forceForeground(hwnd)
	})
	time.Sleep(60 * time.Millisecond)
}

func forceForeground(hwnd uintptr) {
	fg, _, _ := procGetForegroundWindow.Call()
	if fg == hwnd {
		procBringWindowToTop.Call(hwnd)
		procSetForegroundWindow.Call(hwnd)
		return
	}
	var fgTid uintptr
	if fg != 0 {
		fgTid, _, _ = procGetWindowThreadProcessId.Call(fg, 0)
	}
	curTid, _, _ := procGetCurrentThreadId.Call()
	if fgTid != 0 && fgTid != curTid {
		procAttachThreadInput.Call(curTid, fgTid, 1)
		defer procAttachThreadInput.Call(curTid, fgTid, 0)
	}
	procBringWindowToTop.Call(hwnd)
	procSetForegroundWindow.Call(hwnd)
}

func raiseTopMost(hwnd uintptr) {
	flags := uintptr(swpNoMove | swpNoSize | swpShowWindow)
	procSetWindowPos.Call(hwnd, hwndTopMost, 0, 0, 0, 0, flags)
}

func clearTopMost(hwnd uintptr) {
	flags := uintptr(swpNoMove | swpNoSize | swpShowWindow)
	procSetWindowPos.Call(hwnd, hwndNoTopMost, 0, 0, 0, 0, flags)
}

// ClickResult 描述一次点击的坐标换算。
type ClickResult struct {
	ClientX, ClientY int
	ScreenX, ScreenY int
	CursorX, CursorY int // GetCursorPos 校正后读回
	VirtX, VirtY     int
	VirtW, VirtH     int
}

// ClickClient 点击客户区坐标（模板匹配中心，1280×720 参考空间）。
func ClickClient(hwnd uintptr, cx, cy int) error {
	_, err := ClickClientEx(hwnd, cx, cy)
	return err
}

// MoveClient 仅移动光标到客户区坐标（手牌悬停扫描用，不点击）。
func MoveClient(hwnd uintptr, cx, cy int) error {
	var err error
	FocusIfNeeded(hwnd)
	windpi.WithPhysical(func() {
		ax, ay, e := refClientToActual(hwnd, cx, cy)
		if e != nil {
			err = e
			return
		}
		err = moveClientPhysical(hwnd, ax, ay)
	})
	return err
}

// LeftClick 在当前光标位置按下/抬起左键（不加 Focus / 不移动），用于双击第二下。
func LeftClick() error {
	if err := sendMouseAbs(0, 0, mouseEventLeftDown, 0); err != nil {
		return err
	}
	time.Sleep(40 * time.Millisecond)
	return sendMouseAbs(0, 0, mouseEventLeftUp, 0)
}

// DoubleClickClient 对齐 Python cast：移到目标 → 停稳 → 原地连点两下（中间不再 Focus/置顶）。
func DoubleClickClient(hwnd uintptr, cx, cy int) error {
	FocusIfNeeded(hwnd)
	var err error
	windpi.WithPhysical(func() {
		ax, ay, e := refClientToActual(hwnd, cx, cy)
		if e != nil {
			err = e
			return
		}
		if e := moveClientPhysical(hwnd, ax, ay); e != nil {
			err = e
			return
		}
		time.Sleep(500 * time.Millisecond) // Python cast 悬停后 0.5s 再点
		if e := LeftClick(); e != nil {
			err = e
			return
		}
		time.Sleep(100 * time.Millisecond)
		err = LeftClick()
	})
	return err
}

// FocusIfNeeded 仅在 MTGA 不在前台时抢焦点（Python：多余激活会让 Unity 丢悬停）。
func FocusIfNeeded(hwnd uintptr) {
	fg, _, _ := procGetForegroundWindow.Call()
	if fg == hwnd {
		return
	}
	Focus(hwnd)
}

func moveClientPhysical(hwnd uintptr, cx, cy int) error {
	var pt point
	pt.X = int32(cx)
	pt.Y = int32(cy)
	r, _, e := procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&pt)))
	if r == 0 {
		return fmt.Errorf("ClientToScreen: %v", e)
	}
	vx, vy, vw, vh := virtualDesktop()
	absX, absY := normalise(int(pt.X), int(pt.Y), vx, vy, vw, vh)
	if err := sendMouseAbs(absX, absY, mouseEventMove|mouseEventAbsolute|mouseEventVirtualDesk, 0); err != nil {
		return err
	}
	procSetCursorPos.Call(uintptr(pt.X), uintptr(pt.Y))
	return nil
}

// ClickClientEx 同 ClickClient，并返回屏幕坐标。ClientX/Y 记录入参（参考空间）。
func ClickClientEx(hwnd uintptr, cx, cy int) (ClickResult, error) {
	var out ClickResult
	var err error
	out.ClientX, out.ClientY = cx, cy

	Focus(hwnd)
	windpi.WithPhysical(func() {
		raiseTopMost(hwnd)
		time.Sleep(50 * time.Millisecond)
		defer clearTopMost(hwnd)

		ax, ay, e := refClientToActual(hwnd, cx, cy)
		if e != nil {
			err = e
			return
		}
		var pt point
		pt.X = int32(ax)
		pt.Y = int32(ay)
		r, _, e := procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&pt)))
		if r == 0 {
			err = fmt.Errorf("ClientToScreen: %v", e)
			return
		}
		out.ScreenX, out.ScreenY = int(pt.X), int(pt.Y)
		out.VirtX, out.VirtY, out.VirtW, out.VirtH = virtualDesktop()
		err = clickScreenAbsolute(&out)
	})
	return out, err
}

func systemMetric(idx int) int {
	v, _, _ := procGetSystemMetrics.Call(uintptr(idx))
	return int(int32(v))
}

func virtualDesktop() (vx, vy, vw, vh int) {
	vx = systemMetric(smXVirtualScreen)
	vy = systemMetric(smYVirtualScreen)
	vw = systemMetric(smCXVirtualScreen)
	vh = systemMetric(smCYVirtualScreen)
	if vw < 2 {
		vw = systemMetric(smCXScreen)
		vx = 0
	}
	if vh < 2 {
		vh = systemMetric(smCYScreen)
		vy = 0
	}
	return
}

// normalise 与 Python _Win32MouseMotion.normalise 一致：65535 覆盖桌面最后一像素。
func normalise(x, y, vx, vy, vw, vh int) (int32, int32) {
	spanX := vw - 1
	spanY := vh - 1
	if spanX < 1 {
		spanX = 1
	}
	if spanY < 1 {
		spanY = 1
	}
	nx := int(float64(x-vx)*65535.0/float64(spanX) + 0.5)
	ny := int(float64(y-vy)*65535.0/float64(spanY) + 0.5)
	if nx < 0 {
		nx = 0
	}
	if nx > 65535 {
		nx = 65535
	}
	if ny < 0 {
		ny = 0
	}
	if ny > 65535 {
		ny = 65535
	}
	return int32(nx), int32(ny)
}

// clickScreenAbsolute 对齐原版 _click_abs：
//  1. SendInput 绝对移动（Unity 6 Raw Input 需要真实 motion，不能只靠 SetCursorPos）
//  2. SetCursorPos 校正到精确像素
//  3. 按下/抬起不加 ABSOLUTE，点在当前光标处
func clickScreenAbsolute(out *ClickResult) error {
	x, y := out.ScreenX, out.ScreenY
	vx, vy, vw, vh := out.VirtX, out.VirtY, out.VirtW, out.VirtH
	absX, absY := normalise(x, y, vx, vy, vw, vh)
	moveFlags := uint32(mouseEventMove | mouseEventAbsolute | mouseEventVirtualDesk)

	if err := sendMouseAbs(absX, absY, moveFlags, 0); err != nil {
		return err
	}
	// 归一化有损；校正到精确像素。motion 事件已发出，Unity 已收到移动。
	procSetCursorPos.Call(uintptr(x), uintptr(y))
	time.Sleep(100 * time.Millisecond)

	var cur point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&cur)))
	out.CursorX, out.CursorY = int(cur.X), int(cur.Y)

	if err := sendMouseAbs(0, 0, mouseEventLeftDown, 0); err != nil {
		return err
	}
	time.Sleep(60 * time.Millisecond)
	return sendMouseAbs(0, 0, mouseEventLeftUp, 0)
}

func sendMouseAbs(dx, dy int32, flags uint32, data uint32) error {
	in := mouseInput{Type: inputMouse, dx: dx, dy: dy, mouseData: data, flags: flags}
	n, _, err := procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
	if n == 0 {
		return fmt.Errorf("SendInput: %v", err)
	}
	return nil
}

// WheelClient 在客户区某点滚轮。
func WheelClient(hwnd uintptr, cx, cy int, delta int) error {
	Focus(hwnd)
	var err error
	windpi.WithPhysical(func() {
		raiseTopMost(hwnd)
		time.Sleep(30 * time.Millisecond)
		defer clearTopMost(hwnd)

		var pt point
		ax, ay, ce := refClientToActual(hwnd, cx, cy)
		if ce != nil {
			err = ce
			return
		}
		pt.X = int32(ax)
		pt.Y = int32(ay)
		r, _, e := procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&pt)))
		if r == 0 {
			err = fmt.Errorf("ClientToScreen: %v", e)
			return
		}
		sx, sy := int(pt.X), int(pt.Y)
		vx, vy, vw, vh := virtualDesktop()
		absX, absY := normalise(sx, sy, vx, vy, vw, vh)
		_ = sendMouseAbs(absX, absY, mouseEventMove|mouseEventAbsolute|mouseEventVirtualDesk, 0)
		procSetCursorPos.Call(uintptr(sx), uintptr(sy))
		time.Sleep(20 * time.Millisecond)
		err = sendMouseAbs(0, 0, mouseEventWheel, uint32(int32(delta)))
	})
	return err
}
