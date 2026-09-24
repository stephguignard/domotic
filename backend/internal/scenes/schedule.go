package scenes

import (
	"slices"
	"time"

	"github.com/stephguignard/domotic/internal/store"
)

// Place situe la maison : fuseau des heures, coordonnées du soleil.
type Place struct {
	TimeZone       *time.Location
	Latitude       float64
	Longitude      float64
	HasCoordinates bool
}

// occurrence retourne l'instant où l'horaire se déclenche le jour civil donné,
// ou faux s'il ne se déclenche pas ce jour-là.
//
// Changements d'heure : une heure fixe qui n'existe pas (02:30 le dernier
// dimanche de mars) est décalée par time.Date à l'heure suivante ; une heure
// qui existe deux fois (02:30 fin octobre) n'est produite qu'une fois.
func (p Place) occurrence(s store.SceneSchedule, year int, month time.Month, day int) (time.Time, bool) {
	date := time.Date(year, month, day, 12, 0, 0, 0, p.TimeZone)
	if !slices.Contains(s.Days, isoWeekday(date)) {
		return time.Time{}, false
	}

	switch s.At {
	case "time":
		h, m, ok := parseClock(s.Time)
		if !ok {
			return time.Time{}, false
		}
		return time.Date(year, month, day, h, m, 0, 0, p.TimeZone), true

	case "sunrise", "sunset":
		if !p.HasCoordinates {
			return time.Time{}, false
		}
		rise, set, ok := sunTimes(year, month, day, p.Latitude, p.Longitude)
		if !ok {
			return time.Time{}, false
		}
		t := rise
		if s.At == "sunset" {
			t = set
		}
		t = t.Add(time.Duration(s.OffsetMinutes) * time.Minute).In(p.TimeZone)

		if h, m, ok := parseClock(s.NotBefore); ok {
			if bound := time.Date(year, month, day, h, m, 0, 0, p.TimeZone); t.Before(bound) {
				t = bound
			}
		}
		if h, m, ok := parseClock(s.NotAfter); ok {
			if bound := time.Date(year, month, day, h, m, 0, 0, p.TimeZone); t.After(bound) {
				t = bound
			}
		}
		// Arrondi à la minute : un horaire se lit, et se déclenche, à la minute.
		return t.Truncate(time.Minute), true
	}
	return time.Time{}, false
}

// Next retourne le prochain déclenchement strictement postérieur à after.
func (p Place) Next(s store.SceneSchedule, after time.Time) (time.Time, bool) {
	local := after.In(p.TimeZone)
	for i := 0; i <= 8; i++ {
		d := local.AddDate(0, 0, i)
		if t, ok := p.occurrence(s, d.Year(), d.Month(), d.Day()); ok && t.After(after) {
			return t, true
		}
	}
	return time.Time{}, false
}

// Prev retourne le dernier déclenchement antérieur ou égal à at.
func (p Place) Prev(s store.SceneSchedule, at time.Time) (time.Time, bool) {
	local := at.In(p.TimeZone)
	for i := 0; i <= 8; i++ {
		d := local.AddDate(0, 0, -i)
		if t, ok := p.occurrence(s, d.Year(), d.Month(), d.Day()); ok && !t.After(at) {
			return t, true
		}
	}
	return time.Time{}, false
}

// NextRun retourne le prochain déclenchement d'une scène, tous horaires
// actifs confondus.
func (p Place) NextRun(sc store.Scene, after time.Time) (time.Time, bool) {
	var best time.Time
	for _, s := range sc.Schedules {
		if !s.Enabled {
			continue
		}
		if t, ok := p.Next(s, after); ok && (best.IsZero() || t.Before(best)) {
			best = t
		}
	}
	return best, !best.IsZero()
}

// lastDue retourne le dernier déclenchement passé d'une scène, tous horaires
// actifs confondus.
func (p Place) lastDue(sc store.Scene, at time.Time) (time.Time, bool) {
	var best time.Time
	for _, s := range sc.Schedules {
		if !s.Enabled {
			continue
		}
		if t, ok := p.Prev(s, at); ok && t.After(best) {
			best = t
		}
	}
	return best, !best.IsZero()
}

// isoWeekday numérote les jours de 1 (lundi) à 7 (dimanche).
func isoWeekday(t time.Time) int {
	if wd := t.Weekday(); wd != time.Sunday {
		return int(wd)
	}
	return 7
}

// parseClock lit une heure « HH:MM ».
func parseClock(s string) (h, m int, ok bool) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, 0, false
	}
	return t.Hour(), t.Minute(), true
}
