// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package bitbucket

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
)

// ApproveBot handles the logic for automatically approving pull requests
type ApproveBot struct {
	client *Client
	config *Config
}

// NewApproveBot creates a new approval bot instance
func NewApproveBot(client *Client, config *Config) *ApproveBot {
	return &ApproveBot{
		client: client,
		config: config,
	}
}

// ProcessPullRequests checks all pull requests and approves qualifying ones
func (bot *ApproveBot) ProcessPullRequests(ctx context.Context) error {
	var repos []Repository
	var err error

	// Get repositories to check
	if bot.config.Repository != "" {
		// Single repository mode
		repos = []Repository{{
			Slug: bot.config.Repository,
			Project: struct {
				Key  string `json:"key"`
				Name string `json:"name"`
			}{
				Key: bot.config.Project,
			},
		}}
	} else {
		// All repositories in project mode
		repos, err = bot.client.GetRepositories(ctx, bot.config.Project)
		if err != nil {
			return fmt.Errorf("failed to get repositories: %w", err)
		}
	}

	log.Printf("Checking %d repositories for pull requests", len(repos))

	var totalPRs, approvedPRs int

	for _, repo := range repos {
		prs, err := bot.client.GetPullRequests(ctx, bot.config.Project, repo.Slug)
		if err != nil {
			log.Printf("Failed to get pull requests for %s/%s: %v", bot.config.Project, repo.Slug, err)
			continue
		}

		totalPRs += len(prs)

		for _, pr := range prs {
			if bot.shouldApprovePR(pr) {
				// Check if already approved by us
				alreadyApproved, err := bot.client.IsApproved(ctx, bot.config.Project, repo.Slug, pr.ID)
				if err != nil {
					log.Printf("Failed to check approval status for PR #%d in %s/%s: %v", pr.ID, bot.config.Project, repo.Slug, err)
					continue
				}

				if alreadyApproved {
					log.Printf("PR #%d in %s/%s is already approved by us", pr.ID, bot.config.Project, repo.Slug)
					continue
				}

				if bot.config.DryRun {
					log.Printf("DRY RUN: Would approve PR #%d in %s/%s: %s (by %s)", 
						pr.ID, bot.config.Project, repo.Slug, pr.Title, pr.Author.User.Name)
					approvedPRs++
				} else {
					err := bot.client.ApprovePullRequest(ctx, bot.config.Project, repo.Slug, pr.ID)
					if err != nil {
						log.Printf("Failed to approve PR #%d in %s/%s: %v", pr.ID, bot.config.Project, repo.Slug, err)
						continue
					}
					log.Printf("Approved PR #%d in %s/%s: %s (by %s)", 
						pr.ID, bot.config.Project, repo.Slug, pr.Title, pr.Author.User.Name)
					approvedPRs++
				}
			}
		}
	}

	if totalPRs > 0 {
		log.Printf("Processed %d pull requests, approved %d", totalPRs, approvedPRs)
	}

	return nil
}

// shouldApprovePR determines if a pull request should be auto-approved based on the configured rules
func (bot *ApproveBot) shouldApprovePR(pr PullRequest) bool {
	// Check if author is in allowed users list
	if bot.config.AllowedUsers != "" {
		allowedUsers := strings.Split(bot.config.AllowedUsers, ",")
		authorAllowed := false
		for _, user := range allowedUsers {
			user = strings.TrimSpace(user)
			if user == pr.Author.User.Name || user == pr.Author.User.Slug || user == pr.Author.User.DisplayName {
				authorAllowed = true
				break
			}
		}
		if !authorAllowed {
			log.Printf("PR #%d by %s not approved: author not in allowed users list", pr.ID, pr.Author.User.Name)
			return false
		}
	}

	// Check if title matches required pattern
	if bot.config.TitlePattern != "" {
		matched, err := regexp.MatchString(bot.config.TitlePattern, pr.Title)
		if err != nil {
			log.Printf("PR #%d: invalid title pattern regex: %v", pr.ID, err)
			return false
		}
		if !matched {
			log.Printf("PR #%d not approved: title '%s' does not match pattern '%s'", pr.ID, pr.Title, bot.config.TitlePattern)
			return false
		}
	}

	log.Printf("PR #%d by %s qualifies for auto-approval: %s", pr.ID, pr.Author.User.Name, pr.Title)
	return true
}