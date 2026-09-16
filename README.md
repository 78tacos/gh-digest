# gh-digest

A small command-line tool that digests **GitHub notifications** and **open pull requests** for the authenticated user.

v1 is a terminal summary, not a dashboard. It reads GitHub through the [`gh`](https://cli.github.com) CLI so you reuse the same login you already use for `gh pr` and `gh api`.

## Prerequisites

1. [Go 1.22+](https://go.dev/dl/) to build from source.
2. [GitHub CLI](https://cli.github.com) (`gh`) installed and authenticated:

```bash
gh auth login
gh auth status
```

`gh-digest` calls `gh api` for:

- `GET /user`
- `GET /notifications`
- `GET /search/issues` for open PRs you authored, were asked to review, or are assigned

If notification fetches fail with 403/401, refresh the `notifications` scope:

```bash
gh auth refresh -s notifications
```

`gh-digest` needs a **user** login. GitHub App installation tokens and `GITHUB_TOKEN` in Actions typically cannot call `/user` or list your notifications.

## Install

From this repository:

```bash
git clone https://github.com/78tacos/gh-digest.git
cd gh-digest
go install ./cmd/gh-digest
```

`go install` puts the binary in `$(go env GOPATH)/bin` (add that directory to `PATH`).

To build a local binary instead:

```bash
go build -o gh-digest ./cmd/gh-digest
./gh-digest --help
```

## Usage

```bash
gh-digest
gh-digest --json
gh-digest --all --limit 20
gh-digest --since 24h
gh-digest --prs-only
gh-digest --notifs-only --participating
```

| Flag | Meaning |
| --- | --- |
| `--json` | Machine-readable JSON instead of text |
| `--all` | Include already-read notifications |
| `--participating` | Only threads you are participating in |
| `--limit int` | Max notifications to include (default 50) |
| `--since duration` | Only notifications updated in this window (`24h`, `7d`, or RFC3339) |
| `--prs-only` | Skip the notifications section |
| `--notifs-only` | Skip the open-PR section |
| `--version` | Print version |

### Sample text output

```
GitHub digest for @octocat  ·  16 Sep 2026 16:00 UTC

Notifications  3 unread · 3 shown
----------------------------------------
octo/hello
  * review requested   PR #9        Please review auth                 30m
acme/api
  * mention            PR #88       Add digest CLI                     2h
  * comment            Issue #41    Rate limit retries                 5h

Open pull requests
----------------------------------------
Authored (1)
  acme/api#88  Add digest CLI  2h  3 comments

Review requested (1)
  octo/hello#9  Please review auth  30m  1 comment

Assigned (1)
  acme/web#15  Fix nav  1d  no comments
```

Unread notifications are marked with `*`. Open PRs are split into exclusive buckets: **authored** wins, then **review requested**, then **assigned**.

## Tests

```bash
go test ./...
go vet ./...
```

Core digest rules (grouping, unread filtering, limits, exclusive PR classification) live in `internal/digest` and are covered without calling GitHub.

## License

MIT. See [LICENSE](LICENSE).
