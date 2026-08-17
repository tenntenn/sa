package tui

import (
	"fmt"
	"os"
)

// OpenTTY opens the controlling terminal for reading and writing.
//
// The one file is used for both: keys cannot come from stdin, which carries
// the diff, and frames cannot go to stdout, which may be a pipe. Windows has
// no /dev/tty, so this fails there at run time - the package still builds for
// it, because a build tag would only hide the same outcome behind a compile.
func OpenTTY() (*os.File, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("cannot open /dev/tty: %w", err)
	}
	return tty, nil
}
