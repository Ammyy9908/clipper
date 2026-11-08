package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ammyy9908/clipper/internal/config"
)

func TestPublicConfigReportsUploadLimits(t *testing.T) {
	a := &API{cfg: config.Config{MaxUploadBytes: 512 << 20}}
	rec := httptest.NewRecorder()
	a.publicConfig(rec, httptest.NewRequest(http.MethodGet, "/v1/config", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body struct {
		Max  int64    `json:"max_upload_bytes"`
		Exts []string `json:"allowed_upload_extensions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Max != 512<<20 {
		t.Errorf("max_upload_bytes = %d, want %d", body.Max, 512<<20)
	}
	// It must mirror what POST /v1/uploads actually accepts, in a stable order.
	want := []string{".m4v", ".mkv", ".mov", ".mp4", ".webm"}
	if len(body.Exts) != len(want) {
		t.Fatalf("extensions = %v, want %v", body.Exts, want)
	}
	for i := range want {
		if body.Exts[i] != want[i] {
			t.Errorf("extensions = %v, want %v", body.Exts, want)
			break
		}
	}
	for _, e := range body.Exts {
		if !allowedExt[e] {
			t.Errorf("%q is advertised but the upload route would refuse it", e)
		}
	}
}
