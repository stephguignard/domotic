package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/stephguignard/domotic/internal/store"
)

// SetRoomInput porte la pièce choisie pour un équipement.
type SetRoomInput struct {
	ID   string `path:"id" doc:"Identifiant de l'équipement"`
	Body struct {
		Room string `json:"room" maxLength:"64" doc:"Pièce choisie ; vide pour ranger l'équipement sans pièce"`
	}
}

// RoomOutput renvoie l'équipement avec sa pièce à jour.
type RoomOutput struct {
	Body store.Device
}

func registerRooms(api huma.API, d Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "set-device-room",
		Method:      http.MethodPut,
		Path:        "/api/devices/{id}/room",
		Summary:     "Choisir la pièce d'un équipement",
		Description: "Range l'équipement dans une pièce, indépendamment de celle que fournit sa source. " +
			"Le choix survit aux rafraîchissements et entre dans l'historique des actions.",
		Tags: []string{"Devices"},
	}, func(ctx context.Context, in *SetRoomInput) (*RoomOutput, error) {
		room := strings.TrimSpace(in.Body.Room)
		return d.changeRoom(ctx, in.ID, &room, "setRoom", []any{room})
	})

	huma.Register(api, huma.Operation{
		OperationID: "reset-device-room",
		Method:      http.MethodDelete,
		Path:        "/api/devices/{id}/room",
		Summary:     "Rétablir la pièce de la source",
		Description: "Annule le choix fait dans l'interface : l'équipement reprend la pièce fournie par sa source.",
		Tags:        []string{"Devices"},
	}, func(ctx context.Context, in *GetDeviceInput) (*RoomOutput, error) {
		return d.changeRoom(ctx, in.ID, nil, "resetRoom", nil)
	})
}

// changeRoom applique un choix de pièce, le consigne, et renvoie l'équipement
// à jour.
func (d Deps) changeRoom(ctx context.Context, id string, room *string, action string, params []any) (*RoomOutput, error) {
	device, err := d.Store.GetDevice(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, huma.Error404NotFound("équipement introuvable")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("lecture de l'équipement", err)
	}

	if err := d.Store.SetRoomOverride(ctx, id, room); err != nil {
		d.recordCommand(ctx, device, action, params, err.Error())
		return nil, huma.Error500InternalServerError("enregistrement de la pièce", err)
	}
	d.recordCommand(ctx, device, action, params, "")

	updated, err := d.Store.GetDevice(ctx, id)
	if err != nil {
		return nil, huma.Error500InternalServerError("lecture de l'équipement", err)
	}
	return &RoomOutput{Body: updated}, nil
}
