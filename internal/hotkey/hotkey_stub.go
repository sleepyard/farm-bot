//go:build !windows

package hotkey

import (
	"context"
	"fmt"
)

// WatchF9 非 Windows 平台不支持全局热键。
func WatchF9(ctx context.Context, onPress func()) error {
	return fmt.Errorf("全局热键仅支持 Windows")
}
