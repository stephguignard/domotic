package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/stephguignard/domotic/internal/scenes"
	"github.com/stephguignard/domotic/internal/store"
)

// ListScenesOutput est la réponse de la liste des scènes.
type ListScenesOutput struct {
	Body struct {
		Scenes []scenes.SceneView `json:"scenes" nullable:"false" doc:"Scènes, dans l'ordre d'affichage"`
		// Ces deux champs permettent à l'interface de ne proposer que les
		// horaires que le service sait calculer.
		SolarAvailable bool   `json:"solar_available" doc:"Les horaires solaires sont-ils disponibles (coordonnées configurées) ?"`
		TimeZone       string `json:"time_zone" doc:"Fuseau dans lequel les heures des horaires s'entendent"`
	}
}

// SceneIDInput identifie une scène.
type SceneIDInput struct {
	ID int64 `path:"id" doc:"Identifiant de la scène"`
}

// SceneInput porte la description d'une scène à créer.
type SceneInput struct {
	Body store.SceneSpec
}

// UpdateSceneInput porte la description d'une scène à modifier.
type UpdateSceneInput struct {
	ID   int64 `path:"id" doc:"Identifiant de la scène"`
	Body store.SceneSpec
}

// SceneOutput renvoie une scène.
type SceneOutput struct {
	Body scenes.SceneView
}

// SceneOrderBody porte l'ordre d'affichage des scènes.
type SceneOrderBody struct {
	IDs []int64 `json:"ids" nullable:"false" maxItems:"200" doc:"Scènes, dans l'ordre d'affichage ; vide pour revenir à l'ordre alphabétique"`
}

// SceneOrderInput porte le nouvel ordre des scènes.
type SceneOrderInput struct {
	Body SceneOrderBody
}

// ResolveInput porte des étapes dont on veut connaître les cibles.
type ResolveInput struct {
	Body struct {
		Steps []store.SceneStep `json:"steps" nullable:"false" maxItems:"50" doc:"Étapes à résoudre"`
	}
}

// ResolvedDevice est un équipement qu'une action toucherait.
type ResolvedDevice struct {
	ID   string `json:"id" doc:"Identifiant de l'équipement"`
	Name string `json:"name" doc:"Nom de l'équipement"`
	Room string `json:"room" doc:"Pièce de l'équipement"`
	Kind string `json:"kind" doc:"Type de l'équipement"`
}

// ResolvedStep est le résultat de la résolution d'une étape.
type ResolvedStep struct {
	Targets []ResolvedDevice `json:"targets" nullable:"false" doc:"Équipements qui recevraient l'action"`
	Ignored []scenes.Ignored `json:"ignored" nullable:"false" doc:"Équipements visés mais écartés, avec le motif"`
}

// ResolveOutput est la réponse de la résolution.
type ResolveOutput struct {
	Body struct {
		Steps []ResolvedStep `json:"steps" nullable:"false" doc:"Résolution de chaque étape, dans l'ordre ; vide pour une attente"`
	}
}

