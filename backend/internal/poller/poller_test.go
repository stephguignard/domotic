package poller

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/stephguignard/domotic/internal/config"
	"github.com/stephguignard/domotic/internal/hue"
	"github.com/stephguignard/domotic/internal/store"
)

func newTestPoller(t *testing.T) (*Poller, *store.Store) {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return New(&config.Config{}, st, Clients{}, slog.New(slog.NewTextHandler(io.Discard, nil))), st
}

func hueEvent(t *testing.T, raw string) []hue.Event {
	t.Helper()
	var events []hue.Event
	if err := json.Unmarshal([]byte(raw), &events); err != nil {
		t.Fatalf("événement de test illisible: %v", err)
	}
	return events
}

func nudged(p *Poller, source string) bool {
	select {
	case <-p.nudges[source]:
		return true
	default:
		return false
	}
}

func TestHueUpdateMergesPartialState(t *testing.T) {
	ctx := context.Background()
	p, st := newTestPoller(t)

	if err := st.UpsertDevices(ctx, []store.Device{{
		ID: "l-1", Source: "hue", Name: "Plafonnier", Kind: "light", Room: "Salon",
		State: `{"brightness":40,"on":true}`, Reachable: false, UpdatedAt: time.Now().UTC(),
	}}); err != nil {
		t.Fatalf("UpsertDevices: %v", err)
	}

	// L'événement ne porte que « on » : la luminosité doit être conservée.
	p.applyHueEvents(ctx, hueEvent(t, `[{"type":"update","data":[{"id":"l-1","type":"light","on":{"on":false}}]}]`))

	d, err := st.GetDevice(ctx, "l-1")
	if err != nil {
		t.Fatalf("GetDevice: %v", err)
	}
	if d.State != `{"brightness":40,"on":false}` {
		t.Errorf("état = %s", d.State)
	}
	if !d.Reachable {
		t.Error("un événement reçu doit rendre la lumière joignable")
	}
	if d.Room != "Salon" || d.Name != "Plafonnier" {
		t.Errorf("métadonnées modifiées: %+v", d)
	}
	if nudged(p, "hue") {
		t.Error("une mise à jour d'état ne doit pas relancer l'inventaire")
	}
}

func TestHueEventsNudgeInventory(t *testing.T) {
	cases := map[string]string{
		"lumière inconnue":    `[{"type":"update","data":[{"id":"l-9","type":"light","on":{"on":true}}]}]`,
		"ajout":               `[{"type":"add","data":[{"id":"l-9","type":"light"}]}]`,
		"liaison Zigbee":      `[{"type":"update","data":[{"id":"z-1","type":"zigbee_connectivity"}]}]`,
		"changement de pièce": `[{"type":"update","data":[{"id":"r-1","type":"room"}]}]`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			p, _ := newTestPoller(t)
			p.applyHueEvents(context.Background(), hueEvent(t, raw))
			if !nudged(p, "hue") {
				t.Error("attendu une relecture de l'inventaire")
			}
		})
	}
}
