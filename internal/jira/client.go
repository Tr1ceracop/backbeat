package jira

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"backbeat/internal/config"
)

type Client struct {
	baseURL    string
	authHeader string
	httpClient *http.Client
}

type issueResponse struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

type myselfResponse struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
}

func NewClient(cfg config.JiraConfig) *Client {
	auth := base64.StdEncoding.EncodeToString([]byte(cfg.Email + ":" + cfg.APIToken))
	return &Client{
		baseURL:    cfg.BaseURL,
		authHeader: "Basic " + auth,
		httpClient: &http.Client{},
	}
}

// GetMyself returns the account ID and display name of the authenticated user.
func (c *Client) GetMyself() (accountID, displayName string, err error) {
	req, err := http.NewRequest("GET", c.baseURL+"/rest/api/3/myself", nil)
	if err != nil {
		return "", "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("jira API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("jira API %d: %s", resp.StatusCode, string(body))
	}

	var me myselfResponse
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
		return "", "", fmt.Errorf("decode response: %w", err)
	}

	return me.AccountID, me.DisplayName, nil
}

// GetIssueID resolves a Jira issue key (e.g. "PROJ-123") to its numeric ID.
func (c *Client) GetIssueID(issueKey string) (int, error) {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s?fields=id", c.baseURL, issueKey)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("jira API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("jira API %d: %s", resp.StatusCode, string(body))
	}

	var issue issueResponse
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}

	id, err := strconv.Atoi(issue.ID)
	if err != nil {
		return 0, fmt.Errorf("parse issue ID %q: %w", issue.ID, err)
	}

	return id, nil
}
