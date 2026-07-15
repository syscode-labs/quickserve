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

type Zone struct {
	ID      string
	Name    string
	Account Account
}

type Account struct {
	ID   string
	Name string
}

type Tunnel struct {
	ID     string
	Name   string
	Status string
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

	body, err := c.get(ctx, fmt.Sprintf("/accounts/%s/cfd_tunnel/%s/token", accountID, tunnelID), nil, apiToken)
	if err != nil {
		return "", err
	}
	return parseTunnelToken(body)
}

func (c Client) ZoneByName(ctx context.Context, name, apiToken string) (Zone, error) {
	if name == "" {
		return Zone{}, fmt.Errorf("zone name is required")
	}
	body, err := c.get(ctx, "/zones", map[string]string{"name": name, "per_page": "1"}, apiToken)
	if err != nil {
		return Zone{}, err
	}
	var envelope struct {
		Result []Zone `json:"result"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return Zone{}, err
	}
	if len(envelope.Result) == 0 {
		return Zone{}, fmt.Errorf("zone %q was not found", name)
	}
	return envelope.Result[0], nil
}

func (c Client) FindZoneForHostname(ctx context.Context, hostname, apiToken string) (Zone, error) {
	parts := strings.Split(strings.Trim(hostname, "."), ".")
	for i := 0; i < len(parts)-1; i++ {
		name := strings.Join(parts[i:], ".")
		zone, err := c.ZoneByName(ctx, name, apiToken)
		if err == nil {
			return zone, nil
		}
		if !strings.Contains(err.Error(), "was not found") {
			return Zone{}, err
		}
	}
	return Zone{}, fmt.Errorf("no Cloudflare zone found for hostname %q", hostname)
}

func (c Client) Tunnels(ctx context.Context, accountID, name, apiToken string) ([]Tunnel, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account id is required")
	}
	query := map[string]string{"per_page": "100"}
	if name != "" {
		query["name"] = name
	}
	body, err := c.get(ctx, fmt.Sprintf("/accounts/%s/tunnels", accountID), query, apiToken)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Result []Tunnel `json:"result"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	return envelope.Result, nil
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

func (c Client) get(ctx context.Context, path string, query map[string]string, apiToken string) ([]byte, error) {
	if apiToken == "" {
		return nil, fmt.Errorf("api token is required")
	}
	baseURL := strings.TrimRight(c.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	values := req.URL.Query()
	for key, value := range query {
		values.Set(key, value)
	}
	req.URL.RawQuery = values.Encode()

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Cloudflare request failed with %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return body, nil
}
