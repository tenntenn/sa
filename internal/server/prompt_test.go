package server_test

import (
	"strings"
	"testing"
	"time"

	"github.com/tenntenn/sbnn/internal/model"
	"github.com/tenntenn/sbnn/internal/server"
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

// A question and a change request are different asks, and the prose does
// not always separate them. The prompt has to, or the agent rewrites code
// that only needed an explanation.
func TestPromptSeparatesQuestionsFromChanges(t *testing.T) {
	g := &model.Group{
		Name:       "api",
		ReviewedAt: time.Now(),
		Comments: []*model.Comment{
			{Path: "main.go", Side: "new", StartLine: 4, EndLine: 4,
				Body: "Should this be a 404?", Question: true},
			{Path: "main.go", Side: "new", StartLine: 9, EndLine: 9,
				Body: "rename this"},
		},
	}
	got := server.Prompt(g, server.PromptOptions{})
	for _, want := range []string{
		"2 comment(s) to address, 1 of them a question",
		"This one is a question: answer it.",
		"asking for an answer, not for a change",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt does not say %q:\n%s", want, got)
		}
	}
}

// Nothing about questions appears when none was asked.
func TestPromptSaysNothingAboutQuestionsWhenThereAreNone(t *testing.T) {
	g := &model.Group{Name: "api", ReviewedAt: time.Now(), Comments: []*model.Comment{
		{Path: "main.go", Side: "new", StartLine: 4, EndLine: 4, Body: "rename this"},
	}}
	if got := server.Prompt(g, server.PromptOptions{}); strings.Contains(got, "question") {
		t.Errorf("prompt talks about questions when none was asked:\n%s", got)
	}
}
