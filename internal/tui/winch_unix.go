//go:build !windows

package tui

import (
	"os"
	"os/signal"
	"syscall"
)

// notifyResize asks to be told when the terminal changes size. Unix says so
// with SIGWINCH, which is the only way to hear about it without polling.
func notifyResize(ch chan<- os.Signal) {
	signal.Notify(ch, syscall.SIGWINCH)
}

func stopResize(ch chan<- os.Signal) {
	signal.Stop(ch)
}
