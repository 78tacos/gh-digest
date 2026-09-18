package digest

import (
	"testing"
	"time"
)

func fixtureTime(now time.Time, ago time.Duration) time.Time {
	return now.Add(-ago)
}

func testSnapshot(now time.Time) Snapshot {
	return Snapshot{
		User: User{Login: "octocat"},
		Notifications: []Notification{
			{
				ID: "1", Unread: true, Reason: "review_requested", Title: "Please review auth",
				Type: "PullRequest", Repo: "octo/hello", Number: 9,
				HTMLURL:   "https://github.com/octo/hello/pull/9",
				UpdatedAt: fixtureTime(now, 30*time.Minute),
			},
			{
				ID: "2", Unread: true, Reason: "mention", Title: "Add digest CLI",
				Type: "PullRequest", Repo: "acme/api", Number: 88,
				HTMLURL:   "https://github.com/acme/api/pull/88",
				UpdatedAt: fixtureTime(now, 2*time.Hour),
			},
			{
				ID: "3", Unread: true, Reason: "comment", Title: "Rate limit retries",
				Type: "Issue", Repo: "acme/api", Number: 41,
				HTMLURL:   "https://github.com/acme/api/issues/41",
				UpdatedAt: fixtureTime(now, 5*time.Hour),
			},
			{
				ID: "4", Unread: false, Reason: "subscribed", Title: "Design tokens",
				Type: "Issue", Repo: "acme/web", Number: 2,
				HTMLURL:   "https://github.com/acme/web/issues/2",
				UpdatedAt: fixtureTime(now, 72*time.Hour),
			},
			{
				ID: "5", Unread: true, Reason: "ci_activity", Title: "CI failed on main",
				Type: "CheckSuite", Repo: "acme/api", Number: 0,
				UpdatedAt: fixtureTime(now, 10*24*time.Hour),
			},
		},
		Authored: []PullRequest{
			{
				Repo: "acme/api", Number: 88, Title: "Add digest CLI",
				HTMLURL: "https://github.com/acme/api/pull/88", Author: "octocat",
				Comments: 3, UpdatedAt: fixtureTime(now, 2*time.Hour),
			},
			{
				Repo: "acme/api", Number: 88, Title: "Add digest CLI (dup)",
				HTMLURL: "https://github.com/acme/api/pull/88", Author: "octocat",
				Comments: 3, UpdatedAt: fixtureTime(now, 2*time.Hour),
			},
		},
		ReviewRequested: []PullRequest{
			{
				Repo: "acme/api", Number: 88, Title: "Add digest CLI",
				HTMLURL: "https://github.com/acme/api/pull/88", Author: "octocat",
				Comments: 3, UpdatedAt: fixtureTime(now, 2*time.Hour),
			},
			{
				Repo: "octo/hello", Number: 9, Title: "Please review auth",
				HTMLURL: "https://github.com/octo/hello/pull/9", Author: "hubot",
				Comments: 1, UpdatedAt: fixtureTime(now, 30*time.Minute),
			},
		},
		Assigned: []PullRequest{
			{
				Repo: "octo/hello", Number: 9, Title: "Please review auth",
				HTMLURL: "https://github.com/octo/hello/pull/9", Author: "hubot",
				Comments: 1, UpdatedAt: fixtureTime(now, 30*time.Minute),
			},
			{
				Repo: "acme/web", Number: 15, Title: "Fix nav",
				HTMLURL: "https://github.com/acme/web/pull/15", Author: "hubot",
				Comments: 0, UpdatedAt: fixtureTime(now, 24*time.Hour),
			},
		},
	}
}

func TestBuilderGroupsNotificationsByRepoAndDropsRead(t *testing.T) {
	now := time.Date(2026, 9, 16, 16, 0, 0, 0, time.UTC)
	b := Builder{Opts: Options{
		Now:                  now,
		NotificationLimit:    50,
		IncludeRead:          false,
		IncludeNotifications: true,
		IncludePRs:           false,
	}}

	d := b.Build(testSnapshot(now))
	if d.User.Login != "octocat" {
		t.Fatalf("user = %q", d.User.Login)
	}
	if d.Notifications == nil {
		t.Fatal("expected notifications section")
	}
	if d.PullRequests != nil {
		t.Fatal("did not request PRs")
	}
	n := d.Notifications
	if n.Unread != 4 {
		t.Fatalf("unread = %d, want 4", n.Unread)
	}
	if n.Shown != 4 || n.Total != 4 {
		t.Fatalf("shown=%d total=%d, want 4/4 (read item dropped)", n.Shown, n.Total)
	}
	if len(n.Repos) != 2 {
		t.Fatalf("repos = %d, want 2 (acme/web dropped because read)", len(n.Repos))
	}
	if n.Repos[0].Repo != "octo/hello" {
		t.Fatalf("first repo = %q, want most recently updated", n.Repos[0].Repo)
	}
	if n.Repos[1].Repo != "acme/api" {
		t.Fatalf("second repo = %q", n.Repos[1].Repo)
	}
	if len(n.Repos[1].Items) != 3 {
		t.Fatalf("acme/api items = %d, want 3", len(n.Repos[1].Items))
	}
	if n.Repos[1].Items[0].Number != 88 {
		t.Fatalf("first acme/api item = #%d, want newest (#88)", n.Repos[1].Items[0].Number)
	}
}

