package input

import (
	"fmt"
	"time"
	"unsafe"
)

const (
	inputKeyboard  = 1
	keyeventfKeyup = 0x0002
	vkEscape       = 0x1B
)

// kbInput 对齐 64 位 Windows INPUT + KEYBDINPUT（与 mouseInput 同宽 40 字节）。
type kbInput struct {
	Type      uint32
	_pad      uint32
	vk        uint16
	scan      uint16
	flags     uint32
	time      uint32
	extraInfo uintptr
	_unionPad [8]byte
}

// TapEscape 按下并抬起 Esc（需窗口已在前台）。
func TapEscape() error {
	if err := sendKey(vkEscape, 0); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	return sendKey(vkEscape, keyeventfKeyup)
}

func sendKey(vk uint16, flags uint32) error {
	in := kbInput{Type: inputKeyboard, vk: vk, flags: flags}
	n, _, err := procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
	if n == 0 {
		return fmt.Errorf("SendInput key: %v", err)
	}
	return nil
}
