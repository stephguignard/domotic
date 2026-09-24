package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// newTestStore ouvre une base jetable, migrations appliquées.
func newTestStore(t *testing.T) *Store {
	t.Helper()

	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func testDevice(id string) Device {
	return Device{
		ID:        id,
		Source:    "tahoma",
		Name:      "Volet salon",
		Kind:      "shutter",
		Room:      "Salon",
		State:     `{"core:ClosureState":100}`,
		Reachable: true,
		UpdatedAt: time.Now().UTC().Truncate(time.Second),
	}
}

func TestUpsertDevicesInsertsThenUpdates(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	d := testDevice("io://1234/1")
	if err := s.UpsertDevices(ctx, []Device{d}); err != nil {
		t.Fatalf("premier UpsertDevices: %v", err)
	}

	// Le second passage doit mettre à jour, pas dupliquer : les boucles de
	// polling réécrivent l'inventaire complet à chaque cycle.
	d.Name = "Volet séjour"
	d.State = `{"core:ClosureState":0}`
	if err := s.UpsertDevices(ctx, []Device{d}); err != nil {
		t.Fatalf("second UpsertDevices: %v", err)
	}

	devices, err := s.ListDevices(ctx, DeviceFilter{})
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("attendu 1 équipement, obtenu %d", len(devices))
	}
	if devices[0].Name != "Volet séjour" {
		t.Errorf("nom non mis à jour: %q", devices[0].Name)
	}
}

func TestListDevicesFilters(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	netatmo := testDevice("70:ee:50:00:00:01")
	netatmo.Source = "netatmo"
	netatmo.Kind = "weather_station"
	netatmo.Room = "Bureau"

	if err := s.UpsertDevices(ctx, []Device{testDevice("io://1234/1"), netatmo}); err != nil {
		t.Fatalf("UpsertDevices: %v", err)
	}

	for _, c := range []struct {
		name   string
		filter DeviceFilter
		want   int
	}{
		{"sans filtre", DeviceFilter{}, 2},
		{"par source", DeviceFilter{Source: "netatmo"}, 1},
		{"par pièce", DeviceFilter{Room: "Salon"}, 1},
		{"combinaison sans résultat", DeviceFilter{Source: "netatmo", Room: "Salon"}, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			devices, err := s.ListDevices(ctx, c.filter)
			if err != nil {
				t.Fatalf("ListDevices: %v", err)
			}
			if len(devices) != c.want {
				t.Errorf("attendu %d équipements, obtenu %d", c.want, len(devices))
			}
		})
	}
}

func TestGetDeviceNotFound(t *testing.T) {
	_, err := newTestStore(t).GetDevice(context.Background(), "inconnu")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("attendu ErrNotFound, obtenu %v", err)
	}
}

func TestUpdateDeviceStateRejectsUnknownDevice(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// Le flux d'événements TaHoma peut mentionner un équipement pas encore
	// connu de l'inventaire : le poller compte sur ErrNotFound pour l'ignorer.
	err := s.UpdateDeviceState(ctx, "io://1234/inconnu", `{}`, true, time.Now())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("attendu ErrNotFound, obtenu %v", err)
	}
}

func TestMarkUnreachableMatchesPrefixOnly(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	var devices []Device
	for _, id := range []string{"shellypro3-aa:switch:0", "shellypro3-aa:switch:1", "shellypro3-aab:switch:0"} {
		d := testDevice(id)
		d.Source = "shelly"
		devices = append(devices, d)
	}
	if err := s.UpsertDevices(ctx, devices); err != nil {
		t.Fatalf("UpsertDevices: %v", err)
	}

	if err := s.MarkUnreachable(ctx, "shelly", "shellypro3-aa:"); err != nil {
		t.Fatalf("MarkUnreachable: %v", err)
	}

	want := map[string]bool{
		"shellypro3-aa:switch:0":  false,
		"shellypro3-aa:switch:1":  false,
		"shellypro3-aab:switch:0": true, // préfixe voisin, autre module
	}
	for id, reachable := range want {
		d, err := s.GetDevice(ctx, id)
		if err != nil {
			t.Fatalf("GetDevice(%s): %v", id, err)
		}
		if d.Reachable != reachable {
			t.Errorf("%s joignable = %v, attendu %v", id, d.Reachable, reachable)
		}
		if d.State != devices[0].State {
			t.Errorf("%s : l'état ne doit pas être modifié", id)
		}
	}
}

