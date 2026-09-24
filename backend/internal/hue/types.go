package hue

import (
	"encoding/json"
	"strings"
)

// envelope est l'enveloppe commune des réponses CLIP v2.
type envelope struct {
	Errors []struct {
		Description string `json:"description"`
	} `json:"errors"`
	Data json.RawMessage `json:"data"`
}

func (e envelope) describeErrors() string {
	if len(e.Errors) == 0 {
		return "erreur inconnue"
	}
	msgs := make([]string, len(e.Errors))
	for i, err := range e.Errors {
		msgs[i] = err.Description
	}
	return strings.Join(msgs, "; ")
}

// ref désigne une autre ressource du pont.
type ref struct {
	RID   string `json:"rid"`
	RType string `json:"rtype"`
}

// light est une ressource « light ». Dans un événement « update », seuls les
// champs modifiés sont présents : d'où les pointeurs.
type light struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Owner    ref    `json:"owner"`
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	On *struct {
		On bool `json:"on"`
	} `json:"on"`
	Dimming *struct {
		Brightness float64 `json:"brightness"`
	} `json:"dimming"`
	Color *struct {
		XY struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		} `json:"xy"`
	} `json:"color"`
	// ColorTemperature est présent pour toute lampe d'ambiance ; Mirek est nul
	// quand la lampe est réglée sur une couleur plutôt que sur un blanc.
	ColorTemperature *struct {
		Mirek      *int `json:"mirek"`
		MirekValid bool `json:"mirek_valid"`
	} `json:"color_temperature"`
}

// state retourne les grandeurs connues de la lumière, au format de l'état
// unifié. Seules figurent celles que la lampe sait produire : une prise n'a
// pas de luminosité, une lampe blanche pas de couleur. Le frontend s'appuie
// sur cette présence pour choisir les réglages à proposer.
//
// color_temperature vaut nil quand une lampe d'ambiance est réglée sur une
// couleur : la clé reste présente, puisque la lampe sait produire des blancs.
func (l light) state() map[string]any {
	out := map[string]any{}
	if l.On != nil {
		out["on"] = l.On.On
	}
	if l.Dimming != nil {
		out["brightness"] = l.Dimming.Brightness
	}
	if l.Color != nil {
		out["color"] = xyToHex(l.Color.XY.X, l.Color.XY.Y)
	}
	if ct := l.ColorTemperature; ct != nil {
		if ct.MirekValid && ct.Mirek != nil && *ct.Mirek > 0 {
			out["color_temperature"] = mirekToKelvin(*ct.Mirek)
		} else {
			out["color_temperature"] = nil
		}
	}
	return out
}

// room est une ressource « room » : ses enfants sont des appareils.
type room struct {
	ID       string `json:"id"`
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Children []ref `json:"children"`
}

// zigbeeConnectivity suit la liaison Zigbee d'un appareil.
type zigbeeConnectivity struct {
	ID     string `json:"id"`
	Owner  ref    `json:"owner"`
	Status string `json:"status"` // connected, disconnected, connectivity_issue…
}
