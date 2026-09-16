package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/78tacos/gh-digest/internal/digest"
	"github.com/78tacos/gh-digest/internal/format"
	"github.com/78tacos/gh-digest/internal/github"
)

const Version = "0.1.0"

const usage = `gh-digest summarizes your GitHub notifications and open pull requests.

Usage:
  gh-digest [flags]

Flags:
  --json              Output JSON instead of text
  --all               Include already-read notifications
  --participating     Only notifications you are participating in
  --limit int         Max notifications to include (default 50)
  --since duration    Only notifications updated within this window (e.g. 24h, 7d)
  --prs-only          Show only open pull requests
  --notifs-only       Show only notifications
  --version           Print version and exit
  -h, --help          Show this help

Auth:
  Requires the GitHub CLI. Run: gh auth login
`

// App is the gh-digest command. Source is injected so tests can skip the network.
type App struct {
	Source github.Source
	Now    time.Time
}

type config struct {
	json          bool
	all           bool
	participating bool
	limit         int
	since         string
	prsOnly       bool
	notifsOnly    bool
	version       bool
	help          bool
}

func (a *App) Run(args []string, stdout, stderr io.Writer) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		fmt.Fprint(stderr, usage)
		return 2
	}
	if cfg.help {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if cfg.version {
		fmt.Fprintln(stdout, Version)
		return 0
	}

	now := a.now()
	since, err := ParseSince(cfg.since, now)
	if err != nil {
		fmt.Fprintf(stderr, "invalid --since: %v\n", err)
		return 2
	}

	src := a.Source
	if src == nil {
		src = github.NewClient()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	snap, err := src.Load(ctx, github.LoadOptions{
		All:           cfg.all,
		Participating: cfg.participating,
		Since:         since,
	})
	if err != nil {
		fmt.Fprintf(stderr, "gh-digest: %v\n", err)
		return 1
	}

	includeNotifs := !cfg.prsOnly
	includePRs := !cfg.notifsOnly
	d := digest.Builder{Opts: digest.Options{
		Now:                  now,
		NotificationLimit:    cfg.limit,
		IncludeRead:          cfg.all,
		Since:                since,
		IncludeNotifications: includeNotifs,
		IncludePRs:           includePRs,
	}}.Build(snap)

	var fmtErr error
	if cfg.json {
		fmtErr = (format.Formatter{}).JSON(stdout, d)
	} else {
		fmtErr = (format.Formatter{}).Text(stdout, d)
	}
	if fmtErr != nil {
		fmt.Fprintf(stderr, "gh-digest: %v\n", fmtErr)
		return 1
	}
	return 0
}

func (a *App) now() time.Time {
	if a != nil && !a.Now.IsZero() {
		return a.Now.UTC()
	}
	return time.Now().UTC()
}

func parseArgs(args []string) (config, error) {
	cfg := config{limit: 50}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			cfg.help = true
		case arg == "--version":
			cfg.version = true
		case arg == "--json":
			cfg.json = true
		case arg == "--all":
			cfg.all = true
		case arg == "--participating":
			cfg.participating = true
		case arg == "--prs-only":
			cfg.prsOnly = true
		case arg == "--notifs-only":
			cfg.notifsOnly = true
		case arg == "--limit":
			if i+1 >= len(args) {
				return config{}, errors.New("--limit requires a value")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 0 {
				return config{}, fmt.Errorf("invalid --limit %q", args[i])
			}
			cfg.limit = n
		case strings.HasPrefix(arg, "--limit="):
			n, err := strconv.Atoi(strings.TrimPrefix(arg, "--limit="))
			if err != nil || n < 0 {
				return config{}, fmt.Errorf("invalid --limit %q", arg)
			}
			cfg.limit = n
		case arg == "--since":
			if i+1 >= len(args) {
				return config{}, errors.New("--since requires a value")
			}
			i++
			cfg.since = args[i]
		case strings.HasPrefix(arg, "--since="):
			cfg.since = strings.TrimPrefix(arg, "--since=")
		default:
			return config{}, fmt.Errorf("unknown flag: %s", arg)
		}
	}
	if cfg.prsOnly && cfg.notifsOnly {
		return config{}, errors.New("--prs-only and --notifs-only cannot be used together")
	}
	return cfg, nil
}

// ParseSince accepts RFC3339 timestamps or durations like 24h and 7d.
func ParseSince(value string, now time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	if strings.HasSuffix(value, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err != nil || days < 0 {
			return time.Time{}, fmt.Errorf("invalid day duration %q", value)
		}
		return now.Add(-time.Duration(days) * 24 * time.Hour), nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return time.Time{}, err
	}
	if d < 0 {
		return time.Time{}, fmt.Errorf("duration must be positive")
	}
	return now.Add(-d), nil
}
