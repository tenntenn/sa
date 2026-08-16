package cmd

import (
	"encoding/json"
	"io"

	"github.com/pkg/browser"
)

func jsonEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc
}

func browserOpen(url string) error {
	return browser.OpenURL(url)
}
