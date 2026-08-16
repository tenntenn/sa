// Package paths resolves the directories sbnn keeps its state and its
// generated preview files in.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const appName = "sbnn"

// StateDir returns the directory holding the session state, creating it if
// needed. It follows the XDG base directory specification on Unix.
func StateDir() (string, error) {
	var base string
	switch {
	case os.Getenv("XDG_STATE_HOME") != "":
		base = os.Getenv("XDG_STATE_HOME")
	case runtime.GOOS == "windows" || runtime.GOOS == "darwin":
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		base = dir
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "state")
	}
	dir := filepath.Join(base, appName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create state dir: %w", err)
	}
	return dir, nil
}

// SessionFile returns the path of the session state file for a port. Servers
// on different ports keep independent sessions, like mo does.
func SessionFile(port int) (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fmt.Sprintf("session-%d.json", port)), nil
}

// HistoryFile returns the log of submitted reviews. It is one file for the
// whole machine on purpose: the point of keeping reviews is to read them
// together, long after the servers that recorded them are gone.
func HistoryFile() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "reviews.jsonl"), nil
}

// CacheDir returns the directory for files sbnn generates, such as the
// Markdown reconstructed from a diff and handed to mo.
func CacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, appName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}
	return dir, nil
}
