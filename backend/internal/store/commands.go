package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// CommandLogEntry est une action faite depuis l'interface, telle
// qu'enregistrée dans l'historique : une commande transmise à la source, ou un
// changement de pièce (setRoom, resetRoom), qui ne quitte pas le service.
type CommandLogEntry struct {
	ID         int64     `json:"id" doc:"Identifiant de l'entrée"`
	DeviceID   string    `json:"device_id" doc:"Équipement visé"`
	DeviceName string    `json:"device_name" doc:"Nom de l'équipement au moment de l'action"`
	DeviceKind string    `json:"device_kind" doc:"Type de l'équipement au moment de l'action"`
	Source     string    `json:"source" enum:"netatmo,tahoma,hue,shelly" doc:"Source de l'équipement"`
	Command    string    `json:"command" doc:"Commande envoyée, ex. on, setBrightness"`
	Parameters []any     `json:"parameters" nullable:"false" doc:"Paramètres de la commande"`
	Success    bool      `json:"success" doc:"La source a-t-elle accepté la commande ?"`
	Error      string    `json:"error,omitempty" doc:"Motif du refus, le cas échéant"`
	Origin     string    `json:"origin" enum:"interface,scene_manual,scene_schedule" doc:"Origine : l'interface, ou une scène lancée à la main ou par un horaire"`
	SceneID    *int64    `json:"scene_id,omitempty" doc:"Scène à l'origine de l'action, le cas échéant"`
	SceneName  string    `json:"scene_name,omitempty" doc:"Nom de la scène au moment de l'action"`
	CreatedAt  time.Time `json:"created_at" doc:"Date de l'action"`
}

// Origines d'une action de l'historique.
const (
	OriginInterface     = "interface"
	OriginSceneManual   = "scene_manual"
	OriginSceneSchedule = "scene_schedule"
)

// CommandLogFilter restreint l'historique retourné.
type CommandLogFilter struct {
	DeviceID string
	SceneID  int64
	Limit    int
}

// RecordCommand ajoute une entrée à l'historique. ID est ignoré.
func (s *Store) RecordCommand(ctx context.Context, e CommandLogEntry) error {
	params := e.Parameters
	if params == nil {
		params = []any{}
	}
	origin := e.Origin
	if origin == "" {
		origin = OriginInterface
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("historique de la commande %s: %w", e.Command, err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO command_log
			(device_id, device_name, device_kind, source, command, parameters, success, error,
			 origin, scene_id, scene_name, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.DeviceID, e.DeviceName, e.DeviceKind, e.Source, e.Command, string(encoded), e.Success, e.Error,
		origin, e.SceneID, e.SceneName, e.CreatedAt)
	if err != nil {
		return fmt.Errorf("historique de la commande %s: %w", e.Command, err)
	}
	return nil
}

// ListCommands retourne l'historique, de l'action la plus récente à la plus
// ancienne.
func (s *Store) ListCommands(ctx context.Context, f CommandLogFilter) ([]CommandLogEntry, error) {
	limit := f.Limit
	if limit <= 0 || limit > 1000 {
		limit = 200
	}

	query := `SELECT id, device_id, device_name, device_kind, source, command, parameters, success, error,
		origin, scene_id, scene_name, created_at
		FROM command_log`
	var (
		where []string
		args  []any
	)
	if f.DeviceID != "" {
		where = append(where, `device_id = ?`)
		args = append(args, f.DeviceID)
	}
	if f.SceneID != 0 {
		where = append(where, `scene_id = ?`)
		args = append(args, f.SceneID)
	}
	if len(where) > 0 {
		query += ` WHERE ` + strings.Join(where, ` AND `)
	}
	// id départage deux actions de la même seconde.
	query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("lecture de l'historique: %w", err)
	}
	defer rows.Close()

	out := []CommandLogEntry{}
	for rows.Next() {
		var (
			e      CommandLogEntry
			params string
		)
		if err := rows.Scan(&e.ID, &e.DeviceID, &e.DeviceName, &e.DeviceKind, &e.Source, &e.Command,
			&params, &e.Success, &e.Error, &e.Origin, &e.SceneID, &e.SceneName, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("lecture de l'historique: %w", err)
		}
		if err := json.Unmarshal([]byte(params), &e.Parameters); err != nil || e.Parameters == nil {
			// Une entrée illisible reste affichable, sans ses paramètres.
			e.Parameters = []any{}
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// PurgeCommandsBefore supprime les entrées antérieures à la date donnée et
// retourne le nombre de lignes effacées.
func (s *Store) PurgeCommandsBefore(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM command_log WHERE created_at < ?`, before)
	if err != nil {
		return 0, fmt.Errorf("purge de l'historique: %w", err)
	}
	return res.RowsAffected()
}
