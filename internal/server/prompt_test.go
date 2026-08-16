package server_test

import (
	"strings"
	"testing"
	"time"

	"github.com/tenntenn/sa/internal/model"
	"github.com/tenntenn/sa/internal/server"
)

// The verdict is the first thing an agent reads, because it decides what
// the comments mean: the same three remarks block a change or do not,
// depending on what the reviewer said about the change as a whole.
func TestPromptStatesTheVerdict(t *testing.T) {
	group := func(v model.Verdict) *model.Group {
		return &model.Group{
			Name:          "api",
			ReviewedAt:    time.Now(),
			ReviewVerdict: v,
			Comments: []*model.Comment{
				{Path: "main.go", Side: "new", StartLine: 4, EndLine: 4, Body: "a remark"},
			},
		}
	}
	cases := []struct {
		verdict model.Verdict
		want    []string
		absent  []string
	}{
		{model.VerdictApproved,
			[]string{"approved the change", "does not block", "came with the approval"},
			[]string{"to address", "Address every comment"}},
		{model.VerdictChangesRequested,
			[]string{"asked for changes", "should not go ahead", "to address"},
			[]string{"approved"}},
		{model.VerdictCommented,
			[]string{"without deciding either way", "to address"},
			[]string{"approved", "asked for changes"}},
	}
	for _, tc := range cases {
		t.Run(string(tc.verdict), func(t *testing.T) {
			got := server.Prompt(group(tc.verdict), server.PromptOptions{})
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("prompt does not say %q:\n%s", want, got)
				}
			}
			for _, absent := range tc.absent {
				if strings.Contains(got, absent) {
					t.Errorf("prompt should not say %q:\n%s", absent, got)
				}
			}
		})
	}
}

// A round nobody has submitted yet says nothing about a verdict: there is
// no reviewer to attribute one to.
func TestPromptSaysNothingBeforeSubmission(t *testing.T) {
	g := &model.Group{Name: "api", Comments: []*model.Comment{
		{Path: "main.go", Side: "new", StartLine: 4, EndLine: 4, Body: "a remark"},
	}}
	if got := server.Prompt(g, server.PromptOptions{}); strings.Contains(got, "The reviewer ") {
		t.Errorf("prompt claims a verdict before one was given:\n%s", got)
	}
}
