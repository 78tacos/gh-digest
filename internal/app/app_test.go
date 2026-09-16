package app

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/78tacos/gh-digest/internal/digest"
	"github.com/78tacos/gh-digest/internal/github"
)

type stubSource struct {
	snap digest.Snapshot
	err  error
	opts github.LoadOptions
}

func (s *stubSource) Load(_ context.Context, opts github.LoadOptions) (digest.Snapshot, error) {
	s.opts = opts
	return s.snap, s.err
}

func TestRunHelpAndVersion(t *testing.T) {
	a := &App{}
	var out bytes.Buffer
	if code := a.Run([]string{"--help"}, &out, &out); code != 0 {
		t.Fatalf("help exit %d", code)
	}
	if !strings.Contains(out.String(), "gh-digest summarizes") {
		t.Fatalf("help = %s", out.String())
	}

	out.Reset()
	if code := a.Run([]string{"--version"}, &out, &out); code != 0 {
		t.Fatalf("version exit %d", code)
	}
	if strings.TrimSpace(out.String()) != Version {
		t.Fatalf("version = %q", out.String())
	}
}

func TestRunUnknownFlag(t *testing.T) {
	var errBuf bytes.Buffer
	code := (&App{}).Run([]string{"--nope"}, ioDiscard{}, &errBuf)
	if code != 2 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(errBuf.String(), "unknown flag") {
		t.Fatalf("stderr = %s", errBuf.String())
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

func TestRunTextAndJSON(t *testing.T) {
	now := time.Date(2026, 9, 16, 16, 0, 0, 0, time.UTC)
	src := &stubSource{snap: digest.Snapshot{
		User: digest.User{Login: "octocat"},
		Notifications: []digest.Notification{{
			ID: "1", Unread: true, Reason: "mention", Title: "Hello",
			Type: "Issue", Repo: "acme/api", Number: 1, UpdatedAt: now.Add(-time.Hour),
		}},
		Authored: []digest.PullRequest{{
			Repo: "acme/api", Number: 2, Title: "Work", Author: "octocat",
			UpdatedAt: now.Add(-2 * time.Hour),
		}},
	}}
	a := &App{Source: src, Now: now}

	var out bytes.Buffer
	if code := a.Run(nil, &out, &out); code != 0 {
		t.Fatalf("text exit %d: %s", code, out.String())
	}
	if !strings.Contains(out.String(), "@octocat") || !strings.Contains(out.String(), "acme/api#2") {
		t.Fatalf("text = %s", out.String())
	}

	out.Reset()
	if code := a.Run([]string{"--json", "--prs-only"}, &out, &out); code != 0 {
		t.Fatalf("json exit %d: %s", code, out.String())
	}
	if strings.Contains(out.String(), `"notifications"`) {
		t.Fatalf("prs-only JSON still has notifications: %s", out.String())
	}
	if !strings.Contains(out.String(), `"pull_requests"`) {
		t.Fatalf("json = %s", out.String())
	}
}

func TestRunNotifsOnlyAndAll(t *testing.T) {
	now := time.Date(2026, 9, 16, 16, 0, 0, 0, time.UTC)
	src := &stubSource{snap: digest.Snapshot{
		User: digest.User{Login: "octocat"},
		Notifications: []digest.Notification{
			{ID: "1", Unread: false, Reason: "subscribed", Title: "Old", Type: "Issue", Repo: "acme/web", Number: 3, UpdatedAt: now.Add(-time.Hour)},
		},
	}}
	a := &App{Source: src, Now: now}
	var out bytes.Buffer
	if code := a.Run([]string{"--notifs-only", "--all"}, &out, &out); code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	if !src.opts.All {
		t.Fatal("expected Load All=true")
	}
	if !strings.Contains(out.String(), "acme/web") {
		t.Fatalf("expected read notification included, got %s", out.String())
	}
	if strings.Contains(out.String(), "Open pull requests") {
		t.Fatalf("notifs-only still printed PRs: %s", out.String())
	}
}

func TestRunSourceError(t *testing.T) {
	src := &stubSource{err: errors.New("boom")}
	var errBuf bytes.Buffer
	code := (&App{Source: src}).Run(nil, ioDiscard{}, &errBuf)
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(errBuf.String(), "boom") {
		t.Fatalf("stderr = %s", errBuf.String())
	}
}

func TestParseSince(t *testing.T) {
	now := time.Date(2026, 9, 16, 16, 0, 0, 0, time.UTC)
	got, err := ParseSince("24h", now)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(now.Add(-24 * time.Hour)) {
		t.Fatalf("24h = %s", got)
	}
	got, err = ParseSince("7d", now)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(now.Add(-7 * 24 * time.Hour)) {
		t.Fatalf("7d = %s", got)
	}
	got, err = ParseSince("2026-09-01T00:00:00Z", now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Year() != 2026 || got.Month() != 9 || got.Day() != 1 {
		t.Fatalf("rfc3339 = %s", got)
	}
	if _, err := ParseSince("nope", now); err == nil {
		t.Fatal("expected error")
	}
}

func TestConflictingSectionFlags(t *testing.T) {
	var errBuf bytes.Buffer
	code := (&App{}).Run([]string{"--prs-only", "--notifs-only"}, ioDiscard{}, &errBuf)
	if code != 2 {
		t.Fatalf("exit %d", code)
	}
}
