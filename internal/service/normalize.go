package service

import (
	"strconv"
	"strings"

	"homelab-api/internal/beszel"
	"homelab-api/internal/domain"
)

func normalizeSystems(data beszel.RawData) []domain.System {
	bySystem := mergeRelatedMetrics(data.SystemStats, data.Realtime, "system", "system_id", "systemId")
	result := make([]domain.System, 0, len(data.Systems))
	for _, record := range data.Systems {
		id := recordID(record)
		metrics := numericFields(record)
		mergeMetricMap(metrics, bySystem[id])
		result = append(result, domain.System{
			ID:        id,
			Name:      firstString(record, "name", "system_name", "hostname", "host", "id"),
			Host:      firstString(record, "host", "hostname", "ip", "address"),
			Status:    statusValue(record),
			UpdatedAt: firstString(record, "updated", "updated_at", "last_seen", "last_update"),
			Metrics:   metrics,
		})
	}
	return result
}

func normalizeContainers(data beszel.RawData) []domain.Container {
	byContainer := mergeRelatedMetrics(data.ContainerStats, nil, "container", "container_id", "containerId")
	result := make([]domain.Container, 0, len(data.Containers))
	for _, record := range data.Containers {
		id := recordID(record)
		metrics := numericFields(record)
		mergeMetricMap(metrics, byContainer[id])
		result = append(result, domain.Container{
			ID:        id,
			Name:      firstString(record, "name", "container_name", "display_name", "id"),
			SystemID:  relatedID(record, "system", "system_id", "systemId"),
			Status:    firstString(record, "status", "state", "running"),
			Health:    firstString(record, "health", "health_status"),
			UpdatedAt: firstString(record, "updated", "updated_at", "last_seen"),
			Metrics:   metrics,
		})
	}
	return result
}

func normalizeAlerts(records []beszel.Record) []domain.Alert {
	result := make([]domain.Alert, 0, len(records))
	for _, record := range records {
		result = append(result, domain.Alert{
			ID:        recordID(record),
			Name:      firstString(record, "name", "alert_name", "title"),
			SystemID:  relatedID(record, "system", "system_id", "systemId"),
			Status:    firstString(record, "status", "state", "severity"),
			Message:   firstString(record, "message", "description", "detail"),
			CreatedAt: firstString(record, "created", "created_at", "updated", "updated_at"),
		})
	}
	return result
}

func mergeRelatedMetrics(primary, secondary []beszel.Record, relationKeys ...string) map[string]map[string]any {
	result := make(map[string]map[string]any)
	for _, records := range [][]beszel.Record{primary, secondary} {
		for _, record := range records {
			id := relatedID(record, relationKeys...)
			if id == "" {
				continue
			}
			if result[id] == nil {
				result[id] = make(map[string]any)
			}
			mergeMetricMap(result[id], numericFields(record))
		}
	}
	return result
}

func mergeMetricMap(target map[string]any, source map[string]any) {
	for key, value := range source {
		target[key] = value
	}
}

func recordID(record beszel.Record) string {
	return firstString(record, "id", "system_id", "systemId", "container_id", "containerId")
}

func relatedID(record beszel.Record, keys ...string) string {
	for _, key := range keys {
		value, ok := record[key]
		if !ok {
			continue
		}
		if text := stringValue(value); text != "" {
			return text
		}
		if nested, ok := value.(map[string]any); ok {
			if text := firstString(nested, "id", "system_id", "container_id"); text != "" {
				return text
			}
		}
	}
	return ""
}

func firstString(record map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := record[key]; ok {
			if text := stringValue(value); text != "" {
				return text
			}
		}
	}
	return ""
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
}

func statusValue(record map[string]any) string {
	if value := firstString(record, "status", "state"); value != "" {
		return strings.ToLower(value)
	}
	if value, ok := record["online"].(bool); ok {
		if value {
			return "online"
		}
		return "offline"
	}
	return "unknown"
}

func numericFields(record map[string]any) map[string]any {
	result := make(map[string]any)
	collectNumeric(result, record, "", 0)
	return result
}

func collectNumeric(result map[string]any, value any, prefix string, depth int) {
	if depth > 3 {
		return
	}
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if sensitiveKey(key) {
				continue
			}
			name := key
			if prefix != "" {
				name = prefix + "." + key
			}
			if nested, ok := child.(map[string]any); ok {
				collectNumeric(result, nested, name, depth+1)
				continue
			}
			if number, ok := numberValue(child); ok {
				result[name] = number
			}
		}
	case []any:
		// Arrays may contain identifiers or data that is not useful in the
		// normalized summary, so they are deliberately omitted.
	}
}

func numberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func sensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, blocked := range []string{"password", "secret", "token", "fingerprint", "private", "email", "apikey", "api_key"} {
		if strings.Contains(lower, blocked) {
			return true
		}
	}
	return false
}

func summarize(snapshot *domain.Snapshot) {
	snapshot.Summary = domain.Summary{
		SystemsTotal:    len(snapshot.Systems),
		ContainersTotal: len(snapshot.Containers),
		ServicesTotal:   len(snapshot.Services),
		ActiveAlerts:    len(snapshot.Alerts),
	}
	for _, system := range snapshot.Systems {
		if system.Status == "online" || system.Status == "up" || system.Status == "running" {
			snapshot.Summary.SystemsOnline++
		}
	}
	for _, service := range snapshot.Services {
		switch service.Status {
		case "up":
			snapshot.Summary.ServicesUp++
		case "down":
			snapshot.Summary.ServicesDown++
		}
	}
}

func sourceError(errorsByCollection map[string]string) string {
	if len(errorsByCollection) == 0 {
		return ""
	}
	return strconv.Itoa(len(errorsByCollection)) + " collection(s) unavailable"
}
