package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/stephguignard/domotic/internal/store"
)

// ListCommandsInput porte les filtres de l'historique des commandes.
type ListCommandsInput struct {
	DeviceID string `query:"device_id" doc:"Ne retourner que les actions sur cet équipement"`
	Limit    int    `query:"limit" minimum:"1" maximum:"1000" default:"200" doc:"Nombre maximum d'entrées"`
}

// ListCommandsOutput est la réponse de l'historique des commandes.
type ListCommandsOutput struct {
	Body struct {
		Entries []store.CommandLogEntry `json:"entries" nullable:"false" doc:"Actions, de la plus récente à la plus ancienne"`
		Total   int                     `json:"total" doc:"Nombre d'entrées retournées"`
	}
}

func registerCommands(api huma.API, d Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "list-commands",
		Method:      http.MethodGet,
		Path:        "/api/commands",
		Summary:     "Historique des actions",
		Description: "Retourne les actions faites depuis l'interface — commandes, réussies ou non, et " +
			"changements de pièce (setRoom, resetRoom) —, de la plus récente à la plus ancienne. " +
			"Un équipement disparu depuis garde son historique.",
		Tags: []string{"History"},
	}, func(ctx context.Context, in *ListCommandsInput) (*ListCommandsOutput, error) {
		entries, err := d.Store.ListCommands(ctx, store.CommandLogFilter{DeviceID: in.DeviceID, Limit: in.Limit})
		if err != nil {
			return nil, huma.Error500InternalServerError("lecture de l'historique", err)
		}

		out := &ListCommandsOutput{}
		out.Body.Entries = entries
		out.Body.Total = len(entries)
		return out, nil
	})
}
