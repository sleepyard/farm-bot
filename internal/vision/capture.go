package vision

import (
	"fmt"
	"image"
	"math"
	"time"
	"unsafe"

	"github.com/flourbrain/mtga-farm-bot/internal/windpi"
	"golang.org/x/sys/windows"
)

var (
	modGdi32 = windows.NewLazySystemDLL("gdi32.dll")
	modUser  = windows.NewLazySystemDLL("user32.dll")

	procGetDC                  = modUser.NewProc("GetDC")
	procReleaseDC              = modUser.NewProc("ReleaseDC")
	procClientToScreen         = modUser.NewProc("ClientToScreen")
	procGetClientRect          = modUser.NewProc("GetClientRect")
	procPrintWindow            = modUser.NewProc("PrintWindow")
	procSetWindowPos           = modUser.NewProc("SetWindowPos")
	procCreateCompatibleDC     = modGdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = modGdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = modGdi32.NewProc("SelectObject")
	procBitBlt                 = modGdi32.NewProc("BitBlt")
	procDeleteDC               = modGdi32.NewProc("DeleteDC")
	procDeleteObject           = modGdi32.NewProc("DeleteObject")
	procGetDIBits              = modGdi32.NewProc("GetDIBits")
)

const (
	srcCopy             = 0x00CC0020
	biRGB               = 0
	dibRGBColors        = 0
	pwRenderFullContent = 0x00000002
	// SetWindowPos
	hwndTopMost   = ^uintptr(0) // (HWND)-1
	hwndNoTopMost = ^uintptr(1) // (HWND)-2
	swpNoMove     = 0x0002
	swpNoSize     = 0x0001
	swpNoActivate = 0x0010
	swpShowWindow = 0x0040
	minUsefulLuma = 8.0 // PrintWindow 黑屏时均值接近 0
)

type winRect struct {
	Left, Top, Right, Bottom int32
}

type point struct {
	X, Y int32
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [1]uint32
}

// CaptureClient 截取 hwnd 客户区。
//
// 策略（减轻被其它窗口遮挡）：
//  1. 优先 PrintWindow(PW_RENDERFULLCONTENT)：不依赖桌面合成，遮挡时仍可能出图
//     （部分 DirectX/Unity 会黑屏，需校验亮度）。
//  2. 失败则短暂 HWND_TOPMOST 后从桌面 DC BitBlt，再取消置顶。
//
// 全程在物理 DPI 上下文中执行，保证与鼠标坐标同一像素空间。
//
// 返回前统一归一化到 1280×720 参考空间：ROI、模板与点击坐标全部以此为
// 唯一坐标空间，实际客户区尺寸只在捕获（此处）与点击（input 包）两个边界换算。
func CaptureClient(hwnd uintptr) (*image.RGBA, error) {
	var img *image.RGBA
	var err error
	windpi.WithPhysical(func() {
		img, err = captureClientPhysical(hwnd)
	})
	if err != nil || img == nil {
		return img, err
	}
	if b := img.Bounds(); b.Dx() != LiveWidth || b.Dy() != LiveHeight {
		img = resizeRGBA(img, LiveWidth, LiveHeight)
	}
	return img, nil
}

func captureClientPhysical(hwnd uintptr) (*image.RGBA, error) {
	w, h, err := clientSize(hwnd)
	if err != nil {
		return nil, err
	}
	// 匹配坐标必须与屏幕点击同一像素来源：始终用桌面 BitBlt，不用 PrintWindow
	//（PrintWindow 对 Unity 可能内容/尺寸与真实客户区不一致，造成点击偏移）。
	return captureViaScreenTopMost(hwnd, w, h)
}

func clientSize(hwnd uintptr) (int, int, error) {
	var rc winRect
	r, _, err := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	if r == 0 {
		return 0, 0, fmt.Errorf("GetClientRect: %v", err)
	}
	w := int(rc.Right - rc.Left)
	h := int(rc.Bottom - rc.Top)
	if w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("客户区尺寸无效: %dx%d", w, h)
	}
	return w, h, nil
}

func captureViaPrintWindow(hwnd uintptr, w, h int) (*image.RGBA, bool) {
	hdcWin, _, _ := procGetDC.Call(hwnd)
	if hdcWin == 0 {
		return nil, false
	}
	defer procReleaseDC.Call(hwnd, hdcWin)

	hdcMem, hbmp, ok := allocBitmap(hdcWin, w, h)
	if !ok {
		return nil, false
	}
	defer freeBitmap(hdcMem, hbmp)

	old, _, _ := procSelectObject.Call(hdcMem, hbmp)
	defer procSelectObject.Call(hdcMem, old)

	r, _, _ := procPrintWindow.Call(hwnd, hdcMem, pwRenderFullContent)
	if r == 0 {
		return nil, false
	}
	img, err := bitsToRGBA(hdcMem, hbmp, w, h)
	if err != nil || MeanLuma(img) < minUsefulLuma {
		return nil, false
	}
	return img, true
}

