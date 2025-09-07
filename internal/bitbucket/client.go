// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package bitbucket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Config holds the configuration for the Bitbucket client
type Config struct {
	ServerURL    string
	Username     string
	Password     string
	Project      string
	Repository   string
	AllowedUsers string
	TitlePattern string
	DryRun       bool
}

// Client represents a Bitbucket Server API client
type Client struct {
	BaseURL    *url.URL
	Username   string
	Password   string
	HTTPClient *http.Client
}

// PullRequest represents a Bitbucket Server pull request
type PullRequest struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       string `json:"state"`
	Open        bool   `json:"open"`
	Closed      bool   `json:"closed"`
	Author      struct {
		User struct {
			Name         string `json:"name"`
			EmailAddress string `json:"emailAddress"`
			DisplayName  string `json:"displayName"`
			Slug         string `json:"slug"`
		} `json:"user"`
	} `json:"author"`
	ToRef struct {
		Repository struct {
			Slug    string `json:"slug"`
			Name    string `json:"name"`
			Project struct {
				Key  string `json:"key"`
				Name string `json:"name"`
			} `json:"project"`
		} `json:"repository"`
		DisplayID string `json:"displayId"`
	} `json:"toRef"`
	FromRef struct {
		DisplayID string `json:"displayId"`
	} `json:"fromRef"`
	Links struct {
		Self []struct {
			Href string `json:"href"`
		} `json:"self"`
	} `json:"links"`
}

// PullRequestList represents a paginated list of pull requests
type PullRequestList struct {
	Size       int           `json:"size"`
	Limit      int           `json:"limit"`
	IsLastPage bool          `json:"isLastPage"`
	Start      int           `json:"start"`
	Values     []PullRequest `json:"values"`
}

// Repository represents a Bitbucket repository
type Repository struct {
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	Project struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"project"`
}

// RepositoryList represents a paginated list of repositories
type RepositoryList struct {
	Size       int          `json:"size"`
	Limit      int          `json:"limit"`
	IsLastPage bool         `json:"isLastPage"`
	Start      int          `json:"start"`
	Values     []Repository `json:"values"`
}

// NewClient creates a new Bitbucket Server client
func NewClient(config *Config) (*Client, error) {
	baseURL, err := url.Parse(config.ServerURL)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL: %w", err)
	}

	// Ensure the URL has the API path
	if baseURL.Path == "" || baseURL.Path == "/" {
		baseURL.Path = "/rest/api/1.0"
	}

	return &Client{
		BaseURL:  baseURL,
		Username: config.Username,
		Password: config.Password,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// makeRequest makes an HTTP request to the Bitbucket API
func (c *Client) makeRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	reqURL := c.BaseURL.ResolveReference(&url.URL{Path: path})
	req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.Username, c.Password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// GetRepositories gets all repositories for a project
func (c *Client) GetRepositories(ctx context.Context, projectKey string) ([]Repository, error) {
	var allRepos []Repository
	start := 0
	limit := 25

	for {
		path := fmt.Sprintf("/projects/%s/repos?start=%d&limit=%d", projectKey, start, limit)
		resp, err := c.makeRequest(ctx, "GET", path, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get repositories: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		var repoList RepositoryList
		if err := json.NewDecoder(resp.Body).Decode(&repoList); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		allRepos = append(allRepos, repoList.Values...)

		if repoList.IsLastPage {
			break
		}
		start += limit
	}

	return allRepos, nil
}

// GetPullRequests gets all open pull requests for a repository
func (c *Client) GetPullRequests(ctx context.Context, projectKey, repoSlug string) ([]PullRequest, error) {
	var allPRs []PullRequest
	start := 0
	limit := 25

	for {
		path := fmt.Sprintf("/projects/%s/repos/%s/pull-requests?state=OPEN&start=%d&limit=%d", projectKey, repoSlug, start, limit)
		resp, err := c.makeRequest(ctx, "GET", path, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get pull requests: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		var prList PullRequestList
		if err := json.NewDecoder(resp.Body).Decode(&prList); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		allPRs = append(allPRs, prList.Values...)

		if prList.IsLastPage {
			break
		}
		start += limit
	}

	return allPRs, nil
}

// ApprovePullRequest approves a pull request
func (c *Client) ApprovePullRequest(ctx context.Context, projectKey, repoSlug string, prID int) error {
	path := fmt.Sprintf("/projects/%s/repos/%s/pull-requests/%d/approve", projectKey, repoSlug, prID)
	resp, err := c.makeRequest(ctx, "POST", path, nil)
	if err != nil {
		return fmt.Errorf("failed to approve pull request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("approve request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// IsApproved checks if a pull request is already approved by the current user
func (c *Client) IsApproved(ctx context.Context, projectKey, repoSlug string, prID int) (bool, error) {
	path := fmt.Sprintf("/projects/%s/repos/%s/pull-requests/%d/participants", projectKey, repoSlug, prID)
	resp, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		return false, fmt.Errorf("failed to get PR participants: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("participants request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var participants struct {
		Values []struct {
			User struct {
				Name string `json:"name"`
			} `json:"user"`
			Approved bool   `json:"approved"`
			Status   string `json:"status"`
		} `json:"values"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&participants); err != nil {
		return false, fmt.Errorf("failed to decode participants response: %w", err)
	}

	for _, participant := range participants.Values {
		if participant.User.Name == c.Username && participant.Approved {
			return true, nil
		}
	}

	return false, nil
}