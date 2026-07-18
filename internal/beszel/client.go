package beszel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type Record map[string]any

type RawData struct {
	Systems        []Record
	SystemStats    []Record
	Containers     []Record
	ContainerStats []Record
	Realtime       []Record
	Alerts         []Record
}

type Client struct {
	baseURL     string
	identity    string
	password    string
	collections collections
	httpClient  *http.Client
	tokenMu     sync.RWMutex
	token       string
}

type collections struct {
	systems        string
	systemStats    string
	containers     string
	containerStats string
	realtime       string
	alerts         string
}

type Config struct {
	BaseURL                  string
	Identity                 string
	Password                 string
	SystemsCollection        string
	SystemStatsCollection    string
	ContainersCollection     string
	ContainerStatsCollection string
	RealtimeCollection       string
	AlertsCollection         string
	HTTPClient               *http.Client
}

func NewClient(cfg Config) *Client {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		baseURL:  strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		identity: cfg.Identity,
		password: cfg.Password,
		collections: collections{
			systems:        collectionOr(cfg.SystemsCollection, "systems"),
			systemStats:    collectionOr(cfg.SystemStatsCollection, "system_stats"),
			containers:     collectionOr(cfg.ContainersCollection, "containers"),
			containerStats: collectionOr(cfg.ContainerStatsCollection, "container_stats"),
			realtime:       collectionOr(cfg.RealtimeCollection, "rt_metrics"),
			alerts:         collectionOr(cfg.AlertsCollection, "alerts"),
		},
		httpClient: httpClient,
	}
}

func collectionOr(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}

func (c *Client) Configured() bool {
	return c != nil && c.baseURL != ""
}

func (c *Client) Fetch(ctx context.Context) (RawData, map[string]string) {
	data := RawData{}
	errorsByCollection := make(map[string]string)
	if !c.Configured() {
		return data, errorsByCollection
	}

	fetch := func(name string, target *[]Record, optional bool) {
		if strings.TrimSpace(name) == "" {
			return
		}
		records, err := c.list(ctx, name)
		if err != nil {
			if optional {
				return
			}
			errorsByCollection[name] = safeError(err)
			return
		}
		*target = records
	}

	fetch(c.collections.systems, &data.Systems, false)
	fetch(c.collections.systemStats, &data.SystemStats, false)
	fetch(c.collections.containers, &data.Containers, false)
	fetch(c.collections.containerStats, &data.ContainerStats, false)
	// Realtime metrics are supplementary and may be unavailable to readonly users.
	fetch(c.collections.realtime, &data.Realtime, true)
	fetch(c.collections.alerts, &data.Alerts, false)
	return data, errorsByCollection
}

func (c *Client) list(ctx context.Context, collection string) ([]Record, error) {
	request := func() ([]Record, int, error) {
		endpoint := c.baseURL + "/api/collections/" + url.PathEscape(collection) + "/records?perPage=500"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, 0, err
		}
		if token := c.currentToken(); token != "" {
			req.Header.Set("Authorization", token)
		}
		response, err := c.httpClient.Do(req)
		if err != nil {
			return nil, 0, err
		}
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, response.StatusCode, fmt.Errorf("beszel returned HTTP %d", response.StatusCode)
		}
		var payload struct {
			Items []Record `json:"items"`
		}
		if err := json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&payload); err != nil {
			return nil, response.StatusCode, fmt.Errorf("decode beszel response: %w", err)
		}
		return payload.Items, response.StatusCode, nil
	}

	items, status, err := request()
	if err == nil {
		return items, nil
	}
	if status == http.StatusUnauthorized {
		if authErr := c.authenticate(ctx); authErr != nil {
			return nil, authErr
		}
		items, _, err = request()
	}
	return items, err
}

func (c *Client) authenticate(ctx context.Context) error {
	if c.identity == "" || c.password == "" {
		return errors.New("beszel credentials are not configured")
	}
	payload := map[string]string{
		"identity": c.identity,
		"password": c.password,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	endpoint := c.baseURL + "/api/collections/users/auth-with-password"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("beszel authentication returned HTTP %d", response.StatusCode)
	}
	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
		return fmt.Errorf("decode beszel authentication response: %w", err)
	}
	if result.Token == "" {
		return errors.New("beszel authentication response did not include a token")
	}
	c.tokenMu.Lock()
	c.token = result.Token
	c.tokenMu.Unlock()
	return nil
}

func (c *Client) currentToken() string {
	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()
	return c.token
}

func safeError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "password") || strings.Contains(message, "token") || strings.Contains(message, "secret") {
		return "upstream authentication failed"
	}
	return err.Error()
}
