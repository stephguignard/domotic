package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Measurement est un relevé de capteur horodaté.
type Measurement struct {
	DeviceID   string    `json:"device_id" doc:"Équipement ayant produit le relevé"`
	Metric     string    `json:"metric" doc:"Grandeur mesurée, ex. temperature, humidity, co2"`
	Value      float64   `json:"value" doc:"Valeur mesurée"`
	RecordedAt time.Time `json:"recorded_at" doc:"Horodatage du relevé, tel que fourni par la source"`
}

// MeasurementFilter restreint la plage et la nature des relevés retournés.
type MeasurementFilter struct {
	DeviceID string
	Metric   string
	From     time.Time
	To       time.Time
	Limit    int
}

// InsertMeasurements enregistre un lot de relevés. Les doublons — même
// équipement, même grandeur, même horodatage — sont ignorés : le polling
// tourne plus vite que la station ne produit de nouvelles mesures.
func (s *Store) InsertMeasurements(ctx context.Context, ms []Measurement) error {
	if len(ms) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("insertion des relevés: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // sans effet après un Commit réussi

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO measurement (device_id, metric, value, recorded_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (device_id, metric, recorded_at) DO NOTHING`)
	if err != nil {
		return fmt.Errorf("insertion des relevés: %w", err)
	}
	defer stmt.Close()

	for _, m := range ms {
		if _, err := stmt.ExecContext(ctx, m.DeviceID, m.Metric, m.Value, m.RecordedAt); err != nil {
			return fmt.Errorf("insertion du relevé %s/%s: %w", m.DeviceID, m.Metric, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("insertion des relevés: %w", err)
	}
	return nil
}

// ListMeasurements retourne les relevés correspondant au filtre, du plus
// récent au plus ancien.
func (s *Store) ListMeasurements(ctx context.Context, f MeasurementFilter) ([]Measurement, error) {
	where := []string{"device_id = ?"}
	args := []any{f.DeviceID}

	if f.Metric != "" {
		where = append(where, "metric = ?")
		args = append(args, f.Metric)
	}
	if !f.From.IsZero() {
		where = append(where, "recorded_at >= ?")
		args = append(args, f.From)
	}
	if !f.To.IsZero() {
		where = append(where, "recorded_at <= ?")
		args = append(args, f.To)
	}

	limit := f.Limit
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}

	query := `SELECT device_id, metric, value, recorded_at FROM measurement WHERE ` +
		strings.Join(where, " AND ") + ` ORDER BY recorded_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("liste des relevés: %w", err)
	}
	defer rows.Close()

	out := []Measurement{}
	for rows.Next() {
		var m Measurement
		if err := rows.Scan(&m.DeviceID, &m.Metric, &m.Value, &m.RecordedAt); err != nil {
			return nil, fmt.Errorf("liste des relevés: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// PurgeMeasurementsBefore supprime les relevés antérieurs à la date donnée et
// retourne le nombre de lignes effacées. Le NAS n'a qu'un gigaoctet de RAM et
// un disque partagé : laisser l'historique croître sans limite finirait par
// peser.
func (s *Store) PurgeMeasurementsBefore(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM measurement WHERE recorded_at < ?`, before)
	if err != nil {
		return 0, fmt.Errorf("purge des relevés: %w", err)
	}
	return res.RowsAffected()
}
