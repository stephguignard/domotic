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

// RoomOrderBody porte l'ordre d'affichage des pièces.
type RoomOrderBody struct {
	Rooms []string `json:"rooms" nullable:"false" maxItems:"100" doc:"Pièces, dans l'ordre d'affichage ; vide pour revenir à l'ordre alphabétique"`
}

// RoomOrderInput porte le nouvel ordre des pièces.
type RoomOrderInput struct {
	Body RoomOrderBody
}

// RoomOrderOutput renvoie l'ordre des pièces enregistré.
type RoomOrderOutput struct {
	Body RoomOrderBody
}

// RoomOutput renvoie l'équipement avec sa pièce à jour.
type RoomOutput struct {
	Body store.Device
}

func registerRooms(api huma.API, d Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "get-room-order",
		Method:      http.MethodGet,
		Path:        "/api/rooms/order",
		Summary:     "Ordre d'affichage des pièces",
		Description: "Retourne les pièces classées, dans l'ordre choisi. Les pièces absentes de la liste " +
			"n'ont pas été classées : l'interface les affiche ensuite, par ordre alphabétique.",
		Tags: []string{"Devices"},
	}, func(ctx context.Context, _ *struct{}) (*RoomOrderOutput, error) {
		rooms, err := d.Store.RoomOrder(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("lecture de l'ordre des pièces", err)
		}
		return &RoomOrderOutput{Body: RoomOrderBody{Rooms: rooms}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "set-room-order",
		Method:      http.MethodPut,
		Path:        "/api/rooms/order",
		Summary:     "Choisir l'ordre d'affichage des pièces",
		Description: "Remplace l'ordre des pièces. Une liste vide revient à l'ordre alphabétique.",
		Tags:        []string{"Devices"},
	}, func(ctx context.Context, in *RoomOrderInput) (*RoomOrderOutput, error) {
		rooms := make([]string, 0, len(in.Body.Rooms))
		seen := map[string]bool{}
		for _, r := range in.Body.Rooms {
			r = strings.TrimSpace(r)
			if r == "" {
				return nil, huma.Error422UnprocessableEntity("une pièce sans nom ne se classe pas")
			}
			if seen[r] {
				return nil, huma.Error422UnprocessableEntity("pièce en double : " + r)
			}
			seen[r] = true
			rooms = append(rooms, r)
		}

		if err := d.Store.SetRoomOrder(ctx, rooms); err != nil {
			return nil, huma.Error500InternalServerError("enregistrement de l'ordre des pièces", err)
		}
		return &RoomOrderOutput{Body: RoomOrderBody{Rooms: rooms}}, nil
	})

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
