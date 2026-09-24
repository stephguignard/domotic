package scenes

import (
	"errors"
	"strings"
	"testing"

	"github.com/stephguignard/domotic/internal/store"
)

func validSpec() store.SceneSpec {
	return store.SceneSpec{
		Name:      " Soirée ",
		Steps:     []store.SceneStep{action("on", nil, nil, []string{"Salon"})},
		Schedules: []store.SceneSchedule{{Enabled: true, Days: []int{5, 1}, At: "time", Time: "19:30"}},
	}
}

func TestValidateAcceptsAndNormalizes(t *testing.T) {
	spec := validSpec()
	if err := Validate(&spec, Place{}); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if spec.Name != "Soirée" || spec.Schedules[0].Days[0] != 1 {
		t.Errorf("normalisation : %+v", spec)
	}
}

func TestValidateRejects(t *testing.T) {
	cases := map[string]func(*store.SceneSpec){
		"nom vide":              func(s *store.SceneSpec) { s.Name = "  " },
		"sans action":           func(s *store.SceneSpec) { s.Steps = []store.SceneStep{{Type: "wait", WaitMinutes: 5}} },
		"sans cible":            func(s *store.SceneSpec) { s.Steps[0].Targets.Rooms = nil },
		"attente trop longue":   func(s *store.SceneSpec) { s.Steps = append(s.Steps, store.SceneStep{Type: "wait", WaitMinutes: 721}) },
		"luminosité hors plage": func(s *store.SceneSpec) { s.Steps[0] = action("setBrightness", []any{120.0}, []string{"x"}, nil) },
		"couleur mal formée":    func(s *store.SceneSpec) { s.Steps[0] = action("setColor", []any{"rouge"}, []string{"x"}, nil) },
		"paramètre superflu":    func(s *store.SceneSpec) { s.Steps[0] = action("on", []any{1.0}, []string{"x"}, nil) },
		"jour en double":        func(s *store.SceneSpec) { s.Schedules[0].Days = []int{1, 1} },
		"jour hors plage":       func(s *store.SceneSpec) { s.Schedules[0].Days = []int{8} },
		"heure manquante":       func(s *store.SceneSpec) { s.Schedules[0].Time = "" },
		"solaire sans coordonnées": func(s *store.SceneSpec) {
			s.Schedules[0] = store.SceneSchedule{Enabled: true, Days: []int{1}, At: "sunset"}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			spec := validSpec()
			mutate(&spec)
			err := Validate(&spec, Place{})
			if !errors.Is(err, ErrInvalid) {
				t.Errorf("attendu ErrInvalid, obtenu %v", err)
			}
		})
	}
}

func TestValidateSolarBounds(t *testing.T) {
	spec := validSpec()
	spec.Schedules[0] = store.SceneSchedule{Enabled: true, Days: []int{1}, At: "sunrise", NotBefore: "08:00", NotAfter: "07:00"}
	err := Validate(&spec, Place{HasCoordinates: true})
	if err == nil || !strings.Contains(err.Error(), "pas avant") {
		t.Errorf("bornes inversées : %v", err)
	}
}
