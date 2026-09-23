package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/stephguignard/domotic/internal/store"
)

// ListMeasurementsInput porte les filtres de l'historique des relevés.
type ListMeasurementsInput struct {
	ID     string    `path:"id" doc:"Identifiant de l'équipement"`
	Metric string    `query:"metric" doc:"Ne retourner que cette grandeur, ex. temperature"`
	From   time.Time `query:"from" doc:"Début de la plage (RFC 3339)"`
	To     time.Time `query:"to" doc:"Fin de la plage (RFC 3339)"`
	Limit  int       `query:"limit" minimum:"1" maximum:"5000" default:"1000" doc:"Nombre maximum de relevés"`
}

// ListMeasurementsOutput est la réponse de l'historique des relevés.
type ListMeasurementsOutput struct {
	Body struct {
		Measurements []store.Measurement `json:"measurements" nullable:"false" doc:"Relevés, du plus récent au plus ancien"`
		Total        int                 `json:"total" doc:"Nombre de relevés retournés"`
	}
}

func registerMeasurements(api huma.API, d Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "list-measurements",
		Method:      http.MethodGet,
		Path:        "/api/devices/{id}/measurements",
		Summary:     "Historique des relevés",
		Description: "Retourne les relevés enregistrés pour un équipement, du plus récent au plus ancien.",
		Tags:        []string{"Measurements"},
	}, func(ctx context.Context, in *ListMeasurementsInput) (*ListMeasurementsOutput, error) {
		// Vérifier l'existence permet de distinguer « équipement inconnu » de
		// « équipement sans relevé », que renverrait autrement une liste vide.
		if _, err := d.Store.GetDevice(ctx, in.ID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, huma.Error404NotFound("équipement introuvable")
			}
			return nil, huma.Error500InternalServerError("lecture de l'équipement", err)
		}

		if !in.From.IsZero() && !in.To.IsZero() && in.To.Before(in.From) {
			return nil, huma.Error422UnprocessableEntity("la borne 'to' précède la borne 'from'")
		}

		measurements, err := d.Store.ListMeasurements(ctx, store.MeasurementFilter{
			DeviceID: in.ID,
			Metric:   in.Metric,
			From:     in.From,
			To:       in.To,
			Limit:    in.Limit,
		})
		if err != nil {
			return nil, huma.Error500InternalServerError("lecture des relevés", err)
		}

		out := &ListMeasurementsOutput{}
		out.Body.Measurements = measurements
		out.Body.Total = len(measurements)
		return out, nil
	})
}