func TestInsertMeasurementsIgnoresDuplicates(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	d := testDevice("70:ee:50:00:00:01")
	d.Source = "netatmo"
	if err := s.UpsertDevices(ctx, []Device{d}); err != nil {
		t.Fatalf("UpsertDevices: %v", err)
	}

	at := time.Now().UTC().Truncate(time.Second)
	m := Measurement{DeviceID: d.ID, Metric: "temperature", Value: 21.5, RecordedAt: at}

	// Le polling tourne plus vite que la station ne produit des mesures : le
	// même relevé est donc réinséré à chaque cycle et doit être absorbé.
	for range 3 {
		if err := s.InsertMeasurements(ctx, []Measurement{m}); err != nil {
			t.Fatalf("InsertMeasurements: %v", err)
		}
	}

	got, err := s.ListMeasurements(ctx, MeasurementFilter{DeviceID: d.ID})
	if err != nil {
		t.Fatalf("ListMeasurements: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("attendu 1 relevé, obtenu %d", len(got))
	}
	if got[0].Value != 21.5 {
		t.Errorf("valeur inattendue: %v", got[0].Value)
	}
}

func TestMeasurementsCascadeOnDeviceDelete(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	d := testDevice("70:ee:50:00:00:01")
	if err := s.UpsertDevices(ctx, []Device{d}); err != nil {
		t.Fatalf("UpsertDevices: %v", err)
	}
	if err := s.InsertMeasurements(ctx, []Measurement{
		{DeviceID: d.ID, Metric: "temperature", Value: 20, RecordedAt: time.Now().UTC()},
	}); err != nil {
		t.Fatalf("InsertMeasurements: %v", err)
	}

	// La cascade dépend du pragma foreign_keys, activé dans le DSN.
	if _, err := s.DB().ExecContext(ctx, `DELETE FROM device WHERE id = ?`, d.ID); err != nil {
		t.Fatalf("suppression de l'équipement: %v", err)
	}

	var count int
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM measurement`).Scan(&count); err != nil {
		t.Fatalf("comptage des relevés: %v", err)
	}
	if count != 0 {
		t.Errorf("attendu 0 relevé après cascade, obtenu %d", count)
	}
}

func TestPurgeMeasurementsBefore(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	d := testDevice("70:ee:50:00:00:01")
	if err := s.UpsertDevices(ctx, []Device{d}); err != nil {
		t.Fatalf("UpsertDevices: %v", err)
	}

	now := time.Now().UTC()
	if err := s.InsertMeasurements(ctx, []Measurement{
		{DeviceID: d.ID, Metric: "temperature", Value: 18, RecordedAt: now.Add(-100 * 24 * time.Hour)},
		{DeviceID: d.ID, Metric: "temperature", Value: 21, RecordedAt: now},
	}); err != nil {
		t.Fatalf("InsertMeasurements: %v", err)
	}

	n, err := s.PurgeMeasurementsBefore(ctx, now.Add(-90*24*time.Hour))
	if err != nil {
		t.Fatalf("PurgeMeasurementsBefore: %v", err)
	}
	if n != 1 {
		t.Errorf("attendu 1 ligne purgée, obtenu %d", n)
	}
}

func TestTokenRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, err := s.GetToken(ctx, "netatmo"); !errors.Is(err, ErrNotFound) {
		t.Errorf("attendu ErrNotFound avant authentification, obtenu %v", err)
	}

	want := &oauth2.Token{
		AccessToken:  "access-1",
		RefreshToken: "refresh-1",
		TokenType:    "Bearer",
		Expiry:       time.Now().UTC().Add(3 * time.Hour).Truncate(time.Second),
	}
	if err := s.SaveToken(ctx, "netatmo", want); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	// Netatmo fait tourner le refresh token : la seconde écriture doit
	// remplacer la première, pas en créer une seconde.
	rotated := &oauth2.Token{
		AccessToken:  "access-2",
		RefreshToken: "refresh-2",
		TokenType:    "Bearer",
		Expiry:       want.Expiry.Add(3 * time.Hour),
	}
	if err := s.SaveToken(ctx, "netatmo", rotated); err != nil {
		t.Fatalf("SaveToken après rotation: %v", err)
	}

	got, err := s.GetToken(ctx, "netatmo")
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if got.RefreshToken != "refresh-2" || got.AccessToken != "access-2" {
		t.Errorf("jeton non mis à jour: %+v", got)
	}
}

func TestSaveTokenRejectsEmptyRefreshToken(t *testing.T) {
	// Écrire un refresh token vide écraserait celui en base et couperait
	// définitivement l'accès : le store doit refuser.
	err := newTestStore(t).SaveToken(context.Background(), "netatmo", &oauth2.Token{
		AccessToken: "access-1",
	})
	if err == nil {
		t.Error("attendu une erreur pour un jeton sans refresh token")
	}
}

func TestOpenAppliesMigrationsIdempotently(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("première ouverture: %v", err)
	}
	if err := s.UpsertDevices(ctx, []Device{testDevice("io://1234/1")}); err != nil {
		t.Fatalf("UpsertDevices: %v", err)
	}
	s.Close()

	// Rouvrir doit rejouer les migrations sans échouer ni perdre les données :
	// c'est ce qui se passe à chaque redémarrage du conteneur.
	again, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("seconde ouverture: %v", err)
	}
	defer again.Close()

	devices, err := again.ListDevices(ctx, DeviceFilter{})
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if len(devices) != 1 {
		t.Errorf("attendu 1 équipement conservé, obtenu %d", len(devices))
	}
}
