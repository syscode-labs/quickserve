package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const DefaultBaseURL = "https://api.cloudflare.com/client/v4"

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func (c Client) TunnelToken(ctx context.Context, accountID, tunnelID, apiToken string) (string, error) {
	if accountID == "" {
		return "", fmt.Errorf("account id is required")
	}
	if tunnelID == "" {
		return "", fmt.Errorf("tunnel id is required")
	}
	if apiToken == "" {
		return "", fmt.Errorf("api token is required")
	}

	baseURL := strings.TrimRight(c.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	url := fmt.Sprintf("%s/accounts/%s/cfd_tunnel/%s/token", baseURL, accountID, tunnelID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Cloudflare token request failed with %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return parseTunnelToken(body)
}

func parseTunnelToken(body []byte) (string, error) {
	var token string
	if err := json.Unmarshal(body, &token); err == nil && token != "" {
		return token, nil
	}

	var envelope struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Result != "" {
		return envelope.Result, nil
	}

	trimmed := string(bytes.TrimSpace(body))
	if trimmed != "" {
		return trimmed, nil
	}
	return "", fmt.Errorf("Cloudflare token response was empty")
}
