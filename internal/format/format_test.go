package format

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/78tacos/gh-digest/internal/digest"
)

func sampleDigest(now time.Time) digest.Digest {
	b := digest.Builder{Opts: digest.Options{
		Now:                  now,
		IncludeRead:          false,
		IncludeNotifications: true,
		IncludePRs:           true,
	}}
	return b.Build(digest.Snapshot{
		User: digest.User{Login: "octocat"},
		Notifications: []digest.Notification{
			{
				ID: "1", Unread: true, Reason: "mention", Title: "Add digest CLI",
				Type: "PullRequest", Repo: "acme/api", Number: 88,
				UpdatedAt: now.Add(-2 * time.Hour),
			},
		},
		Authored: []digest.PullRequest{
			{
				Repo: "acme/api", Number: 88, Title: "Add digest CLI",
				HTMLURL: "https://github.com/acme/api/pull/88", Author: "octocat",
				Comments: 3, UpdatedAt: now.Add(-2 * time.Hour),
			},
		},
	})
}

func TestTextContainsCoreSections(t *testing.T) {
	now := time.Date(2026, 9, 16, 16, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	if err := (Formatter{}).Text(&buf, sampleDigest(now)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"GitHub digest for @octocat",
		"16 Sep 2026 16:00 UTC",
		"Notifications  1 unread · 1 shown",
		"acme/api",
		"mention",
		"PR #88",
		"Add digest CLI",
		"Open pull requests",
		"Authored (1)",
		"acme/api#88",
		"Review requested (0)",
		"Assigned (0)",
		"3 comments",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("text missing %q\n%s", want, out)
		}
	}
}

func TestTextEmptyStates(t *testing.T) {
	now := time.Date(2026, 9, 16, 16, 0, 0, 0, time.UTC)
	d := digest.Builder{Opts: digest.Options{
		Now:                  now,
		IncludeNotifications: true,
		IncludePRs:           true,
	}}.Build(digest.Snapshot{User: digest.User{Login: "octocat"}})

	var buf bytes.Buffer
	if err := (Formatter{}).Text(&buf, d); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "(none)") {
		t.Fatalf("expected empty-state marker, got:\n%s", out)
	}
}

func TestJSONShape(t *testing.T) {
	now := time.Date(2026, 9, 16, 16, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	if err := (Formatter{}).JSON(&buf, sampleDigest(now)); err != nil {
		t.Fatal(err)
	}
	var parsed digest.Digest
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.User.Login != "octocat" {
		t.Fatalf("login = %q", parsed.User.Login)
	}
	if parsed.Notifications == nil || parsed.Notifications.Unread != 1 {
		t.Fatalf("notifications = %+v", parsed.Notifications)
	}
	if parsed.PullRequests == nil || len(parsed.PullRequests.Authored) != 1 {
		t.Fatalf("prs = %+v", parsed.PullRequests)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 40); got != "short" {
		t.Fatalf("got %q", got)
	}
	got := truncate("abcdefghijklmnopqrstuvwxyz", 10)
	if got != "abcdefg..." {
		t.Fatalf("got %q", got)
	}
}
