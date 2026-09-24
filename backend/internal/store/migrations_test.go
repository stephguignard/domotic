package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
)

// TestSourceMigrationKeepsMeasurements vérifie que la reconstruction de la
// table device (migration 0003) n'efface pas l'historique. Supprimer la table
// avec les clés étrangères actives déclencherait l'ON DELETE CASCADE de
// measurement : c'est exactement l'erreur que la migration doit éviter.
func TestSourceMigrationKeepsMeasurements(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")

	// Base arrêtée à la version 2, avec les mêmes pragmas que store.Open.
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_pragma=foreign_keys(1)", url.PathEscape(path)))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	db.SetMaxOpenConns(1)
	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("SetDialect: %v", err)
	}
	if err := goose.UpToContext(ctx, db, "migrations", 2); err != nil {
		t.Fatalf("migrations jusqu'à la version 2: %v", err)
	}

	at := time.Now().UTC().Truncate(time.Second)
	if _, err := db.ExecContext(ctx,
		`INSERT INTO device (id, source, name, kind, updated_at) VALUES ('mod-1', 'netatmo', 'Station', 'weather_station', ?)`,
		at); err != nil {
		t.Fatalf("insertion de l'équipement: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO measurement (device_id, metric, value, recorded_at) VALUES ('mod-1', 'temperature', 21.5, ?)`,
		at); err != nil {
		t.Fatalf("insertion du relevé: %v", err)
	}
	db.Close()

	s, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	ms, err := s.ListMeasurements(ctx, MeasurementFilter{DeviceID: "mod-1"})
	if err != nil {
		t.Fatalf("ListMeasurements: %v", err)
	}
	if len(ms) != 1 {
		t.Fatalf("attendu 1 relevé conservé, obtenu %d", len(ms))
	}

	// Les nouvelles sources sont acceptées…
	for _, source := range []string{"hue", "shelly"} {
		d := testDevice(source + "-1")
		d.Source = source
		if err := s.UpsertDevices(ctx, []Device{d}); err != nil {
			t.Errorf("source %s refusée: %v", source, err)
		}
	}

	// … la contrainte existe toujours pour les autres…
	d := testDevice("x-1")
	d.Source = "inconnue"
	if err := s.UpsertDevices(ctx, []Device{d}); err == nil {
		t.Error("attendu un refus pour une source inconnue")
	}

	// … et les clés étrangères sont réactivées : supprimer l'équipement
	// emporte bien son historique.
	if _, err := s.DB().ExecContext(ctx, `DELETE FROM device WHERE id = 'mod-1'`); err != nil {
		t.Fatalf("suppression: %v", err)
	}
	var n int
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM measurement`).Scan(&n); err != nil {
		t.Fatalf("comptage: %v", err)
	}
	if n != 0 {
		t.Errorf("clés étrangères inactives après migration : %d relevé(s) orphelin(s)", n)
	}
}
