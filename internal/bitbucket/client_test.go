// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package bitbucket

import (
	"net/url"
	"testing"
)

func TestNewClient(t *testing.T) {
	config := &Config{
		ServerURL: "https://bitbucket.example.com",
		Username:  "testuser",
		Password:  "testpass",
		Project:   "TEST",
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if client.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", client.Username)
	}

	if client.Password != "testpass" {
		t.Errorf("Expected password 'testpass', got '%s'", client.Password)
	}

	expectedURL, _ := url.Parse("https://bitbucket.example.com/rest/api/1.0")
	if client.BaseURL.String() != expectedURL.String() {
		t.Errorf("Expected URL '%s', got '%s'", expectedURL.String(), client.BaseURL.String())
	}
}

func TestNewClientInvalidURL(t *testing.T) {
	config := &Config{
		ServerURL: "://invalid-url",
		Username:  "testuser",
		Password:  "testpass",
		Project:   "TEST",
	}

	_, err := NewClient(config)
	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}
}

func TestNewApproveBot(t *testing.T) {
	config := &Config{
		ServerURL:    "https://bitbucket.example.com",
		Username:     "testuser", 
		Password:     "testpass",
		Project:      "TEST",
		AllowedUsers: "renovate,dependabot",
		TitlePattern: "^chore\\(deps\\):",
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	bot := NewApproveBot(client, config)
	if bot == nil {
		t.Error("Expected bot to be created, got nil")
	}

	if bot.client != client {
		t.Error("Bot client doesn't match expected client")
	}

	if bot.config != config {
		t.Error("Bot config doesn't match expected config")
	}
}

func TestShouldApprovePR(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		pr           PullRequest
		shouldApprove bool
	}{
		{
			name: "allowed user with no pattern",
			config: &Config{
				AllowedUsers: "renovate,dependabot",
			},
			pr: PullRequest{
				ID:    1,
				Title: "Update dependency",
				Author: struct {
					User struct {
						Name         string `json:"name"`
						EmailAddress string `json:"emailAddress"`
						DisplayName  string `json:"displayName"`
						Slug         string `json:"slug"`
					} `json:"user"`
				}{
					User: struct {
						Name         string `json:"name"`
						EmailAddress string `json:"emailAddress"`
						DisplayName  string `json:"displayName"`
						Slug         string `json:"slug"`
					}{
						Name: "renovate",
					},
				},
			},
			shouldApprove: true,
		},
		{
			name: "disallowed user",
			config: &Config{
				AllowedUsers: "renovate,dependabot",
			},
			pr: PullRequest{
				ID:    2,
				Title: "Update dependency",
				Author: struct {
					User struct {
						Name         string `json:"name"`
						EmailAddress string `json:"emailAddress"`
						DisplayName  string `json:"displayName"`
						Slug         string `json:"slug"`
					} `json:"user"`
				}{
					User: struct {
						Name         string `json:"name"`
						EmailAddress string `json:"emailAddress"`
						DisplayName  string `json:"displayName"`
						Slug         string `json:"slug"`
					}{
						Name: "someuser",
					},
				},
			},
			shouldApprove: false,
		},
		{
			name: "allowed user with matching pattern",
			config: &Config{
				AllowedUsers: "renovate",
				TitlePattern: "^chore\\(deps\\):",
			},
			pr: PullRequest{
				ID:    3,
				Title: "chore(deps): update dependency foo",
				Author: struct {
					User struct {
						Name         string `json:"name"`
						EmailAddress string `json:"emailAddress"`
						DisplayName  string `json:"displayName"`
						Slug         string `json:"slug"`
					} `json:"user"`
				}{
					User: struct {
						Name         string `json:"name"`
						EmailAddress string `json:"emailAddress"`
						DisplayName  string `json:"displayName"`
						Slug         string `json:"slug"`
					}{
						Name: "renovate",
					},
				},
			},
			shouldApprove: true,
		},
		{
			name: "allowed user with non-matching pattern",
			config: &Config{
				AllowedUsers: "renovate",
				TitlePattern: "^chore\\(deps\\):",
			},
			pr: PullRequest{
				ID:    4,
				Title: "feat: add new feature",
				Author: struct {
					User struct {
						Name         string `json:"name"`
						EmailAddress string `json:"emailAddress"`
						DisplayName  string `json:"displayName"`
						Slug         string `json:"slug"`
					} `json:"user"`
				}{
					User: struct {
						Name         string `json:"name"`
						EmailAddress string `json:"emailAddress"`
						DisplayName  string `json:"displayName"`
						Slug         string `json:"slug"`
					}{
						Name: "renovate",
					},
				},
			},
			shouldApprove: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := NewClient(&Config{
				ServerURL: "https://test.com",
				Username:  "test",
				Password:  "test",
			})
			bot := NewApproveBot(client, tt.config)

			result := bot.shouldApprovePR(tt.pr)
			if result != tt.shouldApprove {
				t.Errorf("Expected shouldApprovePR to return %v, got %v", tt.shouldApprove, result)
			}
		})
	}
}