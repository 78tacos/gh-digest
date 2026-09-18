package digest

import (
	"strconv"
	"time"
)

// User is the authenticated GitHub account the digest is built for.
type User struct {
	Login string `json:"login"`
}

// Notification is a single GitHub notification thread.
type Notification struct {
	ID        string    `json:"id"`
	Unread    bool      `json:"unread"`
	Reason    string    `json:"reason"`
	Title     string    `json:"title"`
	Type      string    `json:"type"`
	Repo      string    `json:"repo"`
	Number    int       `json:"number,omitempty"`
	HTMLURL   string    `json:"html_url,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PullRequest is an open pull request involving the user.
type PullRequest struct {
	Repo      string    `json:"repo"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	HTMLURL   string    `json:"html_url"`
	Author    string    `json:"author"`
	Comments  int       `json:"comments"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Snapshot is raw GitHub data before digesting.
type Snapshot struct {
	User            User
	Notifications   []Notification
	Authored        []PullRequest
	ReviewRequested []PullRequest
	Assigned        []PullRequest
}

// NotificationSection is the digested notification list.
type NotificationSection struct {
	Unread int                 `json:"unread"`
	Shown  int                 `json:"shown"`
	Total  int                 `json:"total"`
	Repos  []RepoNotifications `json:"repos"`
}

// RepoNotifications groups notification items under one repository.
type RepoNotifications struct {
	Repo  string         `json:"repo"`
	Items []Notification `json:"items"`
}

// PRSection is the digested open-PR list, split into exclusive buckets.
type PRSection struct {
	Authored        []PullRequest `json:"authored"`
	ReviewRequested []PullRequest `json:"review_requested"`
	Assigned        []PullRequest `json:"assigned"`
}

// Digest is the v1 summary produced by Builder.
type Digest struct {
	GeneratedAt   time.Time            `json:"generated_at"`
	User          User                 `json:"user"`
	Notifications *NotificationSection `json:"notifications,omitempty"`
	PullRequests  *PRSection           `json:"pull_requests,omitempty"`
}

// Options control how a Snapshot is reduced into a Digest.
type Options struct {
	Now                  time.Time
	NotificationLimit    int
	IncludeRead          bool
	Since                time.Time
	IncludeNotifications bool
	IncludePRs           bool
}

func (p PullRequest) Key() string {
	return p.Repo + "#" + strconv.Itoa(p.Number)
}