func captureViaScreenTopMost(hwnd uintptr, w, h int) (*image.RGBA, error) {
	flags := uintptr(swpNoMove | swpNoSize | swpNoActivate | swpShowWindow)
	// 短暂置顶，避免桌面 BitBlt 拍到遮盖窗口。
	procSetWindowPos.Call(hwnd, hwndTopMost, 0, 0, 0, 0, flags)
	time.Sleep(30 * time.Millisecond)
	defer procSetWindowPos.Call(hwnd, hwndNoTopMost, 0, 0, 0, 0, flags)

	var origin point
	r, _, err := procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&origin)))
	if r == 0 {
		return nil, fmt.Errorf("ClientToScreen: %v", err)
	}

	hdcScreen, _, _ := procGetDC.Call(0)
	if hdcScreen == 0 {
		return nil, fmt.Errorf("GetDC(desktop) 失败")
	}
	defer procReleaseDC.Call(0, hdcScreen)

	hdcMem, hbmp, ok := allocBitmap(hdcScreen, w, h)
	if !ok {
		return nil, fmt.Errorf("创建位图失败")
	}
	defer freeBitmap(hdcMem, hbmp)

	old, _, _ := procSelectObject.Call(hdcMem, hbmp)
	defer procSelectObject.Call(hdcMem, old)

	okBlt, _, err := procBitBlt.Call(
		hdcMem, 0, 0, uintptr(w), uintptr(h),
		hdcScreen, uintptr(origin.X), uintptr(origin.Y),
		srcCopy,
	)
	if okBlt == 0 {
		return nil, fmt.Errorf("BitBlt 屏幕截图失败: %v", err)
	}
	return bitsToRGBA(hdcMem, hbmp, w, h)
}

func allocBitmap(hdcRef uintptr, w, h int) (hdcMem, hbmp uintptr, ok bool) {
	hdcMem, _, _ = procCreateCompatibleDC.Call(hdcRef)
	if hdcMem == 0 {
		return 0, 0, false
	}
	hbmp, _, _ = procCreateCompatibleBitmap.Call(hdcRef, uintptr(w), uintptr(h))
	if hbmp == 0 {
		procDeleteDC.Call(hdcMem)
		return 0, 0, false
	}
	return hdcMem, hbmp, true
}

func freeBitmap(hdcMem, hbmp uintptr) {
	if hbmp != 0 {
		procDeleteObject.Call(hbmp)
	}
	if hdcMem != 0 {
		procDeleteDC.Call(hdcMem)
	}
}

func bitsToRGBA(hdcMem, hbmp uintptr, w, h int) (*image.RGBA, error) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	bi := bitmapInfo{}
	bi.Header.Size = uint32(unsafe.Sizeof(bi.Header))
	bi.Header.Width = int32(w)
	bi.Header.Height = -int32(h)
	bi.Header.Planes = 1
	bi.Header.BitCount = 32
	bi.Header.Compression = biRGB

	ret, _, err := procGetDIBits.Call(
		hdcMem, hbmp, 0, uintptr(h),
		uintptr(unsafe.Pointer(&img.Pix[0])),
		uintptr(unsafe.Pointer(&bi)),
		dibRGBColors,
	)
	if ret == 0 {
		return nil, fmt.Errorf("GetDIBits: %v", err)
	}
	for i := 0; i+3 < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+2] = img.Pix[i+2], img.Pix[i]
	}
	return img, nil
}

// MeanLuma 返回灰度均值，用于判断截图是否接近全黑。
func MeanLuma(img *image.RGBA) float64 {
	if img == nil || len(img.Pix) == 0 {
		return 0
	}
	var sum float64
	n := 0
	for i := 0; i+2 < len(img.Pix); i += 4 {
		r, g, b := img.Pix[i], img.Pix[i+1], img.Pix[i+2]
		sum += float64(int(r)*299+int(g)*587+int(b)*114) / 1000
		n++
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// resizeRGBA 双线性插值缩放到 w×h。
// 不用最近邻：文字按钮缩到约 0.8 倍时锯齿会拉低 NCC 匹配分数。
func resizeRGBA(src *image.RGBA, w, h int) *image.RGBA {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	if sw < 1 || sh < 1 || w < 1 || h < 1 {
		return dst
	}
	scaleX := float64(sw) / float64(w)
	scaleY := float64(sh) / float64(h)
	for y := 0; y < h; y++ {
		// 目标像素中心映射回源坐标
		fy := (float64(y)+0.5)*scaleY - 0.5
		y0 := int(math.Floor(fy))
		fy -= float64(y0)
		if y0 < 0 {
			y0, fy = 0, 0
		}
		y1 := y0 + 1
		if y1 >= sh {
			y1 = sh - 1
		}
		row0 := src.PixOffset(sb.Min.X, sb.Min.Y+y0)
		row1 := src.PixOffset(sb.Min.X, sb.Min.Y+y1)
		for x := 0; x < w; x++ {
			fx := (float64(x)+0.5)*scaleX - 0.5
			x0 := int(math.Floor(fx))
			fx -= float64(x0)
			if x0 < 0 {
				x0, fx = 0, 0
			}
			x1 := x0 + 1
			if x1 >= sw {
				x1 = sw - 1
			}
			w00 := (1 - fx) * (1 - fy)
			w10 := fx * (1 - fy)
			w01 := (1 - fx) * fy
			w11 := fx * fy
			i00, i10 := row0+x0*4, row0+x1*4
			i01, i11 := row1+x0*4, row1+x1*4
			di := y*dst.Stride + x*4
			for c := 0; c < 4; c++ {
				v := float64(src.Pix[i00+c])*w00 + float64(src.Pix[i10+c])*w10 +
					float64(src.Pix[i01+c])*w01 + float64(src.Pix[i11+c])*w11
				dst.Pix[di+c] = uint8(v + 0.5)
			}
		}
	}
	return dst
}
