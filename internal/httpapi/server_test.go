package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"homelab-api/internal/domain"
)

type fakeStore struct {
	snapshot domain.Snapshot
	ready    bool
}

func (f *fakeStore) Snapshot() domain.Snapshot { return f.snapshot }
func (f *fakeStore) Ready() bool               { return f.ready }
func (f *fakeStore) Subscribe() (<-chan domain.Snapshot, func()) {
	return make(chan domain.Snapshot), func() {}
}

func TestHealthAndBearerAuth(t *testing.T) {
	store := &fakeStore{ready: true}
	handler := NewHandler(store, "test-token")

	healthRequest := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthResponse := httptest.NewRecorder()
	handler.ServeHTTP(healthResponse, healthRequest)
	if healthResponse.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d", healthResponse.Code, http.StatusOK)
	}

	unauthorizedRequest := httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil)
	unauthorizedResponse := httptest.NewRecorder()
	handler.ServeHTTP(unauthorizedResponse, unauthorizedRequest)
	if unauthorizedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want %d", unauthorizedResponse.Code, http.StatusUnauthorized)
	}

	authorizedRequest := httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil)
	authorizedRequest.Header.Set("Authorization", "Bearer test-token")
	authorizedResponse := httptest.NewRecorder()
	handler.ServeHTTP(authorizedResponse, authorizedRequest)
	if authorizedResponse.Code != http.StatusOK {
		t.Fatalf("authorized status = %d, want %d", authorizedResponse.Code, http.StatusOK)
	}
}
