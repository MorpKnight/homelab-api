package domain

import "time"

type Snapshot struct {
	GeneratedAt time.Time      `json:"generated_at"`
	Sources     []SourceStatus `json:"sources"`
	Systems     []System       `json:"systems"`
	Containers  []Container    `json:"containers"`
	Services    []Service      `json:"services"`
	Alerts      []Alert        `json:"alerts"`
	Summary     Summary        `json:"summary"`
}

type SourceStatus struct {
	Name          string     `json:"name"`
	Configured    bool       `json:"configured"`
	Status        string     `json:"status"`
	LastSuccessAt *time.Time `json:"last_success_at,omitempty"`
	Error         string     `json:"error,omitempty"`
}

type System struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Host      string         `json:"host,omitempty"`
	Status    string         `json:"status"`
	UpdatedAt string         `json:"updated_at,omitempty"`
	Metrics   map[string]any `json:"metrics,omitempty"`
}

type Container struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	SystemID  string         `json:"system_id,omitempty"`
	Status    string         `json:"status"`
	Health    string         `json:"health,omitempty"`
	UpdatedAt string         `json:"updated_at,omitempty"`
	Metrics   map[string]any `json:"metrics,omitempty"`
}

type Service struct {
	Name         string             `json:"name"`
	Type         string             `json:"type,omitempty"`
	URL          string             `json:"url,omitempty"`
	Host         string             `json:"host,omitempty"`
	Port         string             `json:"port,omitempty"`
	Status       string             `json:"status"`
	ResponseTime float64            `json:"response_time,omitempty"`
	Metrics      map[string]float64 `json:"metrics,omitempty"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

type Alert struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	SystemID  string `json:"system_id,omitempty"`
	Status    string `json:"status,omitempty"`
	Message   string `json:"message,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

type Summary struct {
	SystemsTotal    int `json:"systems_total"`
	SystemsOnline   int `json:"systems_online"`
	ContainersTotal int `json:"containers_total"`
	ServicesTotal   int `json:"services_total"`
	ServicesUp      int `json:"services_up"`
	ServicesDown    int `json:"services_down"`
	ActiveAlerts    int `json:"active_alerts"`
}
