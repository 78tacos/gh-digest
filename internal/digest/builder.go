package digest

import (
	"sort"
	"strconv"
	"time"
)

// Builder reduces a GitHub Snapshot into a Digest.
type Builder struct {
	Opts Options
}

func (b Builder) now() time.Time {
	if b.Opts.Now.IsZero() {
		return time.Now().UTC()
	}
	return b.Opts.Now.UTC()
}

// Build groups notifications by repo, caps the list, and classifies open PRs
// into exclusive buckets (authored wins, then review requested, then assigned).
func (b Builder) Build(snap Snapshot) Digest {
	d := Digest{
		GeneratedAt: b.now(),
		User:        snap.User,
	}
	if b.Opts.IncludeNotifications {
		section := b.buildNotifications(snap.Notifications)
		d.Notifications = &section
	}
	if b.Opts.IncludePRs {
		section := b.buildPRs(snap)
		d.PullRequests = &section
	}
	return d
}

func (b Builder) buildNotifications(items []Notification) NotificationSection {
	filtered := make([]Notification, 0, len(items))
	unread := 0
	since := b.Opts.Since
	for _, n := range items {
		if n.Unread {
			unread++
		}
		if !b.Opts.IncludeRead && !n.Unread {
			continue
		}
		if !since.IsZero() && n.UpdatedAt.Before(since) {
			continue
		}
		filtered = append(filtered, n)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].UpdatedAt.After(filtered[j].UpdatedAt)
	})

	total := len(filtered)
	limit := b.Opts.NotificationLimit
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	groups := b.groupByRepo(filtered)
	return NotificationSection{
		Unread: unread,
		Shown:  len(filtered),
		Total:  total,
		Repos:  groups,
	}
}

func (b Builder) groupByRepo(items []Notification) []RepoNotifications {
	order := make([]string, 0)
	index := make(map[string]int)
	for _, n := range items {
		repo := n.Repo
		if repo == "" {
			repo = "unknown"
		}
		if _, ok := index[repo]; !ok {
			index[repo] = len(order)
			order = append(order, repo)
		}
	}

	out := make([]RepoNotifications, len(order))
	for i, repo := range order {
		out[i] = RepoNotifications{Repo: repo}
	}
	for _, n := range items {
		repo := n.Repo
		if repo == "" {
			repo = "unknown"
		}
		i := index[repo]
		out[i].Items = append(out[i].Items, n)
	}
	return out
}

func (b Builder) buildPRs(snap Snapshot) PRSection {
	authored := b.sortPRs(b.uniquePRs(snap.Authored))
	authoredKeys := make(map[string]struct{}, len(authored))
	for _, pr := range authored {
		authoredKeys[pr.Key()] = struct{}{}
	}

	review := make([]PullRequest, 0, len(snap.ReviewRequested))
	reviewKeys := make(map[string]struct{})
	for _, pr := range b.sortPRs(b.uniquePRs(snap.ReviewRequested)) {
		if _, ok := authoredKeys[pr.Key()]; ok {
			continue
		}
		review = append(review, pr)
		reviewKeys[pr.Key()] = struct{}{}
	}

	assigned := make([]PullRequest, 0, len(snap.Assigned))
	for _, pr := range b.sortPRs(b.uniquePRs(snap.Assigned)) {
		key := pr.Key()
		if _, ok := authoredKeys[key]; ok {
			continue
		}
		if _, ok := reviewKeys[key]; ok {
			continue
		}
		assigned = append(assigned, pr)
	}

	return PRSection{
		Authored:        authored,
		ReviewRequested: review,
		Assigned:        assigned,
	}
}

func (Builder) uniquePRs(items []PullRequest) []PullRequest {
	seen := make(map[string]struct{}, len(items))
	out := make([]PullRequest, 0, len(items))
	for _, pr := range items {
		key := pr.Key()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, pr)
	}
	return out
}

func (Builder) sortPRs(items []PullRequest) []PullRequest {
	sorted := append([]PullRequest(nil), items...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].UpdatedAt.After(sorted[j].UpdatedAt)
	})
	return sorted
}

// Relative formats t as a short age relative to now, e.g. "2h" or "3d".
func Relative(now, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	d := now.Sub(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return strconv.Itoa(int(d.Minutes())) + "m"
	case d < 24*time.Hour:
		return strconv.Itoa(int(d.Hours())) + "h"
	case d < 30*24*time.Hour:
		return strconv.Itoa(int(d.Hours()/24)) + "d"
	default:
		return t.UTC().Format("2006-01-02")
	}
}

// TypeLabel maps GitHub subject types to a short digest label.
func TypeLabel(subjectType string) string {
	switch subjectType {
	case "PullRequest":
		return "PR"
	case "Issue":
		return "Issue"
	case "Release":
		return "Release"
	case "Discussion":
		return "Discussion"
	case "CheckSuite":
		return "Check"
	case "RepositoryInvitation":
		return "Invite"
	case "":
		return "Item"
	default:
		return subjectType
	}
}

// ReasonLabel maps a notification reason to a readable phrase.
func ReasonLabel(reason string) string {
	switch reason {
	case "review_requested":
		return "review requested"
	case "approval_requested":
		return "approval requested"
	case "team_mention":
		return "team mention"
	case "security_alert":
		return "security alert"
	case "state_change":
		return "state change"
	case "ci_activity":
		return "ci"
	case "assign":
		return "assigned"
	case "author":
		return "author"
	case "comment":
		return "comment"
	case "mention":
		return "mention"
	case "subscribed":
		return "subscribed"
	case "manual":
		return "manual"
	case "invitation":
		return "invitation"
	case "":
		return "notification"
	default:
		return reason
	}
}
