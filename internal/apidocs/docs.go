// Package apidocs serves the versioned Printa API contract and its interactive documentation page.
package apidocs

import (
	_ "embed"
	"net/http"
)

// OpenAPISpec is the canonical, versioned OpenAPI 3.0 contract for the public API.
//
//go:embed openapi.yaml
var OpenAPISpec []byte

// OpenAPIHandler serves the machine-readable API contract.
func OpenAPIHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(OpenAPISpec)
}

// DocsHandler serves a lightweight Swagger UI page that loads the embedded contract.
func DocsHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Printa API Reference</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
  <style>body { margin: 0; background: #fafafa; } .topbar { display: none; }</style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({
      url: '/api/v1/openapi.yaml',
      dom_id: '#swagger-ui',
      deepLinking: true,
      displayRequestDuration: true,
      persistAuthorization: false,
      validatorUrl: null
    });
  </script>
</body>
</html>`))
}
