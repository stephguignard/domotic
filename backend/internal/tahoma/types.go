package tahoma

import (
	"encoding/json"
	"strings"
)

// Device est un équipement tel que décrit par l'API locale Overkiz.
type Device struct {
	DeviceURL        string  `json:"deviceURL"`
	Label            string  `json:"label"`
	ControllableName string  `json:"controllableName"`
	Available        bool    `json:"available"`
	Enabled          bool    `json:"enabled"`
	PlaceOID         string  `json:"placeOID"`
	States           []State `json:"states"`
	Definition       struct {
		Commands []struct {
			CommandName string `json:"commandName"`
			Nparams     int    `json:"nparams"`
		} `json:"commands"`
	} `json:"definition"`
}

// State est un état d'équipement. La valeur est laissée brute : selon l'état,
// Overkiz renvoie un nombre, une chaîne ou un booléen.
type State struct {
	Name  string          `json:"name"`
	Type  int             `json:"type"`
	Value json.RawMessage `json:"value"`
}

// Setup est la réponse de GET /setup, qui porte les équipements et les pièces.
type Setup struct {
	Devices   []Device `json:"devices"`
	RootPlace place    `json:"rootPlace"`
}

// place est un nœud de l'arborescence des pièces.
type place struct {
	OID       string  `json:"oid"`
	Label     string  `json:"label"`
	Type      int     `json:"type"`
	SubPlaces []place `json:"subPlaces"`
}

// flatten aplatit l'arborescence des pièces en une table OID → libellé.
func (p place) flatten(into map[string]string) {
	if p.OID != "" {
		into[p.OID] = p.Label
	}
	for _, sub := range p.SubPlaces {
		sub.flatten(into)
	}
}

// Command est une commande à exécuter sur un équipement.
type Command struct {
	Name       string `json:"name"`
	Parameters []any  `json:"parameters"`
}

// action regroupe les commandes destinées à un même équipement.
type action struct {
	DeviceURL string    `json:"deviceURL"`
	Commands  []Command `json:"commands"`
}

// applyRequest est le corps de POST /exec/apply.
type applyRequest struct {
	Label   string   `json:"label"`
	Actions []action `json:"actions"`
}

// execResponse porte l'identifiant d'exécution renvoyé par /exec/apply.
type execResponse struct {
	ExecID string `json:"execId"`
}

// listenerResponse porte l'identifiant de listener renvoyé par /events/register.
type listenerResponse struct {
	ID string `json:"id"`
}

// Event est un événement remonté par la box.
type Event struct {
	Name         string  `json:"name"`
	Timestamp    int64   `json:"timestamp"`
	DeviceURL    string  `json:"deviceURL"`
	DeviceStates []State `json:"deviceStates"`
	SetupOID     string  `json:"setupOID"`
	ExecID       string  `json:"execId"`
	NewState     string  `json:"newState"`
}

// statesToMap convertit une liste d'états en map exploitable, en décodant les
// valeurs brutes.
func statesToMap(states []State) map[string]any {
	out := make(map[string]any, len(states))
	for _, s := range states {
		if len(s.Value) == 0 {
			continue
		}
		var v any
		if err := json.Unmarshal(s.Value, &v); err != nil {
			// Valeur illisible : conserver la forme brute plutôt que de la perdre.
			v = string(s.Value)
		}
		out[s.Name] = v
	}
	return out
}

// isInfrastructure reconnaît les composants d'infrastructure de la passerelle :
// la box elle-même, ses interfaces réseau, et les ponts ou émetteurs-récepteurs
// par lesquels transitent les autres protocoles. La box les expose dans /setup
// au même titre que les équipements, mais ils ne se pilotent pas et n'ont rien
// à faire dans une interface domestique.
//
// Le critère porte sur le controllableName plutôt que sur le préfixe de
// protocole du deviceURL : filtrer tout « zigbee:// » écarterait aussi les
// vrais équipements Zigbee le jour où l'un sera appairé, alors que seul le
// coordinateur porte un Transceiver.
func isInfrastructure(controllableName string) bool {
	n := strings.ToLower(controllableName)

	// internal: ne contient que les composants propres à la passerelle
	// (PodV3Component, WifiComponent).
	if strings.HasPrefix(n, "internal:") {
		return true
	}

	return strings.HasSuffix(n, ":bridge") || strings.Contains(n, "transceiver")
}

// kindForControllable déduit un type d'équipement unifié depuis le
// controllableName Overkiz, de la forme "io:RollerShutterGenericIOComponent".
func kindForControllable(name string) string {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "rollershutter"), strings.Contains(n, "screen"),
		strings.Contains(n, "blind"), strings.Contains(n, "shutter"):
		return "shutter"
	case strings.Contains(n, "awning"):
		return "awning"
	case strings.Contains(n, "window"):
		return "window"
	case strings.Contains(n, "garage"), strings.Contains(n, "gate"):
		return "gate"
	case strings.Contains(n, "light"), strings.Contains(n, "onoff"):
		return "light"
	case strings.Contains(n, "temperature"), strings.Contains(n, "sensor"):
		return "sensor"
	case strings.Contains(n, "alarm"):
		return "alarm"
	case strings.Contains(n, "heating"), strings.Contains(n, "thermostat"):
		return "thermostat"
	case strings.Contains(n, "pod"), strings.Contains(n, "gateway"):
		return "gateway"
	default:
		return "unknown"
	}
}
