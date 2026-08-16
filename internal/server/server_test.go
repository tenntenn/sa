package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tenntenn/sa/internal/mo"
	"github.com/tenntenn/sa/internal/model"
)

const sampleDiff = `diff --git a/README.md b/README.md
index 1111111..2222222 100644
--- a/README.md
+++ b/README.md
@@ -1,2 +1,3 @@
 # sa
-old line
+new line
+another line
diff --git a/docs/new.md b/docs/new.md
new file mode 100644
--- /dev/null
+++ b/docs/new.md
@@ -0,0 +1,2 @@
+# New
+body
`

func newTestServer(t *testing.T, opts ...func(*Options)) (*httptest.Server, *Server) {
	t.Helper()
	o := Options{
		SessionFile: filepath.Join(t.TempDir(), "session.json"),
		CacheDir:    t.TempDir(),
		Version:     "test",
		Mo:          mo.New("mo-not-installed-for-tests", 0, ""),
	}
	for _, f := range opts {
		f(&o)
	}
	srv, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.handler())
	t.Cleanup(ts.Close)
	return ts, srv
}

func postJSON(t *testing.T, url string, body any, out any) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(url, "application/json", strings.NewReader(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatal(err)
		}
	}
	return resp
}

func getJSON(t *testing.T, url string, out any) *http.Response {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatal(err)
		}
	}
	return resp
}

func TestAddDiffAndReadGroup(t *testing.T) {
	ts, _ := newTestServer(t)

	var added AddDiffResponse
	resp := postJSON(t, ts.URL+"/_/api/groups/default/diffs",
		AddDiffRequest{Title: "first", BaseDir: "/tmp", Content: sampleDiff}, &added)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %s", resp.Status)
	}
	if added.Diff == nil || len(added.Diff.Files) != 2 {
		t.Fatalf("stored diff = %+v", added.Diff)
	}

	var g model.Group
	getJSON(t, ts.URL+"/_/api/groups/default", &g)
	if len(g.Diffs) != 1 {
		t.Fatalf("got %d diffs, want 1", len(g.Diffs))
	}
	files := g.Diffs[0].Files
	if files[0].Status != model.StatusModified || files[0].ViewMode != model.ViewSplit {
		t.Errorf("README.md = %s/%s", files[0].Status, files[0].ViewMode)
	}
	if files[1].Status != model.StatusAdded || files[1].ViewMode != model.ViewUnified {
		t.Errorf("docs/new.md = %s/%s, want added/unified", files[1].Status, files[1].ViewMode)
	}
	if !files[1].IsMarkdown {
		t.Error("docs/new.md should be markdown")
	}
}

func TestAddDiffRejectsGarbage(t *testing.T) {
	ts, _ := newTestServer(t)
	resp := postJSON(t, ts.URL+"/_/api/groups/default/diffs",
		AddDiffRequest{Content: "this is not a diff\n"}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %s, want 400", resp.Status)
	}
}

func TestGroupsAreSeparate(t *testing.T) {
	ts, _ := newTestServer(t)
	postJSON(t, ts.URL+"/_/api/groups/default/diffs", AddDiffRequest{Content: sampleDiff}, nil)
	postJSON(t, ts.URL+"/_/api/groups/api/diffs", AddDiffRequest{Content: sampleDiff}, nil)

	var st Status
	getJSON(t, ts.URL+"/_/api/status", &st)
	if len(st.Groups) != 2 {
		t.Fatalf("got %d groups, want 2", len(st.Groups))
	}
	if st.Groups[0].Name != DefaultGroup {
		t.Errorf("first group = %q, want the default group first", st.Groups[0].Name)
	}
	if st.MoAvailable {
		t.Error("MoAvailable = true, want false when the mo binary is missing")
	}
}

