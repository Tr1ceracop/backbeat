package tempo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"backbeat/internal/config"
)

type Client struct {
	baseURL    string
	token      string
	accountID  string
	httpClient *http.Client
}

func NewClient(cfg config.TempoConfig) *Client {
	return &Client{
		baseURL:    cfg.BaseURL,
		token:      cfg.APIToken,
		accountID:  cfg.AccountID,
		httpClient: &http.Client{},
	}
}

func (c *Client) CreateWorklog(req CreateWorklogRequest) (*WorklogResponse, error) {
	if req.AuthorAccountID == "" {
		req.AuthorAccountID = c.accountID
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.baseURL+"/4/worklogs", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("tempo API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tempo API %d: %s", resp.StatusCode, string(respBody))
	}

	var result WorklogResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

func (c *Client) GetWorklogs(from, to string) ([]WorklogResponse, error) {
	url := fmt.Sprintf("%s/4/worklogs?from=%s&to=%s", c.baseURL, from, to)

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("tempo API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tempo API %d: %s", resp.StatusCode, string(respBody))
	}

	var result WorklogSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result.Results, nil
}