func TestBuilderIncludesReadWhenRequested(t *testing.T) {
	now := time.Date(2026, 9, 16, 16, 0, 0, 0, time.UTC)
	b := Builder{Opts: Options{
		Now:                  now,
		NotificationLimit:    50,
		IncludeRead:          true,
		IncludeNotifications: true,
	}}
	d := b.Build(testSnapshot(now))
	if d.Notifications.Shown != 5 {
		t.Fatalf("shown = %d, want 5", d.Notifications.Shown)
	}
	found := false
	for _, repo := range d.Notifications.Repos {
		if repo.Repo == "acme/web" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected acme/web when including read notifications")
	}
}

func TestBuilderAppliesLimitAndSince(t *testing.T) {
	now := time.Date(2026, 9, 16, 16, 0, 0, 0, time.UTC)
	b := Builder{Opts: Options{
		Now:                  now,
		NotificationLimit:    2,
		IncludeRead:          true,
		Since:                now.Add(-6 * time.Hour),
		IncludeNotifications: true,
	}}
	d := b.Build(testSnapshot(now))
	n := d.Notifications
	if n.Total != 3 {
		t.Fatalf("total after since = %d, want 3 (drops 3d and 10d items)", n.Total)
	}
	if n.Shown != 2 {
		t.Fatalf("shown = %d, want 2 (limit)", n.Shown)
	}
	if n.Repos[0].Items[0].Number != 9 {
		t.Fatalf("newest shown = #%d, want #9", n.Repos[0].Items[0].Number)
	}
}

func TestBuilderLimitZeroShowsNone(t *testing.T) {
	now := time.Date(2026, 9, 16, 16, 0, 0, 0, time.UTC)
	b := Builder{Opts: Options{
		Now:                  now,
		NotificationLimit:    0,
		IncludeNotifications: true,
	}}
	d := b.Build(testSnapshot(now))
	n := d.Notifications
	if n.Total != 4 {
		t.Fatalf("total = %d, want 4 unread", n.Total)
	}
	if n.Shown != 0 || len(n.Repos) != 0 {
		t.Fatalf("shown=%d repos=%d, want 0/0", n.Shown, len(n.Repos))
	}
}

func TestBuilderClassifiesPRsExclusively(t *testing.T) {
	now := time.Date(2026, 9, 16, 16, 0, 0, 0, time.UTC)
	b := Builder{Opts: Options{
		Now:               now,
		NotificationLimit: 50,
		IncludePRs:        true,
	}}
	d := b.Build(testSnapshot(now))
	prs := d.PullRequests
	if prs == nil {
		t.Fatal("expected pull request section")
	}
	if len(prs.Authored) != 1 || prs.Authored[0].Key() != "acme/api#88" {
		t.Fatalf("authored = %+v, want unique acme/api#88", prs.Authored)
	}
	if len(prs.ReviewRequested) != 1 || prs.ReviewRequested[0].Key() != "octo/hello#9" {
		t.Fatalf("review = %+v, want octo/hello#9 (authored PR excluded)", prs.ReviewRequested)
	}
	if len(prs.Assigned) != 1 || prs.Assigned[0].Key() != "acme/web#15" {
		t.Fatalf("assigned = %+v, want acme/web#15 (authored and review excluded)", prs.Assigned)
	}
}

func TestBuilderEmptySnapshot(t *testing.T) {
	b := Builder{Opts: Options{
		Now:                  time.Date(2026, 9, 16, 16, 0, 0, 0, time.UTC),
		NotificationLimit:    50,
		IncludeNotifications: true,
		IncludePRs:           true,
	}}
	d := b.Build(Snapshot{User: User{Login: "octocat"}})
	if d.Notifications.Unread != 0 || d.Notifications.Shown != 0 || len(d.Notifications.Repos) != 0 {
		t.Fatalf("expected empty notifications, got %+v", d.Notifications)
	}
	if len(d.PullRequests.Authored)+len(d.PullRequests.ReviewRequested)+len(d.PullRequests.Assigned) != 0 {
		t.Fatalf("expected empty PRs, got %+v", d.PullRequests)
	}
}

func TestRelative(t *testing.T) {
	now := time.Date(2026, 9, 16, 16, 0, 0, 0, time.UTC)
	cases := []struct {
		ago  time.Duration
		want string
	}{
		{30 * time.Second, "just now"},
		{5 * time.Minute, "5m"},
		{2 * time.Hour, "2h"},
		{3 * 24 * time.Hour, "3d"},
		{40 * 24 * time.Hour, "2026-08-07"},
	}
	for _, tc := range cases {
		got := Relative(now, now.Add(-tc.ago))
		if got != tc.want {
			t.Fatalf("ago %s: got %q want %q", tc.ago, got, tc.want)
		}
	}
}

func TestTypeAndReasonLabels(t *testing.T) {
	if TypeLabel("PullRequest") != "PR" {
		t.Fatalf("PullRequest label = %q", TypeLabel("PullRequest"))
	}
	if TypeLabel("") != "Item" {
		t.Fatalf("empty type label = %q", TypeLabel(""))
	}
	if ReasonLabel("review_requested") != "review requested" {
		t.Fatalf("reason label = %q", ReasonLabel("review_requested"))
	}
	if ReasonLabel("mystery") != "mystery" {
		t.Fatalf("unknown reason should pass through, got %q", ReasonLabel("mystery"))
	}
}
