package github

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	outputs map[string][]byte
	errs    map[string]error
	calls   []string
}

func (f *fakeRunner) CombinedOutput(_ context.Context, name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, key)
	if err, ok := f.errs[key]; ok {
		return f.outputs[key], err
	}
	if out, ok := f.outputs[key]; ok {
		return out, nil
	}
	// Match by API path (args[1] after "api").
	if len(args) >= 2 && args[0] == "api" {
		pathKey := args[1]
		for k, out := range f.outputs {
			if strings.Contains(k, "api "+pathKey) {
				if err, ok := f.errs[k]; ok {
					return out, err
				}
				return out, nil
			}
		}
	}
	return nil, errors.New("unexpected command: " + key)
}

func TestClientLoadParsesAPIPayloads(t *testing.T) {
	runner := &fakeRunner{outputs: map[string][]byte{
		"gh api user": []byte(`{"login":"octocat"}`),
		"gh api notifications": []byte(`[
			{
				"id": "1",
				"unread": true,
				"reason": "mention",
				"updated_at": "2026-09-16T14:00:00Z",
				"subject": {
					"title": "Add digest CLI",
					"url": "https://api.github.com/repos/acme/api/issues/88",
					"type": "PullRequest"
				},
				"repository": {
					"full_name": "acme/api",
					"html_url": "https://github.com/acme/api"
				}
			}
		]`),
		"gh api search/issues": []byte(`{
			"items": [
				{
					"title": "Add digest CLI",
					"number": 88,
					"html_url": "https://github.com/acme/api/pull/88",
					"updated_at": "2026-09-16T14:00:00Z",
					"comments": 3,
					"repository_url": "https://api.github.com/repos/acme/api",
					"pull_request": {"url": "https://api.github.com/repos/acme/api/pulls/88"},
					"user": {"login": "octocat"}
				}
			]
		}`),
	}}

	c := &Client{Runner: runner}
	snap, err := c.Load(context.Background(), LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if snap.User.Login != "octocat" {
		t.Fatalf("login = %q", snap.User.Login)
	}
	if len(snap.Notifications) != 1 {
		t.Fatalf("notifications = %d", len(snap.Notifications))
	}
	n := snap.Notifications[0]
	if n.Repo != "acme/api" || n.Number != 88 || n.HTMLURL != "https://github.com/acme/api/pull/88" {
		t.Fatalf("notification = %+v", n)
	}
	if len(snap.Authored) != 1 || snap.Authored[0].Author != "octocat" {
		t.Fatalf("authored = %+v", snap.Authored)
	}
	if len(snap.ReviewRequested) != 1 || len(snap.Assigned) != 1 {
		t.Fatalf("expected each PR search to return 1 item")
	}
}

func TestClientLoadAuthError(t *testing.T) {
	runner := &fakeRunner{
		outputs: map[string][]byte{
			"gh api user": []byte(`{"message":"Requires authentication"}`),
		},
		errs: map[string]error{
			"gh api user": errors.New("exit status 1: HTTP 401"),
		},
	}
	_, err := (&Client{Runner: runner}).Load(context.Background(), LoadOptions{})
	if err == nil || !strings.Contains(err.Error(), "gh auth login") {
		t.Fatalf("got %v, want auth hint", err)
	}
}

func TestClientLoadMissingGh(t *testing.T) {
	runner := &fakeRunner{
		errs: map[string]error{
			"gh api user": &exec.Error{Name: "gh", Err: exec.ErrNotFound},
		},
		outputs: map[string][]byte{},
	}
	_, err := (&Client{Runner: runner}).Load(context.Background(), LoadOptions{})
	if err == nil || !strings.Contains(err.Error(), "GitHub CLI") {
		t.Fatalf("got %v, want install hint", err)
	}
}

func TestWrapGHErrorIntegrationToken(t *testing.T) {
	err := wrapGHError(
		errors.New("exit status 1"),
		[]byte(`{"message":"Resource not accessible by integration","status":"403"}gh: Resource not accessible by integration (HTTP 403)`),
	)
	if err == nil || !strings.Contains(err.Error(), "gh auth login") {
		t.Fatalf("got %v", err)
	}
	if strings.Contains(err.Error(), "documentation_url") {
		t.Fatalf("raw JSON leaked: %v", err)
	}
}

func TestParseTrailingNumberAndWebURL(t *testing.T) {
	if got := parseTrailingNumber("https://api.github.com/repos/acme/api/issues/41"); got != 41 {
		t.Fatalf("got %d", got)
	}
	if got := webURL("https://github.com/acme/api", "Issue", 41); got != "https://github.com/acme/api/issues/41" {
		t.Fatalf("got %q", got)
	}
	if got := repoFromGitHubURL("https://github.com/acme/api/pull/88"); got != "acme/api" {
		t.Fatalf("got %q", got)
	}
}

func TestNotificationsPassFlags(t *testing.T) {
	runner := &fakeRunner{outputs: map[string][]byte{
		"gh api user":          []byte(`{"login":"octocat"}`),
		"gh api notifications": []byte(`[]`),
		"gh api search/issues": []byte(`{"items":[]}`),
	}}
	since := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	_, err := (&Client{Runner: runner}).Load(context.Background(), LoadOptions{
		All:           true,
		Participating: true,
		Since:         since,
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(runner.calls, "\n")
	if !strings.Contains(joined, "all=true") || !strings.Contains(joined, "participating=true") {
		t.Fatalf("missing notification flags in calls:\n%s", joined)
	}
	if !strings.Contains(joined, "since=2026-09-15T00:00:00Z") {
		t.Fatalf("missing since in calls:\n%s", joined)
	}
}
