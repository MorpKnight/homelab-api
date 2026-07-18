package uptimekuma

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"homelab-api/internal/domain"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewClient(baseURL, apiKey string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:     apiKey,
		httpClient: httpClient,
	}
}

func (c *Client) Configured() bool {
	return c != nil && c.baseURL != ""
}

func (c *Client) Fetch(ctx context.Context) ([]domain.Service, error) {
	if !c.Configured() {
		return nil, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/metrics", nil)
	if err != nil {
		return nil, err
	}
	if c.apiKey != "" {
		// Uptime Kuma's Prometheus API key is supplied as the HTTP Basic password.
		request.SetBasicAuth("", c.apiKey)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("uptime kuma returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("read uptime kuma metrics: %w", err)
	}
	metrics, err := ParseMetrics(string(body))
	if err != nil {
		return nil, err
	}
	return servicesFromMetrics(metrics, time.Now().UTC()), nil
}

func servicesFromMetrics(metrics []Metric, updatedAt time.Time) []domain.Service {
	type serviceKey struct {
		name, typ, url, host, port string
	}
	services := make(map[serviceKey]*domain.Service)
	for _, metric := range metrics {
		key := serviceKey{
			name: metric.Labels["monitor_name"],
			typ:  metric.Labels["monitor_type"],
			url:  safeMonitorURL(metric.Labels["monitor_url"]),
			host: metric.Labels["monitor_hostname"],
			port: metric.Labels["monitor_port"],
		}
		service, ok := services[key]
		if !ok {
			service = &domain.Service{
				Name:      key.name,
				Type:      key.typ,
				URL:       key.url,
				Host:      key.host,
				Port:      key.port,
				Status:    "unknown",
				Metrics:   make(map[string]float64),
				UpdatedAt: updatedAt,
			}
			services[key] = service
		}
		switch metric.Name {
		case "monitor_status":
			service.Status = monitorStatus(metric.Value)
		case "monitor_response_time":
			service.ResponseTime = metric.Value
		default:
			if strings.HasPrefix(metric.Name, "monitor_") {
				service.Metrics[strings.TrimPrefix(metric.Name, "monitor_")] = metric.Value
			}
		}
	}
	result := make([]domain.Service, 0, len(services))
	for _, service := range services {
		result = append(result, *service)
	}
	sortServices(result)
	return result
}

func safeMonitorURL(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func monitorStatus(value float64) string {
	switch value {
	case 1:
		return "up"
	case 0:
		return "down"
	case 2:
		return "pending"
	case 3:
		return "maintenance"
	default:
		return "unknown"
	}
}

func sortServices(services []domain.Service) {
	for i := 1; i < len(services); i++ {
		current := services[i]
		j := i - 1
		for j >= 0 && strings.ToLower(services[j].Name) > strings.ToLower(current.Name) {
			services[j+1] = services[j]
			j--
		}
		services[j+1] = current
	}
}
