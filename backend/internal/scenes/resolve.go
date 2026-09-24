package scenes

import (
	"slices"

	"github.com/stephguignard/domotic/internal/control"
	"github.com/stephguignard/domotic/internal/store"
)

// Ignored est un équipement visé par une action mais qui ne la recevra pas.
type Ignored struct {
	DeviceID string `json:"device_id" doc:"Équipement écarté"`
	Name     string `json:"name" doc:"Nom de l'équipement, ou son identifiant s'il a disparu"`
	Reason   string `json:"reason" doc:"Motif de l'écart"`
}

// kindsFor liste les types d'équipement qui acceptent une action.
var kindsFor = map[string][]string{
	"on":    {"light", "switch"},
	"off":   {"light", "switch"},
	"open":  {"shutter", "awning", "window", "gate"},
	"close": {"shutter", "awning", "window", "gate"},
	"stop":  {"shutter", "awning", "window"},
}

// stateKeyFor désigne, pour les réglages de lumière, la grandeur que la lampe
// doit remonter pour savoir faire ce réglage : une prise Hue n'a pas de
// luminosité, une lampe blanche pas de couleur.
var stateKeyFor = map[string]string{
	"setBrightness":       "brightness",
	"setColor":            "color",
	"setColorTemperature": "color_temperature",
}

// Resolve détermine les équipements qu'une action touchera, parmi devices :
// ceux désignés nommément et ceux des pièces visées, dans leur état présent.
// Les autres sont rendus avec le motif de leur écart. L'ordre est celui des
// cibles, puis des équipements de chaque pièce.
func Resolve(step store.SceneStep, devices []store.Device) (targets []store.Device, ignored []Ignored) {
	targets, ignored = []store.Device{}, []Ignored{}
	if step.Type != "action" || step.Targets == nil {
		return targets, ignored
	}
	byID := make(map[string]store.Device, len(devices))
	for _, d := range devices {
		byID[d.ID] = d
	}

	seen := map[string]bool{}
	consider := func(d store.Device) {
		if seen[d.ID] {
			return
		}
		seen[d.ID] = true
		if reason := incompatibility(step, d); reason != "" {
			ignored = append(ignored, Ignored{DeviceID: d.ID, Name: d.Name, Reason: reason})
			return
		}
		targets = append(targets, d)
	}

	for _, id := range step.Targets.Devices {
		d, ok := byID[id]
		if !ok {
			if !seen[id] {
				seen[id] = true
				ignored = append(ignored, Ignored{DeviceID: id, Name: id, Reason: "équipement introuvable"})
			}
			continue
		}
		consider(d)
	}
	for _, room := range step.Targets.Rooms {
		for _, d := range devices {
			if d.Room == room {
				// Un équipement d'une pièce hors du filtre de type n'est pas
				// « écarté » : il n'était simplement pas visé.
				if len(step.Targets.Kinds) > 0 && !slices.Contains(step.Targets.Kinds, d.Kind) {
					continue
				}
				consider(d)
			}
		}
	}
	return targets, ignored
}

// incompatibility retourne pourquoi un équipement ne peut pas recevoir
// l'action, ou une chaîne vide.
func incompatibility(step store.SceneStep, d store.Device) string {
	if !control.Controllable(d.Source) {
		return "équipement en lecture seule"
	}
	if kinds, ok := kindsFor[step.Command]; ok {
		if !slices.Contains(kinds, d.Kind) {
			return "type incompatible avec l'action"
		}
		return ""
	}
	if key, ok := stateKeyFor[step.Command]; ok {
		if d.Kind != "light" || !hasStateKey(d, key) {
			return "réglage que l'équipement ne sait pas faire"
		}
		return ""
	}
	return "action inconnue"
}
