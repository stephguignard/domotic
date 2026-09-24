package scenes

import (
	"testing"
	"time"

	"github.com/stephguignard/domotic/internal/store"
)

func zurich(t *testing.T) Place {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Zurich")
	if err != nil {
		t.Fatal(err)
	}
	return Place{TimeZone: loc, Latitude: 46.52, Longitude: 6.63, HasCoordinates: true}
}

func at(p Place, s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, p.TimeZone)
	if err != nil {
		panic(err)
	}
	return t
}

func TestNextRespectsDays(t *testing.T) {
	p := zurich(t)
	weekdays := store.SceneSchedule{Enabled: true, Days: []int{1, 2, 3, 4, 5}, At: "time", Time: "07:30"}

	// Vendredi 25 septembre 2026, 08:00 : prochain réveil le lundi.
	next, ok := p.Next(weekdays, at(p, "2026-09-25 08:00"))
	if !ok || !next.Equal(at(p, "2026-09-28 07:30")) {
		t.Errorf("prochain = %v", next)
	}
	// Juste avant l'heure, le jour même.
	next, _ = p.Next(weekdays, at(p, "2026-09-25 07:29"))
	if !next.Equal(at(p, "2026-09-25 07:30")) {
		t.Errorf("prochain = %v", next)
	}
	// Strictement postérieur : l'échéance en cours n'est pas rendue.
	next, _ = p.Next(weekdays, at(p, "2026-09-25 07:30"))
	if !next.Equal(at(p, "2026-09-28 07:30")) {
		t.Errorf("prochain = %v", next)
	}
}

func TestNextAcrossDaylightSavingChanges(t *testing.T) {
	p := zurich(t)
	every := []int{1, 2, 3, 4, 5, 6, 7}

	// 28 mars 2027 : 02:30 n'existe pas, l'horaire part tout de même ce jour-là.
	gap := store.SceneSchedule{Enabled: true, Days: every, At: "time", Time: "02:30"}
	next, _ := p.Next(gap, at(p, "2027-03-28 00:00"))
	if next.Day() != 28 || next.In(p.TimeZone).Hour() != 3 {
		t.Errorf("heure inexistante : %v", next.In(p.TimeZone))
	}

	// 25 octobre 2026 : 02:30 existe deux fois, l'horaire ne part qu'une fois.
	first, _ := p.Next(gap, at(p, "2026-10-25 00:00"))
	second, _ := p.Next(gap, first)
	if second.Sub(first) < 23*time.Hour {
		t.Errorf("deux déclenchements le même jour : %v puis %v", first, second)
	}
}

func TestSolarScheduleWithOffsetAndBounds(t *testing.T) {
	p := zurich(t)
	day := at(p, "2026-06-21 00:00")

	sunset := store.SceneSchedule{Enabled: true, Days: []int{7}, At: "sunset", OffsetMinutes: -15}
	next, ok := p.Next(sunset, day)
	// Coucher à 21:30 selon l'USNO, moins 15 minutes.
	if !ok || next.In(p.TimeZone).Format("15:04") < "21:13" || next.In(p.TimeZone).Format("15:04") > "21:17" {
		t.Errorf("coucher -15 = %v", next.In(p.TimeZone))
	}

	// Lever à 05:40 : borné à 07:00 au plus tôt.
	sunrise := store.SceneSchedule{Enabled: true, Days: []int{7}, At: "sunrise", NotBefore: "07:00"}
	next, _ = p.Next(sunrise, day)
	if got := next.In(p.TimeZone).Format("15:04"); got != "07:00" {
		t.Errorf("lever borné = %s", got)
	}

	// Sans coordonnées, pas d'horaire solaire.
	p.HasCoordinates = false
	if _, ok := p.Next(sunset, day); ok {
		t.Error("horaire solaire sans coordonnées")
	}
}

func TestShouldCatchUp(t *testing.T) {
	p := zurich(t)
	sc := store.Scene{SceneSpec: store.SceneSpec{Schedules: []store.SceneSchedule{
		{Enabled: true, Days: []int{1, 2, 3, 4, 5, 6, 7}, At: "time", Time: "07:30"},
	}}}

	if _, ok := shouldCatchUp(p, sc, at(p, "2026-09-25 07:33")); !ok {
		t.Error("3 minutes de retard : attendu un rattrapage")
	}
	if _, ok := shouldCatchUp(p, sc, at(p, "2026-09-25 07:40")); ok {
		t.Error("10 minutes de retard : attendu aucun rattrapage")
	}

	ran := at(p, "2026-09-25 07:30").Add(time.Second)
	sc.LastRunAt = &ran
	if _, ok := shouldCatchUp(p, sc, at(p, "2026-09-25 07:33")); ok {
		t.Error("échéance déjà exécutée : attendu aucun rattrapage")
	}

	sc.LastRunAt = nil
	sc.Schedules[0].Enabled = false
	if _, ok := shouldCatchUp(p, sc, at(p, "2026-09-25 07:33")); ok {
		t.Error("horaire inactif : attendu aucun rattrapage")
	}
}
