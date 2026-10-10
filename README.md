# gh-digest

`gh-digest` prints your GitHub notifications and open pull requests. It calls the [`gh`](https://cli.github.com) CLI, so it uses the same login as `gh pr` and `gh api`.

## Prerequisites

- [Go 1.22+](https://go.dev/dl/) to build from source
- [GitHub CLI](https://cli.github.com) (`gh`), installed and authenticated:

```bash
gh auth login
gh auth status
```

`gh-digest` calls `gh api` for:

- `GET /user`
- `GET /notifications`
- `GET /search/issues` for open PRs you authored, were asked to review, or are assigned

If a notification fetch returns 401 or 403, refresh the `notifications` scope:

```bash
gh auth refresh -s notifications
```

You need a user login. GitHub App installation tokens and `GITHUB_TOKEN` in Actions usually cannot call `/user` or list your notifications.

## Install

```bash
git clone https://github.com/78tacos/gh-digest.git
cd gh-digest
go install ./cmd/gh-digest
```

`go install` puts the binary in `$(go env GOPATH)/bin`. Add that directory to `PATH` if it is not there already.

To build a binary in this directory:

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

Unread notifications are marked with `*`. An open pull request is listed once: authored, then review requested, then assigned.

## Tests

```bash
go test ./...
go vet ./...
```

Grouping, unread filtering, limits, and pull-request classification are covered in `internal/digest` without calling GitHub.

## License

MIT. See [LICENSE](LICENSE).
