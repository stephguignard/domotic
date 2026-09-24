package scenes

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/stephguignard/domotic/internal/store"
)

// ErrInvalid signale une scène mal formée ; le message dit pourquoi.
var ErrInvalid = errors.New("scène invalide")

// maxWaitMinutes borne une attente : au-delà de 12 h, une scène n'est plus une
// suite d'actions mais une planification déguisée.
const maxWaitMinutes = 720

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Validate vérifie une scène avant enregistrement. Le schéma de l'API couvre
// déjà types et bornes simples ; restent les règles qui lient les champs.
func Validate(spec *store.SceneSpec, place Place) error {
	spec.Name = strings.TrimSpace(spec.Name)
	if spec.Name == "" {
		return invalid("le nom est vide")
	}

	actions := 0
	for i, step := range spec.Steps {
		n := i + 1
		switch step.Type {
		case "wait":
			if step.WaitMinutes < 1 || step.WaitMinutes > maxWaitMinutes {
				return invalid("étape %d : une attente dure de 1 à %d minutes", n, maxWaitMinutes)
			}
		case "action":
			actions++
			if err := validateAction(step); err != nil {
				return invalid("étape %d : %v", n, err)
			}
		default:
			return invalid("étape %d : type inconnu %q", n, step.Type)
		}
	}
	if actions == 0 {
		return invalid("une scène doit contenir au moins une action")
	}

	for i, s := range spec.Schedules {
		if err := validateSchedule(s, place); err != nil {
			return invalid("horaire %d : %v", i+1, err)
		}
	}
	return nil
}

func validateAction(step store.SceneStep) error {
	t := step.Targets
	if t == nil || len(t.Devices)+len(t.Rooms) == 0 {
		return errors.New("aucune cible : choisir des équipements ou des pièces")
	}
	p := step.Parameters

	switch step.Command {
	case "on", "off", "open", "close", "stop":
		if len(p) != 0 {
			return fmt.Errorf("%s ne prend pas de paramètre", step.Command)
		}
	case "setBrightness":
		if v, ok := number(p); !ok || v < 0 || v > 100 {
			return errors.New("la luminosité va de 0 à 100 %")
		}
	case "setColorTemperature":
		if v, ok := number(p); !ok || v < 2000 || v > 6500 {
			return errors.New("le blanc va de 2000 à 6500 K")
		}
	case "setColor":
		if len(p) != 1 {
			return errors.New("la couleur attend une valeur #rrggbb")
		}
		if s, ok := p[0].(string); !ok || !hexColor.MatchString(s) {
			return errors.New("la couleur attend une valeur #rrggbb")
		}
	default:
		return fmt.Errorf("action inconnue %q", step.Command)
	}
	return nil
}

func validateSchedule(s store.SceneSchedule, place Place) error {
	if len(s.Days) == 0 {
		return errors.New("aucun jour choisi")
	}
	seen := map[int]bool{}
	for _, d := range s.Days {
		if d < 1 || d > 7 || seen[d] {
			return errors.New("les jours vont de 1 (lundi) à 7 (dimanche), sans doublon")
		}
		seen[d] = true
	}
	slices.Sort(s.Days)

	switch s.At {
	case "time":
		if _, _, ok := parseClock(s.Time); !ok {
			return errors.New("heure attendue au format HH:MM")
		}
	case "sunrise", "sunset":
		if !place.HasCoordinates {
			return errors.New("horaire solaire impossible : renseigner DOMOTIC_LATITUDE et DOMOTIC_LONGITUDE")
		}
		if s.OffsetMinutes < -180 || s.OffsetMinutes > 180 {
			return errors.New("le décalage va de -180 à 180 minutes")
		}
		hb, mb, okB := parseClock(s.NotBefore)
		ha, ma, okA := parseClock(s.NotAfter)
		if (s.NotBefore != "" && !okB) || (s.NotAfter != "" && !okA) {
			return errors.New("bornes attendues au format HH:MM")
		}
		if okB && okA && hb*60+mb > ha*60+ma {
			return errors.New("la borne « pas avant » suit la borne « pas après »")
		}
	default:
		return fmt.Errorf("déclencheur inconnu %q", s.At)
	}
	return nil
}

// number extrait l'unique paramètre numérique d'une action.
func number(p []any) (float64, bool) {
	if len(p) != 1 {
		return 0, false
	}
	v, ok := p[0].(float64)
	return v, ok
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w : %s", ErrInvalid, fmt.Sprintf(format, args...))
}