func registerScenes(api huma.API, d Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "list-scenes",
		Method:      http.MethodGet,
		Path:        "/api/scenes",
		Summary:     "Lister les scènes",
		Tags:        []string{"Scenes"},
	}, func(ctx context.Context, _ *struct{}) (*ListScenesOutput, error) {
		return d.listScenes(ctx)
	})

	huma.Register(api, huma.Operation{
		OperationID: "set-scene-order",
		Method:      http.MethodPut,
		Path:        "/api/scenes/order",
		Summary:     "Choisir l'ordre des scènes",
		Description: "Range les scènes dans l'ordre donné, sur la page Scènes comme sur le tableau de bord. " +
			"Les scènes absentes de la liste passent après, par nom ; une liste vide revient à l'ordre alphabétique.",
		Tags: []string{"Scenes"},
	}, func(ctx context.Context, in *SceneOrderInput) (*ListScenesOutput, error) {
		seen := map[int64]bool{}
		for _, id := range in.Body.IDs {
			if seen[id] {
				return nil, huma.Error422UnprocessableEntity("scène en double dans l'ordre")
			}
			seen[id] = true
		}
		if err := d.Store.SetSceneOrder(ctx, in.Body.IDs); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, huma.Error422UnprocessableEntity("scène inconnue dans l'ordre")
			}
			return nil, huma.Error500InternalServerError("enregistrement de l'ordre des scènes", err)
		}
		return d.listScenes(ctx)
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-scene",
		Method:      http.MethodGet,
		Path:        "/api/scenes/{id}",
		Summary:     "Détail d'une scène",
		Tags:        []string{"Scenes"},
	}, func(ctx context.Context, in *SceneIDInput) (*SceneOutput, error) {
		return d.sceneView(ctx, in.ID)
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-scene",
		Method:        http.MethodPost,
		Path:          "/api/scenes",
		Summary:       "Créer une scène",
		Tags:          []string{"Scenes"},
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *SceneInput) (*SceneOutput, error) {
		if err := scenes.Validate(&in.Body, d.Scenes.Place()); err != nil {
			return nil, huma.Error422UnprocessableEntity(err.Error())
		}
		sc, err := d.Store.CreateScene(ctx, in.Body)
		if err != nil {
			return nil, huma.Error500InternalServerError("création de la scène", err)
		}
		d.Scenes.Reload()
		return d.sceneView(ctx, sc.ID)
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-scene",
		Method:      http.MethodPut,
		Path:        "/api/scenes/{id}",
		Summary:     "Modifier une scène",
		Description: "Remplace les étapes et les horaires. Une exécution en cours se poursuit avec l'ancienne version.",
		Tags:        []string{"Scenes"},
	}, func(ctx context.Context, in *UpdateSceneInput) (*SceneOutput, error) {
		if err := scenes.Validate(&in.Body, d.Scenes.Place()); err != nil {
			return nil, huma.Error422UnprocessableEntity(err.Error())
		}
		if _, err := d.Store.UpdateScene(ctx, in.ID, in.Body); err != nil {
			return nil, sceneError(err, "modification de la scène")
		}
		d.Scenes.Reload()
		return d.sceneView(ctx, in.ID)
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-scene",
		Method:        http.MethodDelete,
		Path:          "/api/scenes/{id}",
		Summary:       "Supprimer une scène",
		Description:   "Supprime la scène et ses horaires. Son historique reste, sous son nom d'alors.",
		Tags:          []string{"Scenes"},
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *SceneIDInput) (*struct{}, error) {
		if err := d.Store.DeleteScene(ctx, in.ID); err != nil {
			return nil, sceneError(err, "suppression de la scène")
		}
		d.Scenes.Reload()
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "run-scene",
		Method:      http.MethodPost,
		Path:        "/api/scenes/{id}/run",
		Summary:     "Lancer une scène",
		Description: "Lance la scène tout de suite. Une exécution en cours est annulée et la scène repart du début. " +
			"La réponse part dès le lancement : l'issue se lit ensuite sur la scène et dans l'historique.",
		Tags:          []string{"Scenes"},
		DefaultStatus: http.StatusAccepted,
	}, func(ctx context.Context, in *SceneIDInput) (*SceneOutput, error) {
		if err := d.Scenes.Trigger(ctx, in.ID, scenes.TriggerManual); err != nil {
			if errors.Is(err, scenes.ErrNotStarted) {
				return nil, huma.Error503ServiceUnavailable(err.Error())
			}
			return nil, sceneError(err, "lancement de la scène")
		}
		return d.sceneView(ctx, in.ID)
	})

	huma.Register(api, huma.Operation{
		OperationID: "resolve-scene-steps",
		Method:      http.MethodPost,
		Path:        "/api/scenes/resolve",
		Summary:     "Aperçu des cibles",
		Description: "Indique, pour chaque étape, les équipements que l'action toucherait maintenant et ceux " +
			"qu'elle écarterait. Rien n'est envoyé aux équipements.",
		Tags: []string{"Scenes"},
	}, func(ctx context.Context, in *ResolveInput) (*ResolveOutput, error) {
		devices, err := d.Store.ListDevices(ctx, store.DeviceFilter{})
		if err != nil {
			return nil, huma.Error500InternalServerError("lecture des équipements", err)
		}
		out := &ResolveOutput{}
		out.Body.Steps = make([]ResolvedStep, 0, len(in.Body.Steps))
		for _, step := range in.Body.Steps {
			targets, ignored := scenes.Resolve(step, devices)
			rs := ResolvedStep{Targets: make([]ResolvedDevice, 0, len(targets)), Ignored: ignored}
			for _, t := range targets {
				rs.Targets = append(rs.Targets, ResolvedDevice{ID: t.ID, Name: t.Name, Room: t.Room, Kind: t.Kind})
			}
			out.Body.Steps = append(out.Body.Steps, rs)
		}
		return out, nil
	})
}

func (d Deps) listScenes(ctx context.Context) (*ListScenesOutput, error) {
	list, err := d.Store.ListScenes(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("lecture des scènes", err)
	}
	devices, err := d.Store.ListDevices(ctx, store.DeviceFilter{})
	if err != nil {
		return nil, huma.Error500InternalServerError("lecture des équipements", err)
	}

	out := &ListScenesOutput{}
	out.Body.Scenes = make([]scenes.SceneView, 0, len(list))
	for _, sc := range list {
		out.Body.Scenes = append(out.Body.Scenes, d.Scenes.Describe(sc, devices))
	}
	place := d.Scenes.Place()
	out.Body.SolarAvailable = place.HasCoordinates
	out.Body.TimeZone = place.TimeZone.String()
	return out, nil
}

func (d Deps) sceneView(ctx context.Context, id int64) (*SceneOutput, error) {
	sc, err := d.Store.GetScene(ctx, id)
	if err != nil {
		return nil, sceneError(err, "lecture de la scène")
	}
	devices, err := d.Store.ListDevices(ctx, store.DeviceFilter{})
	if err != nil {
		return nil, huma.Error500InternalServerError("lecture des équipements", err)
	}
	return &SceneOutput{Body: d.Scenes.Describe(sc, devices)}, nil
}

func sceneError(err error, what string) error {
	if errors.Is(err, store.ErrNotFound) {
		return huma.Error404NotFound("scène introuvable")
	}
	return huma.Error500InternalServerError(what, err)
}
