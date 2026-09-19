// Package vision 提供 MTGA 客户区截图与模板匹配。
// 全代码库统一使用 1280×720 参考空间：模板图与 ROI/点击点均按此空间制作与使用；
// 截图在捕获时归一化到该空间（CaptureClient），点击在发送时换算回实际客户区（input 包）。
package vision

import (
	"image"
	"math"
)

// LiveWidth / LiveHeight 与 window.RequiredLive* 对齐。
const (
	LiveWidth  = 1280
	LiveHeight = 720
)

// ROI 是客户区局部矩形（相对客户区左上角，单位像素）。
type ROI struct {
	X, Y, W, H int
}

// Point 是客户区局部坐标。
type Point struct {
	X, Y int
}

// ClampROI 将 ROI 裁剪到图像边界内。
func ClampROI(r ROI, bounds image.Rectangle) ROI {
	x0 := max(r.X, bounds.Min.X)
	y0 := max(r.Y, bounds.Min.Y)
	x1 := min(r.X+r.W, bounds.Max.X)
	y1 := min(r.Y+r.H, bounds.Max.Y)
	if x1 <= x0 || y1 <= y0 {
		return ROI{}
	}
	return ROI{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// FullROI 返回整幅客户区。
func FullROI() ROI {
	return ROI{X: 0, Y: 0, W: LiveWidth, H: LiveHeight}
}

// FromBase1920 把 Python 1920×1080 客户区坐标换成 1280×720（×2/3，四舍五入）。
func FromBase1920(x, y int) Point {
	return Point{
		X: int(math.Round(float64(x) * float64(LiveWidth) / 1920)),
		Y: int(math.Round(float64(y) * float64(LiveHeight) / 1080)),
	}
}

// FromBase1920ROI 把 Python 1920×1080 矩形换成 1280×720。
func FromBase1920ROI(x, y, w, h int) ROI {
	p0 := FromBase1920(x, y)
	p1 := FromBase1920(x+w, y+h)
	return ROI{X: p0.X, Y: p0.Y, W: p1.X - p0.X, H: p1.Y - p0.Y}
}
