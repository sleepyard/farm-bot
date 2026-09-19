package input

import "time"

const (
	parkX = 1279
	parkY = 719
)

// ParkCursor 把光标停到 1280×720 客户区右下角 @(1279,719)。
// BitBlt 桌面截图会带上指针；点完按钮后光标还停在图标上，悬停高亮也会让模板对不上。
func ParkCursor(hwnd uintptr) error {
	if hwnd == 0 {
		return nil
	}
	if err := MoveClient(hwnd, parkX, parkY); err != nil {
		return err
	}
	time.Sleep(80 * time.Millisecond)
	return nil
}
