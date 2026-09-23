// Package api expose l'API REST unifiée du service.
//
// Les opérations sont déclarées via Huma, qui dérive la spécification OpenAPI
// 3.1 directement des types Go : le contrat publié ne peut donc pas diverger
// du code, et le client TypeScript du frontend en est généré.
package api

import (
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/stephguignard/domotic/internal/config"
	"github.com/stephguignard/domotic/internal/netatmo"
	"github.com/stephguignard/domotic/internal/poller"
	"github.com/stephguignard/domotic/internal/store"
	"github.com/stephguignard/domotic/internal/tahoma"
)

// Deps regroupe les dépendances des handlers. Les clients sont nil quand la
// source correspondante n'est pas configurée.
type Deps struct {
	Config  *config.Config
	Store   *store.Store
	Netatmo *netatmo.Client
	Tahoma  *tahoma.Client
	Poller  *poller.Poller
	Log     *slog.Logger
}

// Config construit la configuration Huma du service.
func Config(version string) huma.Config {
	cfg := huma.DefaultConfig("Domotic API", version)
	cfg.Info.Description = "API unifiée d'agrégation des équipements Netatmo et Somfy TaHoma."
	cfg.Servers = []*huma.Server{{URL: "/"}}
	return cfg
}

// Register déclare toutes les opérations de l'API.
func Register(api huma.API, d Deps) {
	registerDevices(api, d)
	registerMeasurements(api, d)
	registerHealth(api, d)
}

// RegisterAuthRoutes déclare le flux OAuth2 Netatmo directement sur le mux.
//
// Ces routes restent hors de l'API documentée : elles reposent sur des
// redirections navigateur, que le client TypeScript généré ne saurait pas
// exploiter, et ne sont parcourues qu'une fois à la configuration initiale.
func RegisterAuthRoutes(mux *http.ServeMux, d Deps) {
	if d.Netatmo == nil {
		return
	}
	h := &authHandler{deps: d}
	mux.HandleFunc("GET /auth/netatmo", h.start)
	mux.HandleFunc("GET /auth/netatmo/callback", h.callback)
	mux.HandleFunc("GET /auth/netatmo/status", h.status)
}
