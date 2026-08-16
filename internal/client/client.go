// Package client talks to a running sa server.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tenntenn/sa/internal/model"
	"github.com/tenntenn/sa/internal/server"
)

// Client is an HTTP client for the sa API.
type Client struct {
	Addr string
	HTTP *http.Client
}

// New returns a client for the server at addr (host:port).
func New(addr string, timeout time.Duration) *Client {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &Client{Addr: addr, HTTP: &http.Client{Timeout: timeout}}
}

// BaseURL returns the URL of the server.
func (c *Client) BaseURL() string { return "http://" + c.Addr }

func (c *Client) url(format string, args ...any) string {
	return c.BaseURL() + fmt.Sprintf(format, args...)
}

// Status reports the state of the server. It doubles as the probe telling
// whether a sa server owns the port.
func (c *Client) Status(ctx context.Context) (*server.Status, error) {
	var st server.Status
	if err := c.do(ctx, http.MethodGet, c.url("/_/api/status"), nil, &st); err != nil {
		return nil, err
	}
	if st.App != "sa" {
		return nil, fmt.Errorf("the server on %s is not sa", c.Addr)
	}
	return &st, nil
}

// AddDiff sends a diff to a group.
func (c *Client) AddDiff(ctx context.Context, group string, req server.AddDiffRequest) (*server.AddDiffResponse, error) {
	var res server.AddDiffResponse
	if err := c.do(ctx, http.MethodPost, c.url("/_/api/groups/%s/diffs", url.PathEscape(group)), req, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// Group returns a whole group: its diffs and its comments.
func (c *Client) Group(ctx context.Context, group string) (*model.Group, error) {
	var g model.Group
	if err := c.do(ctx, http.MethodGet, c.url("/_/api/groups/%s", url.PathEscape(group)), nil, &g); err != nil {
		return nil, err
	}
	return &g, nil
}

// Comments returns the review comments of a group.
func (c *Client) Comments(ctx context.Context, group string) ([]*model.Comment, error) {
	var comments []*model.Comment
	if err := c.do(ctx, http.MethodGet, c.url("/_/api/groups/%s/comments", url.PathEscape(group)), nil, &comments); err != nil {
		return nil, err
	}
	return comments, nil
}

// Prompt returns the review comments of a group rendered for an agent. When
// instruction is false the closing "address every comment" line is left out.
func (c *Client) Prompt(ctx context.Context, group string, includeResolved, instruction bool) (string, error) {
	q := url.Values{}
	if includeResolved {
		q.Set("resolved", "true")
	}
	if !instruction {
		q.Set("instruction", "false")
	}
	u := c.url("/_/api/groups/%s/prompt", url.PathEscape(group))
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return string(b), nil
}

// ClearComments removes the comments of a group and returns how many were
// removed.
func (c *Client) ClearComments(ctx context.Context, group string, resolvedOnly bool) (int, error) {
	u := c.url("/_/api/groups/%s/comments", url.PathEscape(group))
	if resolvedOnly {
		u += "?resolved=true"
	}
	var res struct {
		Removed int `json:"removed"`
	}
	if err := c.do(ctx, http.MethodDelete, u, nil, &res); err != nil {
		return 0, err
	}
	return res.Removed, nil
}

// DeleteGroup drops a whole group, diffs and comments alike.
func (c *Client) DeleteGroup(ctx context.Context, group string) error {
	return c.do(ctx, http.MethodDelete, c.url("/_/api/groups/%s", url.PathEscape(group)), nil, nil)
}

// Shutdown asks the server to stop.
func (c *Client) Shutdown(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, c.url("/_/api/shutdown"), nil, nil)
}

func (c *Client) do(ctx context.Context, method, u string, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		text := strings.TrimSpace(string(msg))
		if text == "" {
			text = resp.Status
		}
		return fmt.Errorf("%s %s: %s", method, u, text)
	}
	if out == nil {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return nil
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(out)
}
