package hue

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Conversions entre couleurs sRGB et coordonnées CIE xy, selon la méthode
// publiée par Philips (« RGB to xy Color conversion ») : correction gamma sRGB,
// puis matrice « Wide RGB D65 ». La luminosité est portée à part par dimming :
// xy ne décrit que la teinte et la saturation.
//
// Les lampes ne reproduisent qu'une partie de l'espace xy (leur gamut) ; le
// pont ramène de lui-même une consigne hors gamut à la couleur la plus proche.

// hexToXY convertit une couleur « #rrggbb » en coordonnées xy.
func hexToXY(hex string) (x, y float64, err error) {
	r, g, b, err := parseHex(hex)
	if err != nil {
		return 0, 0, err
	}
	r, g, b = linearize(r), linearize(g), linearize(b)

	X := r*0.664511 + g*0.154324 + b*0.162028
	Y := r*0.283881 + g*0.668433 + b*0.047685
	Z := r*0.000088 + g*0.072310 + b*0.986039

	sum := X + Y + Z
	if sum == 0 {
		// Le noir n'a pas de teinte : c'est une extinction, pas une couleur.
		return 0, 0, fmt.Errorf("le noir n'est pas une couleur de lampe")
	}
	return round4(X / sum), round4(Y / sum), nil
}

// xyToHex convertit des coordonnées xy en couleur « #rrggbb », à pleine
// luminosité : la composante la plus forte est portée au maximum.
func xyToHex(x, y float64) string {
	if y <= 0 {
		return "#ffffff"
	}
	const Y = 1.0
	X := Y / y * x
	Z := Y / y * (1 - x - y)

	r := X*1.656492 - Y*0.354851 - Z*0.255038
	g := -X*0.707196 + Y*1.655397 + Z*0.036152
	b := X*0.051713 - Y*0.121364 + Z*1.011530

	// Hors de l'espace sRGB, une composante peut devenir négative : la couleur
	// affichée n'est alors qu'une approximation, ce qui suffit à l'interface.
	r, g, b = max(r, 0), max(g, 0), max(b, 0)
	if m := max(r, g, b); m > 0 {
		r, g, b = r/m, g/m, b/m
	}
	return fmt.Sprintf("#%02x%02x%02x", to8bit(compress(r)), to8bit(compress(g)), to8bit(compress(b)))
}

func parseHex(hex string) (r, g, b float64, err error) {
	s, ok := strings.CutPrefix(hex, "#")
	if !ok || len(s) != 6 {
		return 0, 0, 0, fmt.Errorf("couleur invalide %q, attendu #rrggbb", hex)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("couleur invalide %q, attendu #rrggbb", hex)
	}
	return float64(v>>16&0xff) / 255, float64(v>>8&0xff) / 255, float64(v&0xff) / 255, nil
}

// linearize retire la correction gamma sRGB.
func linearize(v float64) float64 {
	if v > 0.04045 {
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return v / 12.92
}

// compress applique la correction gamma sRGB.
func compress(v float64) float64 {
	if v <= 0.0031308 {
		return 12.92 * v
	}
	return 1.055*math.Pow(v, 1/2.4) - 0.055
}

func to8bit(v float64) int {
	return int(math.Round(math.Min(math.Max(v, 0), 1) * 255))
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

// Bornes des températures de couleur, en mired, communes aux lampes Hue
// d'ambiance : 153 mired ≈ 6500 K (lumière froide), 500 mired = 2000 K (chaude).
const (
	mirekMin = 153
	mirekMax = 500
)

// kelvinToMirek convertit une température en mired, bornée à la plage des
// lampes Hue.
func kelvinToMirek(kelvin float64) int {
	return min(max(int(math.Round(1e6/kelvin)), mirekMin), mirekMax)
}

// mirekToKelvin convertit des mired en kelvins, arrondis à la dizaine.
func mirekToKelvin(mirek int) int {
	return int(math.Round(1e6/float64(mirek)/10)) * 10
}
