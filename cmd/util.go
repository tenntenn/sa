package cmd

import (
	"encoding/json"
	"io"
	"os"

	"github.com/pkg/browser"

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
