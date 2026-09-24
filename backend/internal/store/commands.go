package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// CommandLogEntry est une commande envoyée depuis l'interface, telle
// qu'enregistrée dans l'historique.
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
	CreatedAt  time.Time `json:"created_at" doc:"Date de l'action"`
}

// CommandLogFilter restreint l'historique retourné.
type CommandLogFilter struct {
	DeviceID string
	Limit    int
}

// RecordCommand ajoute une entrée à l'historique. ID est ignoré.
func (s *Store) RecordCommand(ctx context.Context, e CommandLogEntry) error {
	params := e.Parameters
	if params == nil {
		params = []any{}
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("historique de la commande %s: %w", e.Command, err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO command_log
			(device_id, device_name, device_kind, source, command, parameters, success, error, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.DeviceID, e.DeviceName, e.DeviceKind, e.Source, e.Command, string(encoded), e.Success, e.Error, e.CreatedAt)
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

	query := `SELECT id, device_id, device_name, device_kind, source, command, parameters, success, error, created_at
		FROM command_log`
	var args []any
	if f.DeviceID != "" {
		query += ` WHERE device_id = ?`
		args = append(args, f.DeviceID)
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
			&params, &e.Success, &e.Error, &e.CreatedAt); err != nil {
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
