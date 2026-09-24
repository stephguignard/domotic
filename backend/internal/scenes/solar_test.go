package scenes

import (
	"testing"
	"time"
)

// Horaires de référence pour Lausanne (46.52 N, 6.63 E), en heure locale,
// tels que calculés par l'US Naval Observatory (aa.usno.navy.mil/api/rstt),
// arrondis à la minute. Tolérance : 2 minutes.
func TestSunTimesLausanne(t *testing.T) {
	zurich, err := time.LoadLocation("Europe/Zurich")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		date      time.Time
		rise, set string
	}{
		{time.Date(2026, 6, 21, 0, 0, 0, 0, zurich), "05:40", "21:30"},
		{time.Date(2026, 12, 21, 0, 0, 0, 0, zurich), "08:14", "16:49"},
		{time.Date(2026, 3, 20, 0, 0, 0, 0, zurich), "06:37", "18:46"},
	}
	for _, tc := range cases {
		rise, set, ok := sunTimes(tc.date.Year(), tc.date.Month(), tc.date.Day(), 46.52, 6.63)
		if !ok {
			t.Fatalf("%s : pas de lever calculé", tc.date.Format("2006-01-02"))
		}
		checkNear(t, tc.date, "lever", rise.In(zurich), tc.rise)
		checkNear(t, tc.date, "coucher", set.In(zurich), tc.set)
	}
}

func checkNear(t *testing.T, date time.Time, what string, got time.Time, want string) {
	t.Helper()
	w, _ := time.ParseInLocation("15:04", want, got.Location())
	wantAt := time.Date(got.Year(), got.Month(), got.Day(), w.Hour(), w.Minute(), 0, 0, got.Location())
	if d := got.Sub(wantAt); d < -2*time.Minute || d > 2*time.Minute {
		t.Errorf("%s %s = %s, attendu %s (écart %s)", date.Format("2006-01-02"), what, got.Format("15:04"), want, d)
	}
}

func TestSunTimesPolarNight(t *testing.T) {
	// Tromsø (69.65 N) le 21 décembre : nuit polaire, aucun lever.
	if _, _, ok := sunTimes(2026, 12, 21, 69.65, 18.96); ok {
		t.Error("attendu l'absence de lever pendant la nuit polaire")
	}
}
