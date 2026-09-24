package scenes

import (
	"encoding/json"

	"github.com/stephguignard/domotic/internal/store"
)

// hasStateKey indique si l'état remonté d'un équipement porte la grandeur.
func hasStateKey(d store.Device, key string) bool {
	var state map[string]any
	if json.Unmarshal([]byte(d.State), &state) != nil {
		return false
	}
	_, ok := state[key]
	return ok
}

// touchesRelays indique si une scène vise, au moment présent, au moins un
// relais : le lancer à la main demande alors confirmation.
func touchesRelays(sc store.Scene, devices []store.Device) bool {
	for _, step := range sc.Steps {
		targets, _ := Resolve(step, devices)
		for _, d := range targets {
			if d.Kind == "switch" {
				return true
			}
		}
	}
	return false
}
