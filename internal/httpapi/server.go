package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"homelab-api/internal/domain"
)

type SnapshotReader interface {
	Snapshot() domain.Snapshot
	Ready() bool
	Subscribe() (<-chan domain.Snapshot, func())
}

func NewHandler(store SnapshotReader, apiToken string) http.Handler {
	api := http.NewServeMux()
	api.HandleFunc("GET /v1/overview", func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusOK, store.Snapshot())
	})
	api.HandleFunc("GET /v1/systems", func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusOK, map[string]any{"items": store.Snapshot().Systems})
	})
	api.HandleFunc("GET /v1/containers", func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusOK, map[string]any{"items": store.Snapshot().Containers})
	})
	api.HandleFunc("GET /v1/services", func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusOK, map[string]any{"items": store.Snapshot().Services})
	})
	api.HandleFunc("GET /v1/alerts", func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusOK, map[string]any{"items": store.Snapshot().Alerts})
	})
	api.HandleFunc("GET /v1/events", func(writer http.ResponseWriter, request *http.Request) {
		handleEvents(writer, request, store)
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(writer http.ResponseWriter, request *http.Request) {
		if !store.Ready() {
			writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
			return
		}
		writeJSON(writer, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.Handle("/api/", http.StripPrefix("/api", requireBearer(api, apiToken)))
	return mux
}

func requireBearer(next http.Handler, token string) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if token == "" {
			next.ServeHTTP(writer, request)
			return
		}
		const prefix = "Bearer "
		authorization := request.Header.Get("Authorization")
		if !strings.HasPrefix(authorization, prefix) || strings.TrimSpace(strings.TrimPrefix(authorization, prefix)) != token {
			writer.Header().Set("WWW-Authenticate", `Bearer realm="homelab-api"`)
			writeJSON(writer, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func handleEvents(writer http.ResponseWriter, request *http.Request, store SnapshotReader) {
	flusher, ok := writer.(http.Flusher)
	if !ok {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.WriteHeader(http.StatusOK)

	updates, cancel := store.Subscribe()
	defer cancel()
	keepAlive := time.NewTicker(25 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-request.Context().Done():
			return
		case snapshot, ok := <-updates:
			if !ok {
				return
			}
			payload, err := json.Marshal(snapshot)
			if err != nil {
				return
			}
			_, _ = fmt.Fprintf(writer, "event: snapshot\ndata: %s\n\n", payload)
			flusher.Flush()
		case <-keepAlive.C:
			_, _ = fmt.Fprint(writer, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
