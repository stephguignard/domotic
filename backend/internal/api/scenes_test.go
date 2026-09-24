package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/stephguignard/domotic/internal/command"
	"github.com/stephguignard/domotic/internal/control"
	"github.com/stephguignard/domotic/internal/scenes"
	"github.com/stephguignard/domotic/internal/store"
)

type recordingSource struct {
	mu    sync.Mutex
	calls []string
}

func (r *recordingSource) Execute(_ context.Context, id, cmd string, _ []any) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, id+":"+cmd)
	return "", nil
}

func TestScenesAPI(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	now := time.Now().UTC()
	if err := st.UpsertDevices(ctx, []store.Device{
		{ID: "sw-1", Source: "shelly", Name: "Eau chaude", Kind: "switch", Room: "Buanderie", State: `{"on":true}`, Reachable: true, UpdatedAt: now},
		{ID: "vol-1", Source: "tahoma", Name: "Volet", Kind: "shutter", Room: "Buanderie", State: `{}`, Reachable: true, UpdatedAt: now},
	}); err != nil {
		t.Fatalf("UpsertDevices: %v", err)
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	src := &recordingSource{}
	ctl := control.New(st, map[string]command.Commander{"shelly": src, "tahoma": src}, nil, log)
	engine := scenes.New(st, ctl, scenes.Place{TimeZone: time.UTC}, log)
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { engine.Run(runCtx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })

	_, api := humatest.New(t)
	Register(api, Deps{Store: st, Control: ctl, Scenes: engine})

	spec := map[string]any{
		"name":              "Arrêt buanderie",
		"show_on_dashboard": true,
		"steps": []any{map[string]any{
			"type": "action", "command": "off",
			"targets": map[string]any{"devices": []string{}, "rooms": []string{"Buanderie"}, "kinds": []string{}},
		}},
		"schedules": []any{map[string]any{"enabled": true, "days": []int{1, 2, 3, 4, 5, 6, 7}, "at": "time", "time": "23:00"}},
	}
	resp := api.Post("/api/scenes", spec)
	if resp.Code != http.StatusCreated {
		t.Fatalf("création : statut %d, %s", resp.Code, resp.Body.String())
	}
	var created scenes.SceneView
	if err := json.Unmarshal(resp.Body.Bytes(), &created); err != nil {
		t.Fatalf("réponse illisible: %v", err)
	}
	if created.NextRunAt == nil || !created.TouchesRelays {
		t.Errorf("scène créée : prochaine %v, relais %v", created.NextRunAt, created.TouchesRelays)
	}

	// Un horaire solaire est refusé tant que les coordonnées manquent.
	spec["schedules"] = []any{map[string]any{"enabled": true, "days": []int{1}, "at": "sunset"}}
	if resp := api.Post("/api/scenes", spec); resp.Code != http.StatusUnprocessableEntity {
		t.Errorf("horaire solaire sans coordonnées : statut %d", resp.Code)
	}

	// Aperçu : le relais reçoit « off », le volet est écarté.
	resp = api.Post("/api/scenes/resolve", map[string]any{"steps": spec["steps"]})
	var resolved struct {
		Steps []ResolvedStep `json:"steps"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &resolved); err != nil || len(resolved.Steps) != 1 {
		t.Fatalf("aperçu : %s, %v", resp.Body.String(), err)
	}
	if s := resolved.Steps[0]; len(s.Targets) != 1 || s.Targets[0].ID != "sw-1" || len(s.Ignored) != 1 || s.Ignored[0].DeviceID != "vol-1" {
		t.Errorf("aperçu : %+v", s)
	}

	path := "/api/scenes/" + jsonNumber(created.ID)
	if resp := api.Post(path + "/run"); resp.Code != http.StatusAccepted {
		t.Fatalf("lancement : statut %d, %s", resp.Code, resp.Body.String())
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if sc, _ := st.GetScene(ctx, created.ID); sc.LastStatus == store.SceneSuccess {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	resp = api.Get("/api/commands?scene_id=" + jsonNumber(created.ID))
	var history struct {
		Entries []store.CommandLogEntry `json:"entries"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &history); err != nil || len(history.Entries) != 1 {
		t.Fatalf("historique de la scène : %s", resp.Body.String())
	}
	if e := history.Entries[0]; e.DeviceID != "sw-1" || e.Origin != store.OriginSceneManual || e.SceneName != "Arrêt buanderie" {
		t.Errorf("entrée : %+v", e)
	}

	if resp := api.Delete(path); resp.Code != http.StatusNoContent {
		t.Errorf("suppression : statut %d", resp.Code)
	}
	if resp := api.Get(path); resp.Code != http.StatusNotFound {
		t.Errorf("après suppression : statut %d", resp.Code)
	}
}

func jsonNumber(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
