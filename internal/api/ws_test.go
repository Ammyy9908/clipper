package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJobWebSocketInvalidID(t *testing.T) {
	a := authAPI(t, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/invalid-uuid/ws", nil)
	req.SetPathValue("id", "invalid-uuid")

	a.jobWebSocket(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for invalid job id, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid job id") {
		t.Fatalf("expected error message in body, got %s", rec.Body.String())
	}
}
