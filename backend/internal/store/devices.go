package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrNotFound est retournée quand l'enregistrement demandé n'existe pas.
var ErrNotFound = errors.New("introuvable")

// Device est la représentation unifiée d'un équipement, quelle que soit sa
// source. Les tags JSON servent aussi à la génération du schéma OpenAPI.
type Device struct {
	ID        string    `json:"id" doc:"Identifiant de l'équipement dans sa source d'origine"`
	Source    string    `json:"source" enum:"netatmo,tahoma" doc:"Source de l'équipement"`
	Name      string    `json:"name" doc:"Nom lisible"`
	Kind      string    `json:"kind" doc:"Type d'équipement, ex. weather_station, shutter, camera"`
	Room      string    `json:"room" doc:"Pièce, si connue"`
	State     string    `json:"state" doc:"État courant normalisé, encodé en JSON"`
	Reachable bool      `json:"reachable" doc:"L'équipement répond-il ?"`
	UpdatedAt time.Time `json:"updated_at" doc:"Date du dernier rafraîchissement"`
}

// DeviceFilter restreint la liste des équipements retournés.
type DeviceFilter struct {
	Source string
	Room   string
}

// ListDevices retourne les équipements correspondant au filtre, triés par
// pièce puis par nom.
func (s *Store) ListDevices(ctx context.Context, f DeviceFilter) ([]Device, error) {
	var (
		where []string
		args  []any
	)
	if f.Source != "" {
		where = append(where, "source = ?")
		args = append(args, f.Source)
	}
	if f.Room != "" {
		where = append(where, "room = ?")
		args = append(args, f.Room)
	}

	query := `SELECT id, source, name, kind, room, state, reachable, updated_at FROM device`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY room, name"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("liste des équipements: %w", err)
	}
	defer rows.Close()

	devices := []Device{}
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("liste des équipements: %w", err)
	}
	return devices, nil
}

// GetDevice retourne un équipement par son identifiant.
func (s *Store) GetDevice(ctx context.Context, id string) (Device, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, source, name, kind, room, state, reachable, updated_at FROM device WHERE id = ?`, id)

	d, err := scanDevice(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Device{}, ErrNotFound
	}
	return d, err
}

// UpsertDevices insère ou met à jour un lot d'équipements dans une seule
// transaction. C'est l'opération qu'appellent les boucles de polling après
// chaque rafraîchissement.
func (s *Store) UpsertDevices(ctx context.Context, devices []Device) error {
	if len(devices) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("upsert des équipements: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // sans effet après un Commit réussi

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO device (id, source, name, kind, room, state, reachable, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			name       = excluded.name,
			kind       = excluded.kind,
			room       = excluded.room,
			state      = excluded.state,
			reachable  = excluded.reachable,
			updated_at = excluded.updated_at`)
	if err != nil {
		return fmt.Errorf("upsert des équipements: %w", err)
	}
	defer stmt.Close()

	for _, d := range devices {
		if _, err := stmt.ExecContext(ctx,
			d.ID, d.Source, d.Name, d.Kind, d.Room, d.State, d.Reachable, d.UpdatedAt,
		); err != nil {
			return fmt.Errorf("upsert de l'équipement %s: %w", d.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("upsert des équipements: %w", err)
	}
	return nil
}

// UpdateDeviceState met à jour uniquement l'état d'un équipement, sans toucher
// à ses métadonnées. Utilisé par le flux d'événements TaHoma, qui ne transporte
// que les changements d'état.
func (s *Store) UpdateDeviceState(ctx context.Context, id, state string, reachable bool, at time.Time) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE device SET state = ?, reachable = ?, updated_at = ? WHERE id = ?`,
		state, reachable, at, id)
	if err != nil {
		return fmt.Errorf("mise à jour de l'état de %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mise à jour de l'état de %s: %w", id, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListRooms retourne les pièces connues, sans doublon.
func (s *Store) ListRooms(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT room FROM device WHERE room != '' ORDER BY room`)
	if err != nil {
		return nil, fmt.Errorf("liste des pièces: %w", err)
	}
	defer rows.Close()

	rooms := []string{}
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, fmt.Errorf("liste des pièces: %w", err)
		}
		rooms = append(rooms, r)
	}
	return rooms, rows.Err()
}

// scanner couvre *sql.Row et *sql.Rows, dont les Scan ont la même signature.
type scanner interface {
	Scan(dest ...any) error
}

func scanDevice(sc scanner) (Device, error) {
	var d Device
	err := sc.Scan(&d.ID, &d.Source, &d.Name, &d.Kind, &d.Room, &d.State, &d.Reachable, &d.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Device{}, err // laissé à l'appelant pour distinguer le cas
		}
		return Device{}, fmt.Errorf("lecture d'un équipement: %w", err)
	}
	return d, nil
}
