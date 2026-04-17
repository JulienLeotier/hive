package api

import (
	_ "embed"
	"net/http"
)

// openapiSpec est embeddé depuis docs/openapi.yaml au build. Servi tel
// quel par GET /api/openapi.yaml — un client OpenAPI (Swagger UI,
// Redoc, openapi-generator) peut le pointer directement sans avoir à
// cloner le repo.
//
//go:embed openapi.yaml
var openapiSpec []byte

// handleOpenAPI expose la spec OpenAPI 3.1 statique. Content-Type
// application/yaml + CORS * pour que des tools externes (Swagger UI
// hosted) puissent la charger sans proxy.
func (s *Server) handleOpenAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(openapiSpec)
}
