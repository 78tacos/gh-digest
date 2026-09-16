package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/78tacos/gh-digest/internal/digest"
)

// LoadOptions control which notification threads are fetched from GitHub.
type LoadOptions struct {
	All           bool
	Participating bool
	Since         time.Time
}

// Source loads a Snapshot for the authenticated user.
type Source interface {
	Load(ctx context.Context, opts LoadOptions) (digest.Snapshot, error)
}

// Runner executes a command and returns combined stdout/stderr.
type Runner interface {
	CombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ExecRunner runs processes on the local machine.
type ExecRunner struct{}

func (ExecRunner) CombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

// Client loads notifications and open PRs via the GitHub CLI (`gh api`).
type Client struct {
	Runner Runner
}

func NewClient() *Client {
	return &Client{Runner: ExecRunner{}}
}

func (c *Client) runner() Runner {
	if c.Runner != nil {
		return c.Runner
	}
	return ExecRunner{}
}

func (c *Client) Load(ctx context.Context, opts LoadOptions) (digest.Snapshot, error) {
	user, err := c.currentUser(ctx)
	if err != nil {
		return digest.Snapshot{}, err
	}

	notifs, err := c.notifications(ctx, opts)
	if err != nil {
		return digest.Snapshot{}, err
	}

	authored, err := c.searchPRs(ctx, "is:open is:pr author:@me")
	if err != nil {
		return digest.Snapshot{}, err
	}
	review, err := c.searchPRs(ctx, "is:open is:pr review-requested:@me")
	if err != nil {
		return digest.Snapshot{}, err
	}
	assigned, err := c.searchPRs(ctx, "is:open is:pr assignee:@me")
	if err != nil {
		return digest.Snapshot{}, err
	}

	return digest.Snapshot{
		User:            user,
		Notifications:   notifs,
		Authored:        authored,
		ReviewRequested: review,
		Assigned:        assigned,
	}, nil
}

func (c *Client) currentUser(ctx context.Context) (digest.User, error) {
	body, err := c.api(ctx, []string{"user"})
	if err != nil {
		return digest.User{}, err
	}
	var raw struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return digest.User{}, fmt.Errorf("decode user: %w", err)
	}
	if raw.Login == "" {
		return digest.User{}, fmt.Errorf("GitHub user response was missing login")
	}
	return digest.User{Login: raw.Login}, nil
}

func (c *Client) notifications(ctx context.Context, opts LoadOptions) ([]digest.Notification, error) {
	args := []string{"notifications", "-F", "per_page=100"}
	if opts.All {
		args = append(args, "-F", "all=true")
	}
	if opts.Participating {
		args = append(args, "-F", "participating=true")
	}
	if !opts.Since.IsZero() {
		args = append(args, "-F", "since="+opts.Since.UTC().Format(time.RFC3339))
	}

	body, err := c.api(ctx, args)
	if err != nil {
		return nil, err
	}

	var raw []apiNotification
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode notifications: %w", err)
	}

	out := make([]digest.Notification, 0, len(raw))
	for _, n := range raw {
		out = append(out, n.toDigest())
	}
	return out, nil
}

func (c *Client) searchPRs(ctx context.Context, query string) ([]digest.PullRequest, error) {
	args := []string{
		"search/issues",
		"-f", "q=" + query,
		"-F", "per_page=100",
	}
	body, err := c.api(ctx, args)
	if err != nil {
		return nil, err
	}

	var raw apiSearch
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode search %q: %w", query, err)
	}

	out := make([]digest.PullRequest, 0, len(raw.Items))
	for _, item := range raw.Items {
		if pr := item.toPR(); pr != nil {
			out = append(out, *pr)
		}
	}
	return out, nil
}

func (c *Client) api(ctx context.Context, args []string) ([]byte, error) {
	full := append([]string{"api"}, args...)
	out, err := c.runner().CombinedOutput(ctx, "gh", full...)
	if err != nil {
		return nil, wrapGHError(err, out)
	}
	return bytes.TrimSpace(out), nil
}

func wrapGHError(err error, out []byte) error {
	msg := githubAPIMessage(out)
	if msg == "" {
		msg = strings.TrimSpace(string(out))
	}
	var execErr *exec.Error
	if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
		return fmt.Errorf("gh-digest requires the GitHub CLI (gh). Install it from https://cli.github.com and run: gh auth login")
	}

	lower := strings.ToLower(msg + " " + err.Error() + " " + string(out))
	if strings.Contains(lower, "could not locate gh") || strings.Contains(lower, "executable file not found") {
		return fmt.Errorf("gh-digest requires the GitHub CLI (gh). Install it from https://cli.github.com and run: gh auth login")
	}
	if strings.Contains(lower, "resource not accessible by integration") {
		return fmt.Errorf("this GitHub token cannot read the current user (GitHub App or GITHUB_TOKEN). Authenticate as a user: gh auth login")
	}
	if strings.Contains(lower, "401") || strings.Contains(lower, "authentication") || strings.Contains(lower, "gh auth login") {
		return fmt.Errorf("GitHub CLI is not authenticated. Run: gh auth login")
	}
	if msg != "" {
		return fmt.Errorf("gh api: %s", compact(msg))
	}
	return fmt.Errorf("gh api: %w", err)
}

