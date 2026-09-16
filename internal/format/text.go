package format

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/78tacos/gh-digest/internal/digest"
)

// Formatter renders a Digest as text or JSON.
type Formatter struct{}

func (Formatter) JSON(w io.Writer, d digest.Digest) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(d)
}

func (f Formatter) Text(w io.Writer, d digest.Digest) error {
	header := fmt.Sprintf("GitHub digest for @%s  ·  %s", d.User.Login, d.GeneratedAt.UTC().Format("2 Jan 2006 15:04 UTC"))
	if _, err := fmt.Fprintln(w, header); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	if d.Notifications != nil {
		if err := f.writeNotifications(w, d.GeneratedAt, *d.Notifications); err != nil {
			return err
		}
	}
	if d.PullRequests != nil {
		if d.Notifications != nil {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if err := f.writePRs(w, d.GeneratedAt, *d.PullRequests); err != nil {
			return err
		}
	}
	return nil
}

func (Formatter) writeNotifications(w io.Writer, now time.Time, n digest.NotificationSection) error {
	if _, err := fmt.Fprintf(w, "Notifications  %d unread · %d shown\n", n.Unread, n.Shown); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, strings.Repeat("-", 40)); err != nil {
		return err
	}
	if len(n.Repos) == 0 {
		_, err := fmt.Fprintln(w, "  (none)")
		return err
	}
	for _, repo := range n.Repos {
		if _, err := fmt.Fprintln(w, repo.Repo); err != nil {
			return err
		}
		for _, item := range repo.Items {
			if _, err := fmt.Fprintln(w, formatNotification(now, item)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (Formatter) writePRs(w io.Writer, now time.Time, prs digest.PRSection) error {
	if _, err := fmt.Fprintln(w, "Open pull requests"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, strings.Repeat("-", 40)); err != nil {
		return err
	}

	type bucket struct {
		title string
		items []digest.PullRequest
	}
	sections := []bucket{
		{"Authored", prs.Authored},
		{"Review requested", prs.ReviewRequested},
		{"Assigned", prs.Assigned},
	}
	for i, section := range sections {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "%s (%d)\n", section.title, len(section.items)); err != nil {
			return err
		}
		if len(section.items) == 0 {
			if _, err := fmt.Fprintln(w, "  (none)"); err != nil {
				return err
			}
			continue
		}
		for _, pr := range section.items {
			if _, err := fmt.Fprintln(w, formatPR(now, pr)); err != nil {
				return err
			}
		}
	}
	return nil
}

func formatNotification(now time.Time, n digest.Notification) string {
	kind := digest.TypeLabel(n.Type)
	if n.Number > 0 {
		kind = kind + " #" + strconv.Itoa(n.Number)
	}
	unread := " "
	if n.Unread {
		unread = "*"
	}
	return fmt.Sprintf("  %s %-18s %-12s %s  %s",
		unread,
		truncate(digest.ReasonLabel(n.Reason), 18),
		kind,
		truncate(n.Title, 42),
		digest.Relative(now, n.UpdatedAt),
	)
}

func formatPR(now time.Time, pr digest.PullRequest) string {
	comments := "no comments"
	switch pr.Comments {
	case 1:
		comments = "1 comment"
	default:
		if pr.Comments > 1 {
			comments = strconv.Itoa(pr.Comments) + " comments"
		}
	}
	return fmt.Sprintf("  %s#%d  %s  %s  %s",
		pr.Repo,
		pr.Number,
		truncate(pr.Title, 40),
		digest.Relative(now, pr.UpdatedAt),
		comments,
	)
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 || len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return strings.TrimSpace(s[:n-3]) + "..."
}
