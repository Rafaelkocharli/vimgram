package ui

import (
	"os/exec"
	"runtime"
)

// openWithOS opens path with the system default application.
// The call is fire-and-forget; errors are silently ignored.
func openWithOS(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	_ = cmd.Start()
}
