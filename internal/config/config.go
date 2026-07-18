package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contains runtime configuration. Secrets are loaded from environment
// variables, with optional *_FILE support for deployments using secret mounts.
type Config struct {
	Port            int
	APIToken        string
	PollInterval    time.Duration
	UpstreamTimeout time.Duration

	BeszelURL                   string
	BeszelIdentity              string
	BeszelPassword              string
	BeszelSystemsCollection     string
	BeszelSystemStatsCollection string
	BeszelContainersCollection  string
	BeszelContainerStats        string
	BeszelRealtimeCollection    string
	BeszelAlertsCollection      string

	UptimeKumaURL    string
	UptimeKumaAPIKey string
}

func Load() (Config, error) {
	port, err := intEnv("PORT", 8080)
	if err != nil {
		return Config{}, err
	}
	pollInterval, err := durationEnv("POLL_INTERVAL", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	upstreamTimeout, err := durationEnv("UPSTREAM_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}

	apiToken, err := secretEnv("API_TOKEN")
	if err != nil {
		return Config{}, err
	}
	beszelPassword, err := secretEnv("BESZEL_PASSWORD")
	if err != nil {
		return Config{}, err
	}
	kumaAPIKey, err := secretEnv("UPTIME_KUMA_API_KEY")
	if err != nil {
		return Config{}, err
	}

	return Config{
		Port:                        port,
		APIToken:                    apiToken,
		PollInterval:                pollInterval,
		UpstreamTimeout:             upstreamTimeout,
		BeszelURL:                   strings.TrimRight(strings.TrimSpace(os.Getenv("BESZEL_URL")), "/"),
		BeszelIdentity:              strings.TrimSpace(os.Getenv("BESZEL_IDENTITY")),
		BeszelPassword:              beszelPassword,
		BeszelSystemsCollection:     envOr("BESZEL_COLLECTION_SYSTEMS", "systems"),
		BeszelSystemStatsCollection: envOr("BESZEL_COLLECTION_SYSTEM_STATS", "system_stats"),
		BeszelContainersCollection:  envOr("BESZEL_COLLECTION_CONTAINERS", "containers"),
		BeszelContainerStats:        envOr("BESZEL_COLLECTION_CONTAINER_STATS", "container_stats"),
		BeszelRealtimeCollection:    envOr("BESZEL_COLLECTION_REALTIME", "rt_metrics"),
		BeszelAlertsCollection:      envOr("BESZEL_COLLECTION_ALERTS", "alerts"),
		UptimeKumaURL:               strings.TrimRight(strings.TrimSpace(os.Getenv("UPTIME_KUMA_URL")), "/"),
		UptimeKumaAPIKey:            kumaAPIKey,
	}, nil
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func intEnv(name string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

func durationEnv(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return parsed, nil
}

func secretEnv(name string) (string, error) {
	if value := os.Getenv(name); value != "" {
		return value, nil
	}
	if path := strings.TrimSpace(os.Getenv(name + "_FILE")); path != "" {
		contents, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read %s_FILE: %w", name, err)
		}
		return strings.TrimSpace(string(contents)), nil
	}
	return "", nil
}
