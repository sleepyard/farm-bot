// Package hotkey 提供 Windows 全局热键（RegisterHotKey）。
package hotkey

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	vkF9        = 0x78
	modNoRepeat = 0x4000
	wmHotkey    = 0x0312
	wmQuit      = 0x0012
	hotkeyID    = 1
)

var (
	user32                 = windows.NewLazySystemDLL("user32.dll")
	procRegisterHotKey     = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey   = user32.NewProc("UnregisterHotKey")
	procGetMessageW        = user32.NewProc("GetMessageW")
	procTranslateMessage   = user32.NewProc("TranslateMessage")
	procDispatchMessageW   = user32.NewProc("DispatchMessageW")
	procPostThreadMessageW = user32.NewProc("PostThreadMessageW")
)

type msg struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

// WatchF9 在独立线程上注册全局 F9；按下时调用 onPress（勿阻塞过久）。
// ctx 取消时注销热键并退出。
func WatchF9(ctx context.Context, onPress func()) error {
	return Watch(ctx, vkF9, onPress)
}

// Watch 监听指定虚拟键码的全局热键。
func Watch(ctx context.Context, vk uint32, onPress func()) error {
	if onPress == nil {
		return fmt.Errorf("onPress 为空")
	}
	errCh := make(chan error, 1)
	var threadID uint32
	var ready sync.WaitGroup
	ready.Add(1)

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		threadID = windows.GetCurrentThreadId()
		r, _, e := procRegisterHotKey.Call(0, hotkeyID, modNoRepeat, uintptr(vk))
		if r == 0 {
			ready.Done()
			errCh <- fmt.Errorf("RegisterHotKey 失败: %w", e)
			return
		}
		ready.Done()
		errCh <- nil

		var m msg
		for {
			ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(ret) <= 0 {
				break
			}
			if m.Message == wmHotkey && m.WParam == hotkeyID {
				onPress()
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
		procUnregisterHotKey.Call(0, hotkeyID)
	}()

	ready.Wait()
	if err := <-errCh; err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		if threadID != 0 {
			procPostThreadMessageW.Call(uintptr(threadID), wmQuit, 0, 0)
		}
	}()
	return nil
}
