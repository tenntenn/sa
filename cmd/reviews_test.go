package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tenntenn/sbnn/internal/history"
)

// stream is two comments with everything the text columns draw from.
func stream() []history.CommentRecord {
	return []history.CommentRecord{
		{
			Group:      "api",
			ReviewedAt: time.Date(2026, 8, 15, 10, 0, 0, 0, time.Local),
			Labels:     map[string]string{"branch": "main"},
			Comment: history.Comment{
				Path: "internal/server/server.go", Side: "new",
				StartLine: 12, EndLine: 14,
				Body: "rename this\nand a second line that must not leak",
			},
		},
		{
			Group:      "web",
			ReviewedAt: time.Date(2026, 8, 16, 10, 0, 0, 0, time.Local),
			Comment: history.Comment{
				Path: "README.md", Author: "claude", Side: "new",
				StartLine: 3, EndLine: 3,
				Body:        "a\ttab becomes a space",
				Suggestions: []string{"fixed"},
			},
		},
	}
}

// The tab-separated text form is piped into cut and awk, which makes its
// columns a contract: date, group, path:lines, author, first line of the
// body. Changing any of them breaks pipelines that cannot be seen from
// here, so this test fails on any change, deliberate or not.
func TestCommentStreamTextColumnsAreAContract(t *testing.T) {
	var buf bytes.Buffer
	if err := printCommentStream(&buf, stream(), "text"); err != nil {
		t.Fatal(err)
	}
	want := "" +
		"2026-08-15\tapi\tinternal/server/server.go:12-14\treviewer\trename this\n" +
		"2026-08-16\tweb\tREADME.md:3\tclaude\ta tab becomes a space\n"
	if got := buf.String(); got != want {
		t.Errorf("the text columns changed:\ngot  %q\nwant %q", got, want)
	}
}

// jsonl means one JSON object per line; an indented object is not jsonl.
func TestCommentStreamJSONLIsOneObjectPerLine(t *testing.T) {
	var buf bytes.Buffer
	if err := printCommentStream(&buf, stream(), "jsonl"); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d line(s) for 2 comments:\n%s", len(lines), buf.String())
	}
	for _, line := range lines {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Errorf("line is not a JSON object: %q: %v", line, err)
		}
		// The record has to stand on its own: the fields jq joins on
		// must be on every line, flat, not nested under the comment.
		for _, key := range []string{"group", "reviewedAt", "path", "startLine", "body"} {
			if _, ok := rec[key]; !ok {
				t.Errorf("line lacks %q: %s", key, line)
			}
		}
	}
}
