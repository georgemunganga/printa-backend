package apidocs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAPIHandlerServesContract(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/openapi.yaml", nil)
	rec := httptest.NewRecorder()

	OpenAPIHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/yaml") {
		t.Fatalf("content type = %q, want application/yaml", rec.Header().Get("Content-Type"))
	}

	body := rec.Body.String()
	for _, required := range []string{
		"openapi: 3.0.3",
		"/api/v1/auth/login:",
		"/api/v1/orders:",
		"/api/v1/comms/send:",
		"bearerAuth:",
	} {
		if !strings.Contains(body, required) {
			t.Errorf("contract is missing %q", required)
		}
	}
}

func TestDocsHandlerServesSwaggerUI(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/docs", nil)
	rec := httptest.NewRecorder()

	DocsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("content type = %q, want text/html", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), "SwaggerUIBundle") {
		t.Fatal("documentation page does not initialize Swagger UI")
	}
	if !strings.Contains(rec.Body.String(), "/api/v1/openapi.yaml") {
		t.Fatal("documentation page does not load the versioned OpenAPI contract")
	}
}
