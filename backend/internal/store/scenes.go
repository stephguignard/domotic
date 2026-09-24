package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// SceneTargets désigne les équipements visés par une action : des équipements
// nommés, des pièces — résolues au lancement —, et un filtre de type facultatif.
type SceneTargets struct {
	Devices []string `json:"devices" nullable:"false" maxItems:"100" doc:"Équipements visés, par identifiant"`
	Rooms   []string `json:"rooms" nullable:"false" maxItems:"50" doc:"Pièces visées : leurs équipements au moment du lancement"`
	Kinds   []string `json:"kinds" nullable:"false" maxItems:"20" doc:"Filtre facultatif : ne toucher que ces types, ex. light"`
}

// SceneStep est une étape de scène : une action sur des cibles, ou une attente.
type SceneStep struct {
	Type        string        `json:"type" enum:"action,wait" doc:"Nature de l'étape"`
	Command     string        `json:"command,omitempty" enum:"on,off,open,close,stop,setBrightness,setColor,setColorTemperature" doc:"Action, pour une étape action"`
	Parameters  []any         `json:"parameters,omitempty" doc:"Paramètres de l'action, ex. [40] pour setBrightness"`
	Targets     *SceneTargets `json:"targets,omitempty" doc:"Cibles, pour une étape action"`
	WaitMinutes int           `json:"wait_minutes,omitempty" minimum:"0" maximum:"720" doc:"Durée, pour une étape attente"`
}

// SceneSchedule est un horaire de déclenchement d'une scène.
type SceneSchedule struct {
	Enabled bool   `json:"enabled" doc:"L'horaire est-il actif ?"`
	Days    []int  `json:"days" nullable:"false" minItems:"1" maxItems:"7" doc:"Jours, de 1 (lundi) à 7 (dimanche)"`
	At      string `json:"at" enum:"time,sunrise,sunset" doc:"Heure fixe, lever ou coucher du soleil"`
	Time    string `json:"time,omitempty" pattern:"^([01][0-9]|2[0-3]):[0-5][0-9]$" doc:"Heure locale HH:MM, pour at=time"`
	// Décalage et bornes ne s'appliquent qu'aux horaires solaires.
	OffsetMinutes int    `json:"offset_minutes,omitempty" minimum:"-180" maximum:"180" doc:"Décalage par rapport au soleil, en minutes"`
	NotBefore     string `json:"not_before,omitempty" pattern:"^([01][0-9]|2[0-3]):[0-5][0-9]$" doc:"Pas avant cette heure locale"`
	NotAfter      string `json:"not_after,omitempty" pattern:"^([01][0-9]|2[0-3]):[0-5][0-9]$" doc:"Pas après cette heure locale"`
}

// SceneSpec est ce que l'utilisateur décrit d'une scène.
type SceneSpec struct {
	Name            string          `json:"name" minLength:"1" maxLength:"64" doc:"Nom de la scène"`
	ShowOnDashboard bool            `json:"show_on_dashboard" doc:"Afficher un bouton sur le tableau de bord ?"`
	Steps           []SceneStep     `json:"steps" nullable:"false" minItems:"1" maxItems:"50" doc:"Étapes, exécutées dans l'ordre"`
	Schedules       []SceneSchedule `json:"schedules" nullable:"false" maxItems:"20" doc:"Horaires de déclenchement"`
}

// Scene est une scène enregistrée, avec l'issue de sa dernière exécution.
type Scene struct {
	ID int64 `json:"id" doc:"Identifiant de la scène"`
	SceneSpec
	LastRunAt   *time.Time `json:"last_run_at,omitempty" doc:"Début de la dernière exécution"`
	LastTrigger string     `json:"last_trigger,omitempty" enum:"manual,schedule" doc:"Déclencheur de la dernière exécution"`
	LastStatus  string     `json:"last_status,omitempty" enum:"running,success,partial,failed,interrupted" doc:"Issue de la dernière exécution"`
	CreatedAt   time.Time  `json:"created_at" doc:"Date de création"`
	UpdatedAt   time.Time  `json:"updated_at" doc:"Date de dernière modification"`
}

// Issues d'une exécution de scène.
const (
	SceneRunning     = "running"
	SceneSuccess     = "success"
	ScenePartial     = "partial"
	SceneFailed      = "failed"
	SceneInterrupted = "interrupted"
)

const sceneColumns = `id, name, show_on_dashboard, steps, schedules, last_run_at, last_trigger, last_status,
	created_at, updated_at`

// ListScenes retourne les scènes, par nom.
func (s *Store) ListScenes(ctx context.Context) ([]Scene, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+sceneColumns+` FROM scene ORDER BY name COLLATE NOCASE, id`)
	if err != nil {
		return nil, fmt.Errorf("liste des scènes: %w", err)
	}
	defer rows.Close()

	scenes := []Scene{}
	for rows.Next() {
		sc, err := scanScene(rows)
		if err != nil {
			return nil, err
		}
		scenes = append(scenes, sc)
	}
	return scenes, rows.Err()
}

