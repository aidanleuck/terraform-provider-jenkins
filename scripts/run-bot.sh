#!/bin/bash

# Example script to run the Bitbucket Auto-Approve Bot

# Configuration - adjust these values for your environment
BITBUCKET_SERVER="https://bitbucket.company.com"
PROJECT_KEY="MYPROJ"
REPO_NAME="myrepo"  # Leave empty to check all repos in the project
ALLOWED_USERS="renovate,dependabot"
TITLE_PATTERN="^(chore|feat|fix)\(deps\):"
POLLING_INTERVAL="60s"

# Build the bot (if not already built)
if [ ! -f "./bitbucket-approve-bot" ]; then
    echo "Building Bitbucket Auto-Approve Bot..."
    go build -o bitbucket-approve-bot ./cmd/bitbucket-approve-bot
    if [ $? -ne 0 ]; then
        echo "Build failed!"
        exit 1
    fi
fi

# Check for required environment variables
if [ -z "$BITBUCKET_USERNAME" ]; then
    echo "Error: BITBUCKET_USERNAME environment variable is required"
    echo "Export your Bitbucket username: export BITBUCKET_USERNAME=your-username"
    exit 1
fi

if [ -z "$BITBUCKET_PASSWORD" ]; then
    echo "Error: BITBUCKET_PASSWORD environment variable is required"
    echo "Export your Bitbucket password or token: export BITBUCKET_PASSWORD=your-token"
    exit 1
fi

# Build command arguments
ARGS="-server $BITBUCKET_SERVER -project $PROJECT_KEY"

if [ -n "$REPO_NAME" ]; then
    ARGS="$ARGS -repo $REPO_NAME"
fi

if [ -n "$ALLOWED_USERS" ]; then
    ARGS="$ARGS -allowed-users $ALLOWED_USERS"
fi

if [ -n "$TITLE_PATTERN" ]; then
    ARGS="$ARGS -title-pattern $TITLE_PATTERN"
fi

if [ -n "$POLLING_INTERVAL" ]; then
    ARGS="$ARGS -interval $POLLING_INTERVAL"
fi

# Add dry-run flag if requested
if [ "$1" = "--dry-run" ]; then
    ARGS="$ARGS -dry-run"
    echo "Running in DRY RUN mode - no PRs will actually be approved"
fi

echo "Starting Bitbucket Auto-Approve Bot..."
echo "Server: $BITBUCKET_SERVER"
echo "Project: $PROJECT_KEY"
echo "Repository: ${REPO_NAME:-"ALL"}"
echo "Allowed Users: $ALLOWED_USERS"
echo "Title Pattern: $TITLE_PATTERN"
echo "Polling Interval: $POLLING_INTERVAL"
echo ""

# Run the bot
exec ./bitbucket-approve-bot $ARGS