func githubAPIMessage(out []byte) string {
	i := bytes.IndexByte(out, '{')
	if i < 0 {
		return ""
	}
	var payload struct {
		Message string `json:"message"`
		Status  string `json:"status"`
	}
	if err := json.NewDecoder(bytes.NewReader(out[i:])).Decode(&payload); err != nil || payload.Message == "" {
		return ""
	}
	if payload.Status != "" {
		return payload.Status + " " + payload.Message
	}
	return payload.Message
}

func compact(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 400 {
		return s[:397] + "..."
	}
	return s
}

type apiNotification struct {
	ID        string    `json:"id"`
	Unread    bool      `json:"unread"`
	Reason    string    `json:"reason"`
	UpdatedAt time.Time `json:"updated_at"`
	Subject   struct {
		Title string `json:"title"`
		URL   string `json:"url"`
		Type  string `json:"type"`
	} `json:"subject"`
	Repository struct {
		FullName string `json:"full_name"`
		HTMLURL  string `json:"html_url"`
	} `json:"repository"`
}

func (n apiNotification) toDigest() digest.Notification {
	number := parseTrailingNumber(n.Subject.URL)
	htmlURL := webURL(n.Repository.HTMLURL, n.Subject.Type, number)
	return digest.Notification{
		ID:        n.ID,
		Unread:    n.Unread,
		Reason:    n.Reason,
		Title:     n.Subject.Title,
		Type:      n.Subject.Type,
		Repo:      n.Repository.FullName,
		Number:    number,
		HTMLURL:   htmlURL,
		UpdatedAt: n.UpdatedAt,
	}
}

type apiSearch struct {
	Items []apiIssue `json:"items"`
}

type apiIssue struct {
	Title         string    `json:"title"`
	Number        int       `json:"number"`
	HTMLURL       string    `json:"html_url"`
	UpdatedAt     time.Time `json:"updated_at"`
	Comments      int       `json:"comments"`
	RepositoryURL string    `json:"repository_url"`
	PullRequest   *struct{} `json:"pull_request"`
	User          struct {
		Login string `json:"login"`
	} `json:"user"`
}

func (i apiIssue) toPR() *digest.PullRequest {
	if i.PullRequest == nil && !strings.Contains(i.HTMLURL, "/pull/") {
		return nil
	}
	repo := repoFromIssue(i.HTMLURL, i.RepositoryURL)
	return &digest.PullRequest{
		Repo:      repo,
		Number:    i.Number,
		Title:     i.Title,
		HTMLURL:   i.HTMLURL,
		Author:    i.User.Login,
		Comments:  i.Comments,
		UpdatedAt: i.UpdatedAt,
	}
}

func repoFromIssue(htmlURL, repositoryURL string) string {
	if repo := repoFromGitHubURL(htmlURL); repo != "" {
		return repo
	}
	const prefix = "https://api.github.com/repos/"
	if strings.HasPrefix(repositoryURL, prefix) {
		return strings.Trim(strings.TrimPrefix(repositoryURL, prefix), "/")
	}
	return ""
}

func repoFromGitHubURL(htmlURL string) string {
	const prefix = "https://github.com/"
	if !strings.HasPrefix(htmlURL, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(htmlURL, prefix)
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[0] + "/" + parts[1]
}

func parseTrailingNumber(url string) int {
	if url == "" {
		return 0
	}
	url = strings.TrimSuffix(url, "/")
	i := strings.LastIndex(url, "/")
	if i < 0 || i+1 >= len(url) {
		return 0
	}
	n, err := strconv.Atoi(url[i+1:])
	if err != nil {
		return 0
	}
	return n
}

func webURL(repoHTML, subjectType string, number int) string {
	if repoHTML == "" || number == 0 {
		return ""
	}
	repoHTML = strings.TrimSuffix(repoHTML, "/")
	switch subjectType {
	case "PullRequest":
		return repoHTML + "/pull/" + strconv.Itoa(number)
	case "Issue":
		return repoHTML + "/issues/" + strconv.Itoa(number)
	case "Discussion":
		return repoHTML + "/discussions/" + strconv.Itoa(number)
	case "Release":
		return repoHTML + "/releases"
	default:
		if number > 0 {
			return repoHTML + "/issues/" + strconv.Itoa(number)
		}
		return repoHTML
	}
}
