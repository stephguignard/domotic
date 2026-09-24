package hue

import (
	"math"
	"testing"
)

// Primaires et blanc : les coordonnées attendues sont les sommets du gamut
// « Wide RGB D65 » et le point blanc D65, indépendants de cette implémentation.
func TestHexToXYReferencePoints(t *testing.T) {
	cases := []struct {
		hex  string
		x, y float64
	}{
		{"#ff0000", 0.7006, 0.2993},
		{"#00ff00", 0.1724, 0.7468},
		{"#0000ff", 0.1355, 0.0399},
		{"#ffffff", 0.3227, 0.3290},
	}
	for _, tc := range cases {
		x, y, err := hexToXY(tc.hex)
		if err != nil {
			t.Fatalf("%s: %v", tc.hex, err)
		}
		if math.Abs(x-tc.x) > 0.002 || math.Abs(y-tc.y) > 0.002 {
			t.Errorf("%s → (%.4f, %.4f), attendu (%.4f, %.4f)", tc.hex, x, y, tc.x, tc.y)
		}
	}
}

// Une couleur à pleine intensité (une composante à ff) doit survivre à
// l'aller-retour, à l'arrondi près : c'est ce que l'interface relit après un
// réglage. Une couleur plus sombre revient éclaircie, xy ne portant pas la
// luminosité.
func TestColorRoundTrip(t *testing.T) {
	for _, hex := range []string{"#ff0000", "#00ff00", "#0000ff", "#ff8000", "#8000ff", "#ffffff"} {
		x, y, err := hexToXY(hex)
		if err != nil {
			t.Fatalf("%s: %v", hex, err)
		}
		back := xyToHex(x, y)
		r1, g1, b1, _ := parseHex(hex)
		r2, g2, b2, _ := parseHex(back)
		if math.Abs(r1-r2) > 0.02 || math.Abs(g1-g2) > 0.02 || math.Abs(b1-b2) > 0.02 {
			t.Errorf("%s → (%.4f, %.4f) → %s", hex, x, y, back)
		}
	}
}

func TestHexToXYRejectsInvalid(t *testing.T) {
	for _, bad := range []string{"#000000", "ff0000", "#ff00", "#gg0000", ""} {
		if _, _, err := hexToXY(bad); err == nil {
			t.Errorf("%q aurait dû être refusé", bad)
		}
	}
}

func TestMirekConversions(t *testing.T) {
	if got := kelvinToMirek(2700); got != 370 {
		t.Errorf("2700 K = %d mired, attendu 370", got)
	}
	// Hors plage, la consigne est ramenée aux bornes des lampes.
	if kelvinToMirek(10000) != mirekMin || kelvinToMirek(1000) != mirekMax {
		t.Error("bornes non appliquées")
	}
	// Valeurs relevées sur un pont réel.
	if got := mirekToKelvin(447); got != 2240 {
		t.Errorf("447 mired = %d K, attendu 2240", got)
	}
	if got := mirekToKelvin(153); got != 6540 {
		t.Errorf("153 mired = %d K, attendu 6540", got)
	}
}

func TestLightStateReportsCapabilities(t *testing.T) {
	mirek := 447
	var warm light
	warm.On = &struct {
		On bool `json:"on"`
	}{true}
	warm.Color = &struct {
		XY struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		} `json:"xy"`
	}{}
	warm.Color.XY.X, warm.Color.XY.Y = 0.5018, 0.4152
	warm.ColorTemperature = &struct {
		Mirek      *int `json:"mirek"`
		MirekValid bool `json:"mirek_valid"`
	}{&mirek, true}

	s := warm.state()
	if s["color_temperature"] != 2240 {
		t.Errorf("température = %v", s["color_temperature"])
	}
	// Blanc chaud : rouge au maximum, bleu le plus faible.
	hex, _ := s["color"].(string)
	r, g, b, err := parseHex(hex)
	if err != nil || r != 1 || !(b < g) {
		t.Errorf("couleur d'un blanc chaud = %q", hex)
	}

	// Réglée sur une couleur, la lampe garde la clé, à nil.
	warm.ColorTemperature.MirekValid = false
	warm.ColorTemperature.Mirek = nil
	s = warm.state()
	if v, ok := s["color_temperature"]; !ok || v != nil {
		t.Errorf("température en mode couleur = %v (présente: %v), attendu nil présente", v, ok)
	}
}
