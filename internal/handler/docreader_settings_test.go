package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"eino_agent/internal/config"
)

func TestGetSettingsIncludesDocReaderHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{cfg: &config.Config{DocReader: config.DocReaderConfig{
		Enabled:        true,
		Mode:           "mineru_with_fallback",
		Endpoint:       "localhost:50051",
		MinerUEndpoint: "http://localhost:8500",
		RenderMode:     "auto",
	}}}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)

	h.GetSettings(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var body struct {
		Settings struct {
			DocReader struct {
				Mode           string `json:"mode"`
				Active         bool   `json:"active"`
				Primary        string `json:"primary"`
				Fallback       string `json:"fallback"`
				MinerUEndpoint string `json:"mineru_endpoint"`
			} `json:"docreader"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Settings.DocReader.Mode != "mineru_with_fallback" {
		t.Fatalf("mode = %q", body.Settings.DocReader.Mode)
	}
	if body.Settings.DocReader.Primary != "mineru" || body.Settings.DocReader.Fallback != "local" {
		t.Fatalf("primary/fallback = %q/%q", body.Settings.DocReader.Primary, body.Settings.DocReader.Fallback)
	}
	if body.Settings.DocReader.MinerUEndpoint != "http://localhost:8500" {
		t.Fatalf("mineru_endpoint = %q", body.Settings.DocReader.MinerUEndpoint)
	}
}