func TestCommentLifecycle(t *testing.T) {
	ts, _ := newTestServer(t)
	var added AddDiffResponse
	postJSON(t, ts.URL+"/_/api/groups/default/diffs", AddDiffRequest{Content: sampleDiff}, &added)
	file := added.Diff.Files[0]

	var comment model.Comment
	resp := postJSON(t, ts.URL+"/_/api/groups/default/comments", AddCommentRequest{
		DiffID:    added.Diff.ID,
		FileID:    file.ID,
		Path:      file.Path(),
		Side:      "new",
		StartLine: 2,
		EndLine:   3,
		Body:      "please rephrase",
		Snippet:   "+new line\n+another line",
	}, &comment)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %s", resp.Status)
	}
	if comment.ID == "" || comment.Resolved {
		t.Fatalf("comment = %+v", comment)
	}

	prompt := getText(t, ts.URL+"/_/api/groups/default/prompt")
	if !strings.Contains(prompt, "please rephrase") || !strings.Contains(prompt, "README.md:2-3") {
		t.Errorf("prompt = %q", prompt)
	}

	// Resolving hides the comment from the prompt unless asked for.
	patch, err := http.NewRequest(http.MethodPatch,
		ts.URL+"/_/api/groups/default/comments/"+comment.ID,
		strings.NewReader(`{"resolved":true}`))
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(patch)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	if got := getText(t, ts.URL+"/_/api/groups/default/prompt"); strings.Contains(got, "please rephrase") {
		t.Errorf("resolved comment still in the prompt: %q", got)
	}
	if got := getText(t, ts.URL+"/_/api/groups/default/prompt?resolved=true"); !strings.Contains(got, "please rephrase") {
		t.Errorf("resolved comment missing with ?resolved=true: %q", got)
	}

	del, err := http.NewRequest(http.MethodDelete, ts.URL+"/_/api/groups/default/comments/"+comment.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err = http.DefaultClient.Do(del)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %s", res.Status)
	}

	var comments []*model.Comment
	getJSON(t, ts.URL+"/_/api/groups/default/comments", &comments)
	if len(comments) != 0 {
		t.Errorf("got %d comments after delete, want 0", len(comments))
	}
}

