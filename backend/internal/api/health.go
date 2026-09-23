package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/stephguignard/domotic/internal/poller"
)

// HealthOutput décrit l'état du service et de ses sources.
type HealthOutput struct {
	Body struct {
		Status   string                         `json:"status" enum:"ok,degraded" doc:"État global du service"`
		Database bool                           `json:"database" doc:"La base de données répond-elle ?"`
		Sources  map[string]poller.SourceStatus `json:"sources" doc:"État de chaque source, par nom"`
	}
}

func registerHealth(api huma.API, d Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "health",
		Method:      http.MethodGet,
		Path:        "/api/health",
		Summary:     "État du service",
		Description: "Retourne l'état de la base et de chaque source configurée. " +
			"Le statut global est 'degraded' dès qu'une source activée est en échec.",
		Tags: []string{"Health"},
	}, func(ctx context.Context, _ *struct{}) (*HealthOutput, error) {
		out := &HealthOutput{}
		out.Body.Database = d.Store.Ping(ctx) == nil
		out.Body.Sources = d.Poller.Status()

		// Une source configurée mais en échec dégrade le service sans le rendre
		// indisponible : les données déjà consolidées restent servies.
		healthy := out.Body.Database
		for _, s := range out.Body.Sources {
			if s.Enabled && !s.Healthy {
				healthy = false
			}
		}

		out.Body.Status = "degraded"
		if healthy {
			out.Body.Status = "ok"
		}
		return out, nil
	})
}
