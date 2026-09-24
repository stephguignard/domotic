package store

import (
	"context"
	"fmt"
)

// RoomOrder retourne les pièces classées, dans l'ordre choisi.
func (s *Store) RoomOrder(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT room FROM room_order ORDER BY position`)
	if err != nil {
		return nil, fmt.Errorf("lecture de l'ordre des pièces: %w", err)
	}
	defer rows.Close()

	rooms := []string{}
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, fmt.Errorf("lecture de l'ordre des pièces: %w", err)
		}
		rooms = append(rooms, r)
	}
	return rooms, rows.Err()
}

// SetRoomOrder remplace l'ordre des pièces. Une liste vide revient à l'ordre
// alphabétique.
func (s *Store) SetRoomOrder(ctx context.Context, rooms []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("enregistrement de l'ordre des pièces: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // sans effet après un Commit réussi

	if _, err := tx.ExecContext(ctx, `DELETE FROM room_order`); err != nil {
		return fmt.Errorf("enregistrement de l'ordre des pièces: %w", err)
	}
	for i, room := range rooms {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO room_order (room, position) VALUES (?, ?)`, room, i); err != nil {
			return fmt.Errorf("enregistrement de la position de %q: %w", room, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("enregistrement de l'ordre des pièces: %w", err)
	}
	return nil
}
