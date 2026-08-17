//go:build windows

package tui

import "os"

// Windows has no SIGWINCH: a console reports a resize as an input record on
// the console handle, not as a signal. sbnn tui has no way to open a console
// there anyway - OpenTTY fails first - so nothing is subscribed to, and the
// frame keeps the size it started with.
func notifyResize(chan<- os.Signal) {}

func stopResize(chan<- os.Signal) {}
