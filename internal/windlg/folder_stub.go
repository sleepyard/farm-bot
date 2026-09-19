//go:build !windows

package windlg

import "fmt"

func PickFolder(title string) (string, error) {
	_ = title
	return "", fmt.Errorf("当前系统不支持文件夹选择框")
}