func TestCommentNeedsKnownDiff(t *testing.T) {
	ts, _ := newTestServer(t)
	postJSON(t, ts.URL+"/_/api/groups/default/diffs", AddDiffRequest{Content: sampleDiff}, nil)
	resp := postJSON(t, ts.URL+"/_/api/groups/default/comments", AddCommentRequest{
		DiffID: "nope", FileID: "nope", Path: "x", StartLine: 1, Body: "hi",
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %s, want 400", resp.Status)
	}
}

func TestSessionSurvivesRestart(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	ts, _ := newTestServer(t, func(o *Options) { o.SessionFile = sessionFile })
	postJSON(t, ts.URL+"/_/api/groups/default/diffs", AddDiffRequest{Title: "kept", Content: sampleDiff}, nil)

	// A second server reading the same file is what --restart does.
	restarted, err := New(Options{SessionFile: sessionFile, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	g, ok := restarted.Store().Group(DefaultGroup)
	if !ok || len(g.Diffs) != 1 || g.Diffs[0].Title != "kept" {
		t.Fatalf("restored group = %+v (ok=%v)", g, ok)
	}
}

func TestPreviewNeedsMarkdown(t *testing.T) {
	ts, _ := newTestServer(t)
	var added AddDiffResponse
	postJSON(t, ts.URL+"/_/api/groups/default/diffs", AddDiffRequest{Content: goDiff}, &added)
	file := added.Diff.Files[0]
	resp := getJSON(t, ts.URL+"/_/api/groups/default/diffs/"+added.Diff.ID+"/files/"+file.ID+"/preview", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %s, want 400 for a non Markdown file", resp.Status)
	}
}

func TestPreviewReportsMissingMo(t *testing.T) {
	ts, _ := newTestServer(t)
	var added AddDiffResponse
	postJSON(t, ts.URL+"/_/api/groups/default/diffs", AddDiffRequest{Content: sampleDiff}, &added)
	file := added.Diff.Files[1] // docs/new.md
	var body map[string]string
	resp := getJSON(t, ts.URL+"/_/api/groups/default/diffs/"+added.Diff.ID+"/files/"+file.ID+"/preview", &body)
	if resp.StatusCode != http.StatusFailedDependency {
		t.Fatalf("status = %s, want 424 when mo is missing", resp.Status)
	}
	if !strings.Contains(body["error"], "mo is not installed") {
		t.Errorf("error = %q", body["error"])
	}
}

func TestPreviewUsesMo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stub mo is a shell script")
	}
	dir := t.TempDir()
	stub := filepath.Join(dir, "mo")
	// The stub records its arguments and answers like `mo --json`.
	script := `#!/bin/sh
echo "$@" > ` + filepath.Join(dir, "args") + `
path=$(eval echo \${$#})
printf '{"url":"http://localhost:6275","files":[{"url":"http://localhost:6275/sa-default?file=abc","name":"new.md","path":"%s"}]}\n' "$path"
`
	if err := os.WriteFile(stub, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	ts, _ := newTestServer(t, func(o *Options) {
		o.Mo = mo.New(stub, 6275, "localhost")
		o.CacheDir = cacheDir
	})

	var added AddDiffResponse
	postJSON(t, ts.URL+"/_/api/groups/default/diffs", AddDiffRequest{Content: sampleDiff}, &added)
	file := added.Diff.Files[1] // docs/new.md, a new file not on disk

	var preview PreviewResponse
	resp := getJSON(t, ts.URL+"/_/api/groups/default/diffs/"+added.Diff.ID+"/files/"+file.ID+"/preview", &preview)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %s", resp.Status)
	}
	if preview.Source != SourceReconstructed || !preview.Complete {
		t.Errorf("preview = %+v, want a complete reconstruction of a new file", preview)
	}
	if preview.MoURL != "http://localhost:6275/sa-default?file=abc" {
		t.Errorf("moUrl = %q", preview.MoURL)
	}
	content, err := os.ReadFile(preview.Path)
	if err != nil {
		t.Fatalf("reconstructed file: %v", err)
	}
	if string(content) != "# New\nbody\n" {
		t.Errorf("reconstructed content = %q", content)
	}
	args, err := os.ReadFile(filepath.Join(dir, "args"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), "--target sa-default") {
		t.Errorf("mo args = %q, want sa's own mo group", args)
	}
}

func TestPreviewPrefersWorktreeFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stub mo is a shell script")
	}
	dir := t.TempDir()
	stub := filepath.Join(dir, "mo")
	script := "#!/bin/sh\n" +
		`printf '{"url":"http://localhost:6275","files":[]}\n'` + "\n"
	if err := os.WriteFile(stub, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	if err := os.MkdirAll(filepath.Join(work, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(work, "docs", "new.md")
	if err := os.WriteFile(real, []byte("# From the working tree\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ts, _ := newTestServer(t, func(o *Options) { o.Mo = mo.New(stub, 6275, "localhost") })
	var added AddDiffResponse
	postJSON(t, ts.URL+"/_/api/groups/default/diffs",
		AddDiffRequest{Content: sampleDiff, BaseDir: work}, &added)
	file := added.Diff.Files[1]

	var preview PreviewResponse
	getJSON(t, ts.URL+"/_/api/groups/default/diffs/"+added.Diff.ID+"/files/"+file.ID+"/preview", &preview)
	if preview.Source != SourceWorktree || preview.Path != real {
		t.Errorf("preview = %+v, want the working tree file %s", preview, real)
	}
}

func TestValidateGroupName(t *testing.T) {
	for _, tt := range []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", DefaultGroup, false},
		{"api", "api", false},
		{"feature-1.2_x", "feature-1.2_x", false},
		{"_internal", "", true},
		{"../etc", "", true},
		{"a/b", "", true},
		{"with space", "", true},
	} {
		got, err := ValidateGroupName(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ValidateGroupName(%q) = %q, want an error", tt.in, got)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("ValidateGroupName(%q) = %q, %v", tt.in, got, err)
		}
	}
}

func getText(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const goDiff = `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,2 +1,2 @@
 package main
-var x = 1
+var x = 2
`

func TestFileContentServesMarkdown(t *testing.T) {
	ts, _ := newTestServer(t)
	var added AddDiffResponse
	postJSON(t, ts.URL+"/_/api/groups/default/diffs", AddDiffRequest{Content: sampleDiff}, &added)
	file := added.Diff.Files[1] // docs/new.md, a new file not on disk

	var got FileContentResponse
	resp := getJSON(t, ts.URL+"/_/api/groups/default/diffs/"+added.Diff.ID+"/files/"+file.ID+"/content", &got)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %s", resp.Status)
	}
	if got.Content != "# New\nbody\n" || got.Source != SourceReconstructed || !got.Complete {
		t.Errorf("content = %+v", got)
	}
	// The Markdown is served without mo being involved at all.
	if !strings.Contains(got.Content, "# New") {
		t.Errorf("content = %q", got.Content)
	}
}

func TestFileContentNeedsMarkdown(t *testing.T) {
	ts, _ := newTestServer(t)
	var added AddDiffResponse
	postJSON(t, ts.URL+"/_/api/groups/default/diffs", AddDiffRequest{Content: goDiff}, &added)
	file := added.Diff.Files[0]
	resp := getJSON(t, ts.URL+"/_/api/groups/default/diffs/"+added.Diff.ID+"/files/"+file.ID+"/content", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %s, want 400 for a non Markdown file", resp.Status)
	}
}

func TestSuggestionInPrompt(t *testing.T) {
	ts, _ := newTestServer(t)
	var added AddDiffResponse
	postJSON(t, ts.URL+"/_/api/groups/default/diffs", AddDiffRequest{Content: sampleDiff}, &added)
	file := added.Diff.Files[0]

	var comment model.Comment
	resp := postJSON(t, ts.URL+"/_/api/groups/default/comments", AddCommentRequest{
		DiffID:     added.Diff.ID,
		FileID:     file.ID,
		Path:       file.Path(),
		Side:       "new",
		StartLine:  2,
		EndLine:    3,
		Snippet:    "+new line\n+another line",
		Suggestion: "a better line\nand another",
	}, &comment)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %s", resp.Status)
	}
	// A suggestion is a complete comment on its own: no body needed.
	if comment.Suggestion != "a better line\nand another" {
		t.Fatalf("comment = %+v", comment)
	}

	prompt := getText(t, ts.URL+"/_/api/groups/default/prompt")
	if !strings.Contains(prompt, "```suggestion\na better line\nand another\n```") {
		t.Errorf("prompt lacks an applicable suggestion block: %q", prompt)
	}
	if !strings.Contains(prompt, "Suggested replacement for README.md:2-3") {
		t.Errorf("prompt does not name the replaced lines: %q", prompt)
	}
}

func TestSuggestionOnlyOnTheNewSide(t *testing.T) {
	ts, _ := newTestServer(t)
	var added AddDiffResponse
	postJSON(t, ts.URL+"/_/api/groups/default/diffs", AddDiffRequest{Content: sampleDiff}, &added)
	file := added.Diff.Files[0]
	resp := postJSON(t, ts.URL+"/_/api/groups/default/comments", AddCommentRequest{
		DiffID: added.Diff.ID, FileID: file.ID, Path: file.Path(),
		Side: "old", StartLine: 2, EndLine: 2, Suggestion: "nope",
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %s, want 400 for a suggestion on the old side", resp.Status)
	}
}

func TestEmptyCommentRejected(t *testing.T) {
	ts, _ := newTestServer(t)
	var added AddDiffResponse
	postJSON(t, ts.URL+"/_/api/groups/default/diffs", AddDiffRequest{Content: sampleDiff}, &added)
	resp := postJSON(t, ts.URL+"/_/api/groups/default/comments", AddCommentRequest{
		DiffID: added.Diff.ID, FileID: added.Diff.Files[0].ID, Path: "README.md", StartLine: 1,
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %s, want 400 for a comment with neither body nor suggestion", resp.Status)
	}
}

func TestUpdateCommentSuggestion(t *testing.T) {
	ts, _ := newTestServer(t)
	var added AddDiffResponse
	postJSON(t, ts.URL+"/_/api/groups/default/diffs", AddDiffRequest{Content: sampleDiff}, &added)
	var comment model.Comment
	postJSON(t, ts.URL+"/_/api/groups/default/comments", AddCommentRequest{
		DiffID: added.Diff.ID, FileID: added.Diff.Files[0].ID, Path: "README.md",
		Side: "new", StartLine: 2, EndLine: 2, Body: "hmm",
	}, &comment)

	req, err := http.NewRequest(http.MethodPatch,
		ts.URL+"/_/api/groups/default/comments/"+comment.ID,
		strings.NewReader(`{"suggestion":"replaced"}`))
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var updated model.Comment
	if err := json.NewDecoder(res.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	if updated.Suggestion != "replaced" || updated.Body != "hmm" {
		t.Errorf("updated = %+v, want the suggestion added and the body kept", updated)
	}
}
