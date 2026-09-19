// Package poweroff 在 15 胜完成后安排 Windows 关机（带延迟，可取消）。
package poweroff

import (
	"runtime"
	"strconv"
	"strings"
)

const (
	DefaultDelaySec = 120
	MinDelaySec     = 30
)

var runFn = runCmd

// Supported 是否支持关机（仅 Windows）。
func Supported() bool {
	return runtime.GOOS == "windows"
}

// CancelHint 取消关机的命令提示。
func CancelHint() string {
	return "shutdown /a"
}

func clampDelay(delaySeconds int) int {
	if delaySeconds < MinDelaySec {
		return MinDelaySec
	}
	return delaySeconds
}

func scheduleArgs(delaySeconds int, comment string) []string {
	args := []string{"shutdown", "/s", "/t", strconv.Itoa(clampDelay(delaySeconds))}
	text := strings.TrimSpace(comment)
	if text == "" {
		return args
	}
	if len(text) > 500 {
		text = text[:500]
	}
	return append(args, "/c", text)
}

// Schedule 请求操作系统在 delaySeconds 后关机。延迟至少 MinDelaySec，以便还能取消。
func Schedule(delaySeconds int, comment string) bool {
	if !Supported() {
		return false
	}
	return runFn(scheduleArgs(delaySeconds, comment))
}

// Cancel 取消计划中的关机。
func Cancel() bool {
	if !Supported() {
		return false
	}
	return runFn([]string{"shutdown", "/a"})
}