// GetScene retourne une scène par son identifiant.
func (s *Store) GetScene(ctx context.Context, id int64) (Scene, error) {
	sc, err := scanScene(s.db.QueryRowContext(ctx, `SELECT `+sceneColumns+` FROM scene WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Scene{}, ErrNotFound
	}
	return sc, err
}

// CreateScene enregistre une nouvelle scène.
func (s *Store) CreateScene(ctx context.Context, spec SceneSpec) (Scene, error) {
	steps, schedules, err := encodeSpec(spec)
	if err != nil {
		return Scene{}, err
	}
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO scene (name, show_on_dashboard, steps, schedules, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		spec.Name, spec.ShowOnDashboard, steps, schedules, now, now)
	if err != nil {
		return Scene{}, fmt.Errorf("création de la scène: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Scene{}, fmt.Errorf("création de la scène: %w", err)
	}
	return s.GetScene(ctx, id)
}

// UpdateScene remplace la description d'une scène, sans toucher à l'issue de
// sa dernière exécution.
func (s *Store) UpdateScene(ctx context.Context, id int64, spec SceneSpec) (Scene, error) {
	steps, schedules, err := encodeSpec(spec)
	if err != nil {
		return Scene{}, err
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE scene SET name = ?, show_on_dashboard = ?, steps = ?, schedules = ?, updated_at = ?
		WHERE id = ?`,
		spec.Name, spec.ShowOnDashboard, steps, schedules, time.Now().UTC(), id)
	if err := affectedOne(res, err, "modification de la scène"); err != nil {
		return Scene{}, err
	}
	return s.GetScene(ctx, id)
}

// DeleteScene supprime une scène. Son historique reste, sous son nom d'alors.
func (s *Store) DeleteScene(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM scene WHERE id = ?`, id)
	return affectedOne(res, err, "suppression de la scène")
}

// StartSceneRun note le début d'une exécution.
func (s *Store) StartSceneRun(ctx context.Context, id int64, trigger string, at time.Time) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE scene SET last_run_at = ?, last_trigger = ?, last_status = ? WHERE id = ?`,
		at, trigger, SceneRunning, id)
	return affectedOne(res, err, "début d'exécution de la scène")
}

// FinishSceneRun note l'issue d'une exécution.
func (s *Store) FinishSceneRun(ctx context.Context, id int64, status string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE scene SET last_status = ? WHERE id = ?`, status, id)
	return affectedOne(res, err, "fin d'exécution de la scène")
}

// InterruptRunningScenes marque interrompues les exécutions qu'un arrêt du
// service a coupées. Appelée au démarrage, avant tout lancement.
func (s *Store) InterruptRunningScenes(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE scene SET last_status = ? WHERE last_status = ?`, SceneInterrupted, SceneRunning)
	if err != nil {
		return 0, fmt.Errorf("scènes interrompues: %w", err)
	}
	return res.RowsAffected()
}

func encodeSpec(spec SceneSpec) (steps, schedules string, err error) {
	if spec.Steps == nil {
		spec.Steps = []SceneStep{}
	}
	if spec.Schedules == nil {
		spec.Schedules = []SceneSchedule{}
	}
	a, err := json.Marshal(spec.Steps)
	if err != nil {
		return "", "", fmt.Errorf("encodage des étapes: %w", err)
	}
	b, err := json.Marshal(spec.Schedules)
	if err != nil {
		return "", "", fmt.Errorf("encodage des horaires: %w", err)
	}
	return string(a), string(b), nil
}

func scanScene(sc scanner) (Scene, error) {
	var (
		s                Scene
		steps, schedules string
		lastRun          sql.NullTime
	)
	err := sc.Scan(&s.ID, &s.Name, &s.ShowOnDashboard, &steps, &schedules, &lastRun, &s.LastTrigger,
		&s.LastStatus, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Scene{}, err
		}
		return Scene{}, fmt.Errorf("lecture d'une scène: %w", err)
	}
	if lastRun.Valid {
		t := lastRun.Time
		s.LastRunAt = &t
	}
	if err := json.Unmarshal([]byte(steps), &s.Steps); err != nil {
		return Scene{}, fmt.Errorf("étapes de la scène %d illisibles: %w", s.ID, err)
	}
	if err := json.Unmarshal([]byte(schedules), &s.Schedules); err != nil {
		return Scene{}, fmt.Errorf("horaires de la scène %d illisibles: %w", s.ID, err)
	}
	return s, nil
}

// affectedOne transforme un UPDATE ou DELETE sans ligne touchée en ErrNotFound.
func affectedOne(res sql.Result, err error, what string) error {
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
