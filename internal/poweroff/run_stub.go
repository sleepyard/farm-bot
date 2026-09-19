//go:build !windows

package poweroff

func runCmd(_ []string) bool {
	return false
}
