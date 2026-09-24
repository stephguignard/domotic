package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/stephguignard/domotic/internal/command"
	"github.com/stephguignard/domotic/internal/store"
)

// ListDevicesInput porte les filtres de la liste d'équipements.
type ListDevicesInput struct {
	Source string `query:"source" enum:"netatmo,tahoma,hue,shelly" doc:"Ne retourner que les équipements de cette source"`
	Room   string `query:"room" doc:"Ne retourner que les équipements de cette pièce"`
}

// ListDevicesOutput est la réponse de la liste d'équipements.
type ListDevicesOutput struct {
	Body struct {
		Devices []store.Device `json:"devices" nullable:"false" doc:"Équipements correspondant aux filtres"`
		Total   int            `json:"total" doc:"Nombre d'équipements retournés"`
	}
}

// GetDeviceInput identifie un équipement.
type GetDeviceInput struct {
	ID string `path:"id" doc:"Identifiant de l'équipement"`
}

// GetDeviceOutput est la réponse du détail d'un équipement.
type GetDeviceOutput struct {
	Body store.Device
}

// CommandInput décrit une commande à exécuter sur un équipement.
type CommandInput struct {
	ID   string `path:"id" doc:"Identifiant de l'équipement"`
	Body struct {
		Command    string `json:"command" minLength:"1" doc:"Nom de la commande, ex. open, close, on, off, setBrightness"`
		Parameters []any  `json:"parameters,omitempty" doc:"Paramètres de la commande, selon l'équipement"`
	}
}

// CommandOutput confirme la prise en compte d'une commande.
type CommandOutput struct {
	Body struct {
		ExecID string `json:"exec_id" doc:"Identifiant d'exécution attribué par la passerelle, vide si la source n'en attribue pas"`
	}
}

// ListRoomsOutput est la réponse de la liste des pièces.
type ListRoomsOutput struct {
	Body struct {
		Rooms []string `json:"rooms" nullable:"false" doc:"Pièces connues, sans doublon"`
	}
}

func registerDevices(api huma.API, d Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "list-devices",
		Method:      http.MethodGet,
		Path:        "/api/devices",
		Summary:     "Lister les équipements",
		Description: "Retourne les équipements consolidés depuis toutes les sources configurées.",
		Tags:        []string{"Devices"},
	}, func(ctx context.Context, in *ListDevicesInput) (*ListDevicesOutput, error) {
		devices, err := d.Store.ListDevices(ctx, store.DeviceFilter{Source: in.Source, Room: in.Room})
		if err != nil {
			return nil, huma.Error500InternalServerError("lecture des équipements", err)
		}

		out := &ListDevicesOutput{}
		out.Body.Devices = devices
		out.Body.Total = len(devices)
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-device",
		Method:      http.MethodGet,
		Path:        "/api/devices/{id}",
		Summary:     "Détail d'un équipement",
		Tags:        []string{"Devices"},
	}, func(ctx context.Context, in *GetDeviceInput) (*GetDeviceOutput, error) {
		device, err := d.Store.GetDevice(ctx, in.ID)
		if errors.Is(err, store.ErrNotFound) {
			return nil, huma.Error404NotFound("équipement introuvable")
		}
		if err != nil {
			return nil, huma.Error500InternalServerError("lecture de l'équipement", err)
		}
		return &GetDeviceOutput{Body: device}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "send-command",
		Method:      http.MethodPost,
		Path:        "/api/devices/{id}/command",
		Summary:     "Envoyer une commande",
		Description: "Transmet une commande à l'équipement. Les équipements TaHoma, Hue et Shelly sont pilotables ; " +
			"l'API Netatmo météo est en lecture seule.",
		Tags:          []string{"Devices"},
		DefaultStatus: http.StatusAccepted,
	}, func(ctx context.Context, in *CommandInput) (*CommandOutput, error) {
		device, err := d.Store.GetDevice(ctx, in.ID)
		if errors.Is(err, store.ErrNotFound) {
			return nil, huma.Error404NotFound("équipement introuvable")
		}
		if err != nil {
			return nil, huma.Error500InternalServerError("lecture de l'équipement", err)
		}

		commander, controllable := d.commander(device.Source)
		if !controllable {
			return nil, huma.Error422UnprocessableEntity(
				"les équipements " + device.Source + " ne sont pas pilotables")
		}
		if commander == nil {
			return nil, huma.Error503ServiceUnavailable("intégration " + device.Source + " non configurée")
		}

		execID, err := commander.Execute(ctx, device.ID, in.Body.Command, in.Body.Parameters)
		if errors.Is(err, command.ErrUnsupported) {
			return nil, huma.Error422UnprocessableEntity(err.Error())
		}
		if err != nil {
			return nil, huma.Error502BadGateway("échec de l'envoi de la commande", err)
		}

		// Sans flux d'événements, la source ne remonterait le nouvel état
		// qu'au prochain tour de polling : le demander tout de suite.
		if d.Poller != nil {
			d.Poller.Nudge(device.Source)
		}

		out := &CommandOutput{}
		out.Body.ExecID = execID
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-rooms",
		Method:      http.MethodGet,
		Path:        "/api/rooms",
		Summary:     "Lister les pièces",
		Tags:        []string{"Devices"},
	}, func(ctx context.Context, _ *struct{}) (*ListRoomsOutput, error) {
		rooms, err := d.Store.ListRooms(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("lecture des pièces", err)
		}

		out := &ListRoomsOutput{}
		out.Body.Rooms = rooms
		return out, nil
	})
}
