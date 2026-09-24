package api

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/stephguignard/domotic/internal/store"
)

// TestCommandsAreRecorded vérifie que chaque tentative sur un équipement
// connu entre dans l'historique, y compris celles que la source n'a jamais
// reçues.
func TestCommandsAreRecorded(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	now := time.Now().UTC()
	if err := st.UpsertDevices(ctx, []store.Device{
		{ID: "mod-1", Source: "netatmo", Name: "Station", Kind: "weather_station", UpdatedAt: now},
		{ID: "io://1/1", Source: "tahoma", Name: "Volet salon", Kind: "shutter", UpdatedAt: now},
	}); err != nil {
		t.Fatalf("UpsertDevices: %v", err)
	}

	_, api := humatest.New(t)
	// Aucun client configuré : TaHoma répondra 503, Netatmo 422.
	Register(api, Deps{Store: st})

	if resp := api.Post("/api/devices/mod-1/command", map[string]any{"command": "open"}); resp.Code != http.StatusUnprocessableEntity {
		t.Errorf("Netatmo : statut %d, attendu 422", resp.Code)
	}
	if resp := api.Post("/api/devices/io:%2F%2F1%2F1/command", map[string]any{"command": "close", "parameters": []any{}}); resp.Code != http.StatusServiceUnavailable {
		t.Errorf("TaHoma non configurée : statut %d, attendu 503", resp.Code)
	}
	// Un équipement inconnu n'a rien à historiser.
	if resp := api.Post("/api/devices/inconnu/command", map[string]any{"command": "on"}); resp.Code != http.StatusNotFound {
		t.Errorf("équipement inconnu : statut %d, attendu 404", resp.Code)
	}

	resp := api.Get("/api/commands")
	var body struct {
		Entries []store.CommandLogEntry `json:"entries"`
		Total   int                     `json:"total"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("réponse illisible: %v", err)
	}
	if body.Total != 2 {
		t.Fatalf("attendu 2 entrées, obtenu %d: %+v", body.Total, body.Entries)
	}
	last := body.Entries[0]
	if last.DeviceName != "Volet salon" || last.Command != "close" || last.Success ||
		last.Error != "intégration tahoma non configurée" {
		t.Errorf("dernière entrée: %+v", last)
	}

	resp = api.Get("/api/commands?device_id=mod-1")
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil || body.Total != 1 {
		t.Errorf("filtre par équipement : %d entrée(s), %v", body.Total, err)
	}
}

func TestRoomChangesAreApplied(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.UpsertDevices(ctx, []store.Device{
		{ID: "sw-1", Source: "shelly", Name: "Eau chaude", Kind: "switch", UpdatedAt: time.Now().UTC()},
	}); err != nil {
		t.Fatalf("UpsertDevices: %v", err)
	}

	_, api := humatest.New(t)
	Register(api, Deps{Store: st})

	resp := api.Put("/api/devices/sw-1/room", map[string]any{"room": "  Buanderie "})
	var device store.Device
	if err := json.Unmarshal(resp.Body.Bytes(), &device); err != nil || resp.Code != http.StatusOK {
		t.Fatalf("PUT : statut %d, %v", resp.Code, err)
	}
	if device.Room != "Buanderie" || !device.RoomOverridden {
		t.Errorf("après choix : %+v", device)
	}

	resp = api.Delete("/api/devices/sw-1/room")
	if err := json.Unmarshal(resp.Body.Bytes(), &device); err != nil || resp.Code != http.StatusOK {
		t.Fatalf("DELETE : statut %d, %v", resp.Code, err)
	}
	if device.Room != "" || device.RoomOverridden {
		t.Errorf("après rétablissement : %+v", device)
	}

	if resp := api.Put("/api/devices/inconnu/room", map[string]any{"room": "x"}); resp.Code != http.StatusNotFound {
		t.Errorf("équipement inconnu : statut %d", resp.Code)
	}

	entries, err := st.ListCommands(ctx, store.CommandLogFilter{})
	if err != nil || len(entries) != 2 {
		t.Fatalf("historique : %d entrée(s), %v", len(entries), err)
	}
	if entries[1].Command != "setRoom" || entries[1].Parameters[0] != "Buanderie" || entries[0].Command != "resetRoom" {
		t.Errorf("historique : %+v", entries)
	}
}
