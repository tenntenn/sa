package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/tenntenn/sbnn/internal/model"
)

// hookTimeout bounds a hook, so that a command waiting for something never
// keeps a review round open.
const hookTimeout = 10 * time.Minute

// ReviewEvent is what a hook is told about a submitted review.
type ReviewEvent struct {
	Group      string           `json:"group"`
	URL        string           `json:"url"`
	ReviewedAt time.Time        `json:"reviewedAt"`
	Note       string           `json:"note,omitempty"`
	Comments   []*model.Comment `json:"comments"`
	Prompt     string           `json:"prompt"`
}

// runHooks reacts to a submitted review.
//
// This is the half of the loop that does not need anyone waiting: the person
// who sent the diff may be in a meeting, or long gone, and the review still
// gets picked up when they come back to it.
func (s *Server) runHooks(g *model.Group) {
	event := ReviewEvent{
		Group:      g.Name,
		URL:        GroupURL(s.BaseURL(), g.Name),
		ReviewedAt: g.ReviewedAt,
		Note:       g.ReviewNote,
		Comments:   openComments(g),
		Prompt:     Prompt(g, PromptOptions{}),
	}
	for _, h := range g.Hooks {
		go s.runHook(h, event)
	}
}

func openComments(g *model.Group) []*model.Comment {
	out := make([]*model.Comment, 0, len(g.Comments))
	for _, c := range g.Comments {
		if !c.Resolved {
			out = append(out, c)
		}
	}
	return out
}

func (s *Server) runHook(h *model.Hook, event ReviewEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), hookTimeout)
	defer cancel()

	if h.Command != "" {
		s.runHookCommand(ctx, h, event)
	}
	if h.URL != "" {
		s.postHook(ctx, h, event)
	}
}

// runHookCommand runs the command through the shell, with the review prompt
// on its stdin and the details in the environment.
func (s *Server) runHookCommand(ctx context.Context, h *model.Hook, event ReviewEvent) {
	shell, flag := "/bin/sh", "-c"
	if runtime.GOOS == "windows" {
		shell, flag = "cmd", "/c"
	}
	cmd := exec.CommandContext(ctx, shell, flag, h.Command)
	cmd.Stdin = bytes.NewReader([]byte(event.Prompt))
	cmd.Env = append(cmd.Environ(),
		"SBNN_GROUP="+event.Group,
		"SBNN_URL="+event.URL,
		"SBNN_SERVER="+s.BaseURL(),
		"SBNN_PORT="+strconv.Itoa(s.opts.Port),
		"SBNN_COMMENTS="+strconv.Itoa(len(event.Comments)),
		"SBNN_REVIEW_NOTE="+event.Note,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.Warn("review hook failed", "hook", h.ID, "command", h.Command, "error", err,
			"output", string(bytes.TrimSpace(out)))
		return
	}
	slog.Info("review hook ran", "hook", h.ID, "command", h.Command,
		"output", string(bytes.TrimSpace(out)))
}

func (s *Server) postHook(ctx context.Context, h *model.Hook, event ReviewEvent) {
	body, err := json.Marshal(event)
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.URL, bytes.NewReader(body))
	if err != nil {
		slog.Warn("review hook has an unusable url", "hook", h.ID, "url", h.URL, "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Warn("review hook could not be delivered", "hook", h.ID, "url", h.URL, "error", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		slog.Warn("review hook was refused", "hook", h.ID, "url", h.URL, "status", resp.Status)
		return
	}
	slog.Info("review hook delivered", "hook", h.ID, "url", h.URL, "status", resp.Status)
}
