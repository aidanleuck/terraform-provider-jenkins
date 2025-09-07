# Bitbucket Server Auto-Approve Bot

A simple auto-approval bot for Bitbucket Server, inspired by the renovate-approve-bot pattern.

## Features

- **Auto-approve pull requests** based on configurable rules
- **User allowlist** - only approve PRs from specific users (e.g., renovate, dependabot)
- **Title pattern matching** - only approve PRs with titles matching a regex pattern
- **Dry-run mode** - see what would be approved without actually approving
- **Multi-repository support** - can check all repos in a project or a specific repo
- **Polling mode** - continuously monitor for new PRs

## Usage

### Basic Usage

```bash
# Build the bot
go build -o bitbucket-approve-bot ./cmd/bitbucket-approve-bot

# Approve PRs from renovate and dependabot in a specific repo
./bitbucket-approve-bot \
  -server https://bitbucket.company.com \
  -project MYPROJ \
  -repo myrepo \
  -allowed-users renovate,dependabot

# Check all repos in a project with title pattern matching
./bitbucket-approve-bot \
  -server https://bitbucket.company.com \
  -project MYPROJ \
  -allowed-users renovate \
  -title-pattern "^(chore|feat|fix)\(deps\):"

# Dry run to see what would be approved
./bitbucket-approve-bot \
  -server https://bitbucket.company.com \
  -project MYPROJ \
  -allowed-users renovate,dependabot \
  -dry-run
```

### Command Line Options

- `-server` - Bitbucket Server URL (required)
- `-project` - Bitbucket project key (required)
- `-repo` - Repository name (optional, defaults to all repos in project)
- `-username` - Bitbucket username (or use `BITBUCKET_USERNAME` env var)
- `-password` - Bitbucket password/token (or use `BITBUCKET_PASSWORD` env var)
- `-allowed-users` - Comma-separated list of users whose PRs can be auto-approved
- `-title-pattern` - Regex pattern that PR titles must match
- `-interval` - Polling interval (default: 30s)
- `-dry-run` - Show what would be approved without actually approving
- `-help` - Show help

### Environment Variables

- `BITBUCKET_USERNAME` - Your Bitbucket username
- `BITBUCKET_PASSWORD` - Your Bitbucket password or personal access token

### Authentication

The bot supports Basic Authentication using username/password or username/personal access token. Personal access tokens are recommended for better security.

To create a personal access token in Bitbucket Server:
1. Go to your profile settings
2. Navigate to "Personal access tokens"
3. Create a new token with "Repository read" and "Pull request write" permissions

## Examples

### Renovate Bot Integration

```bash
# Approve all Renovate PRs with dependency update titles
./bitbucket-approve-bot \
  -server https://bitbucket.company.com \
  -project MYPROJ \
  -allowed-users renovate \
  -title-pattern "^(chore|feat|fix)\(deps\):" \
  -interval 60s
```

### Multiple Bot Users

```bash
# Approve PRs from multiple automation users
./bitbucket-approve-bot \
  -server https://bitbucket.company.com \
  -project MYPROJ \
  -allowed-users "renovate,dependabot,greenkeeper" \
  -interval 30s
```

### Docker Usage

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o bitbucket-approve-bot ./cmd/bitbucket-approve-bot

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/bitbucket-approve-bot .
CMD ["./bitbucket-approve-bot"]
```

```bash
# Run with Docker
docker run -e BITBUCKET_USERNAME=myuser -e BITBUCKET_PASSWORD=mytoken \
  myimage ./bitbucket-approve-bot -server https://bitbucket.company.com -project MYPROJ -allowed-users renovate
```

## Configuration Rules

The bot will approve a PR if **all** of the following conditions are met:

1. **Author Check**: If `-allowed-users` is specified, the PR author must be in the list
2. **Title Check**: If `-title-pattern` is specified, the PR title must match the regex pattern
3. **Not Already Approved**: The bot user hasn't already approved the PR

If no rules are specified (no `-allowed-users` or `-title-pattern`), the bot will approve **all** open PRs, which is generally not recommended.

## Logging

The bot provides detailed logging of its actions:

```
2024/01/15 10:30:00 Starting Bitbucket Auto-Approve Bot v1.0.0
2024/01/15 10:30:00 Server: https://bitbucket.company.com
2024/01/15 10:30:00 Project: MYPROJ
2024/01/15 10:30:00 Repository: myrepo
2024/01/15 10:30:00 Polling interval: 30s
2024/01/15 10:30:00 Dry run: false
2024/01/15 10:30:01 Checking 1 repositories for pull requests
2024/01/15 10:30:01 PR #123 by renovate qualifies for auto-approval: chore(deps): update dependency foo to v1.2.3
2024/01/15 10:30:01 Approved PR #123 in MYPROJ/myrepo: chore(deps): update dependency foo to v1.2.3 (by renovate)
2024/01/15 10:30:01 Processed 5 pull requests, approved 1
```

## License

MPL-2.0