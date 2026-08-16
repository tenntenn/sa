package cmd

import (
	"encoding/json"
	"io"
	"os"

	"github.com/pkg/browser"

	"github.com/tenntenn/sa/internal/model"
	"github.com/tenntenn/sa/internal/server"
)

// TargetEnv names the group to work on when --target is not given. It is
// there for whoever is driving sa - a shell, a script, an agent working in
// two checkouts at once - to say which review is theirs, once, instead of
// repeating the flag. sa itself has no idea what the name stands for.
const TargetEnv = "SA_TARGET"

// groupName resolves the group a command works on: --target, then
// $SA_TARGET, then the default group.
func groupName(flag string) (string, error) {
	if flag == "" {
		flag = os.Getenv(TargetEnv)
	}
	return server.ValidateGroupName(flag)
}

func jsonEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc
}

func browserOpen(url string) error {
	return browser.OpenURL(url)
}

// isTerminal reports whether a stream is attached to a terminal. sa is meant
// to sit in a pipeline as much as in a shell, and a pipeline has nobody to
// open a browser for.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// ExitOpenComments is the status of a command that found comments to
// address, in the tradition of grep and diff: nothing to say is 0, something
// to say is 1, and a real failure is above that.
const ExitOpenComments = 1

// exitWithComments ends a --exit-code command.
func exitWithComments(comments []*model.Comment) error {
	for _, c := range comments {
		if !c.Resolved {
			os.Exit(ExitOpenComments)
		}
	}
	return nil
}
