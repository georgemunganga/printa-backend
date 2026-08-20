package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfiguredCORSOriginsIncludesAdminConsole(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	origins := configuredCORSOrigins()
	if !origins["https://admin.printa.co.zm"] {
		t.Fatal("admin.printa.co.zm must be an explicitly trusted browser origin")
	}
}

func TestCORSMiddlewareAllowsAdminConsolePreflight(t *testing.T) {
	handler := corsMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("preflight requests must not reach the next handler")
	}))

	request := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/otp/request", nil)
	request.Header.Set("Origin", "https://admin.printa.co.zm")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, response.Code)
	}
	if origin := response.Header().Get("Access-Control-Allow-Origin"); origin != "https://admin.printa.co.zm" {
		t.Fatalf("expected admin console allow-origin header, got %q", origin)
	}
	if methods := response.Header().Get("Access-Control-Allow-Methods"); methods == "" {
		t.Fatal("expected allowed HTTP methods header")
	}
}
