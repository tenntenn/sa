package cmd

import (
	"context"
	"encoding/json"
	"io"
	"os"

	"github.com/pkg/browser"

	"github.com/tenntenn/sa/internal/client"
	"github.com/tenntenn/sa/internal/server"
)

func jsonEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc
}

func browserOpen(url string) error {
	return browser.OpenURL(url)
}

// TargetEnv names the group to work with when no --target is given. It is
// there for scripts and agents that want a fixed group without repeating the
// flag.
const TargetEnv = "SA_TARGET"

// explicitTarget is the group the user named, by flag or by environment.
func explicitTarget(explicit string) string {
	if explicit != "" {
		return explicit
	}
	return os.Getenv(TargetEnv)
}

// resolveGroup decides which group a command works on.
//
// An explicit --target always wins, then SA_TARGET, and otherwise the group
// of the directory the command runs in: sa keeps one review per checkout, so
// two worktrees of the same project do not review each other's diffs.
func resolveGroup(ctx context.Context, c *client.Client, explicit string) (string, error) {
	explicit = explicitTarget(explicit)
	if explicit != "" {
		return server.ValidateGroupName(explicit)
	}
	res, err := c.Resolve(ctx, workingDir())
	if err != nil {
		return "", err
	}
	return res.Group, nil
}
