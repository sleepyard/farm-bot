package window

import "fmt"

// Rect 表示屏幕上的一个矩形区域，坐标单位为物理像素。
type Rect struct {
	X int // 左上角屏幕 X
	Y int // 左上角屏幕 Y
	W int // 宽度
	H int // 高度
}

// Size 返回 "宽x高" 形式的分辨率字符串，例如 "1280x720"。
func (r Rect) Size() string {
	return fmt.Sprintf("%dx%d", r.W, r.H)
}

// Position 返回 "(x, y)" 形式的坐标字符串。
func (r Rect) Position() string {
	return fmt.Sprintf("(%d, %d)", r.X, r.Y)
}

func (r Rect) String() string {
	return fmt.Sprintf("Rect{x=%d y=%d w=%d h=%d}", r.X, r.Y, r.W, r.H)
}
