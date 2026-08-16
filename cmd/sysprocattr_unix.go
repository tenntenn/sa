//go:build !windows

package cmd

import (
	"os/exec"
	"syscall"
)

// setSysProcAttr detaches the background server from the terminal so that it
// survives the shell that started it.
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
