package shelly

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stephguignard/domotic/internal/command"
	"github.com/stephguignard/domotic/internal/config"
)

// fakeModule imite un Shelly Pro 3 : trois voies, dont une sans nom.
type fakeModule struct {
	password string // vide : authentification désactivée

	mu      sync.Mutex
	outputs [3]bool
	calls   []string
}

func (f *fakeModule) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/rpc" || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	if f.password != "" && !f.authorized(r) {
		w.Header().Set("WWW-Authenticate",
			`Digest qop="auth", realm="shellypro3-841fe88e5b68", nonce="60dc59c6", algorithm=SHA-256`)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var req struct {
		ID     int64          `json:"id"`
		Method string         `json:"method"`
		Params map[string]any `json:"params"`
	}
	body, _ := io.ReadAll(r.Body)
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, req.Method)

	var result any
	switch req.Method {
	case "Shelly.GetConfig":
		result = map[string]any{
			"sys":      map[string]any{"device": map[string]any{"name": "Pro3"}},
			"switch:0": map[string]any{"id": 0, "name": "Eau chaude"},
			"switch:1": map[string]any{"id": 1, "name": "Chauffages muraux"},
			"switch:2": map[string]any{"id": 2, "name": nil},
			"input:0":  map[string]any{"id": 0, "name": nil},
		}
	case "Shelly.GetStatus":
		st := map[string]any{"input:0": map[string]any{"id": 0, "state": true}}
		for i, on := range f.outputs {
			st["switch:"+string(rune('0'+i))] = map[string]any{
				"id": i, "output": on, "temperature": map[string]any{"tC": 44.0},
			}
		}
		result = st
	case "Switch.Set":
		n := int(req.Params["id"].(float64))
		f.outputs[n] = req.Params["on"].(bool)
		result = map[string]any{"was_on": !f.outputs[n]}
	default:
		json.NewEncoder(w).Encode(map[string]any{
			"id": req.ID, "src": "shellypro3-841fe88e5b68",
			"error": map[string]any{"code": 404, "message": "No handler for " + req.Method},
		})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"id": req.ID, "src": "shellypro3-841fe88e5b68", "result": result})
}

// authorized vérifie l'en-tête Digest comme le ferait le module.
func (f *fakeModule) authorized(r *http.Request) bool {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Digest ") {
		return false
	}
	p := map[string]string{}
	for _, part := range splitParams(strings.TrimPrefix(h, "Digest ")) {
		k, v, _ := strings.Cut(part, "=")
		p[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"`)
	}
	want := digestResponse(p["username"], "shellypro3-841fe88e5b68", f.password,
		r.Method, p["uri"], "60dc59c6", p["nc"], p["cnonce"])
	return p["username"] == "admin" && p["response"] == want
}

func newTestClient(t *testing.T, f *fakeModule, password string) *Client {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	cfg := &config.Config{Shelly: config.ShellyConfig{
		Hosts:    []string{strings.TrimPrefix(srv.URL, "http://")},
		Password: password,
	}}
	return NewClient(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestFetchMapsChannels(t *testing.T) {
	f := &fakeModule{outputs: [3]bool{true, false, false}}
	c := newTestClient(t, f, "")

	snap, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(snap.Devices) != 3 {
		t.Fatalf("attendu 3 voies, obtenu %d", len(snap.Devices))
	}

	d := snap.Devices[0]
	if d.ID != "shellypro3-841fe88e5b68:switch:0" || d.Source != "shelly" || d.Kind != "switch" {
		t.Errorf("voie 0 mal convertie: %+v", d)
	}
	if d.Name != "Eau chaude" {
		t.Errorf("nom = %q, attendu le nom de la voie", d.Name)
	}
	if d.State != `{"device_temperature":44,"on":true}` {
		t.Errorf("état = %s", d.State)
	}
	// Une voie sans nom reçoit celui du module, numérotée à partir de 1.
	if snap.Devices[2].Name != "Pro3 voie 3" {
		t.Errorf("nom de repli = %q", snap.Devices[2].Name)
	}
}

func TestFetchReusesConfig(t *testing.T) {
	f := &fakeModule{}
	c := newTestClient(t, f, "")

	for range 3 {
		if _, err := c.Fetch(context.Background()); err != nil {
			t.Fatalf("Fetch: %v", err)
		}
	}
	got := strings.Join(f.calls, ",")
	want := "Shelly.GetConfig,Shelly.GetStatus,Shelly.GetStatus,Shelly.GetStatus"
	if got != want {
		t.Errorf("appels = %s, attendu %s", got, want)
	}
}

func TestFetchReportsUnreachableKnownModule(t *testing.T) {
	f := &fakeModule{}
	srv := httptest.NewServer(f)
	host := strings.TrimPrefix(srv.URL, "http://")
	c := NewClient(&config.Config{Shelly: config.ShellyConfig{Hosts: []string{host}}},
		slog.New(slog.NewTextHandler(io.Discard, nil)))

	if _, err := c.Fetch(context.Background()); err != nil {
		t.Fatalf("premier Fetch: %v", err)
	}
	srv.Close()

	snap, err := c.Fetch(context.Background())
	if err == nil {
		t.Fatal("attendu une erreur pour un module arrêté")
	}
	if len(snap.Unreachable) != 1 || snap.Unreachable[0] != "shellypro3-841fe88e5b68" {
		t.Errorf("modules injoignables = %v", snap.Unreachable)
	}
}

func TestExecuteSwitchesChannel(t *testing.T) {
	f := &fakeModule{}
	c := newTestClient(t, f, "")
	ctx := context.Background()

	if _, err := c.Fetch(ctx); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if _, err := c.Execute(ctx, "shellypro3-841fe88e5b68:switch:1", "on", nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !f.outputs[1] || f.outputs[0] || f.outputs[2] {
		t.Errorf("sorties = %v, seule la voie 1 devait s'allumer", f.outputs)
	}
}

func TestExecuteRejectsUnknownCommand(t *testing.T) {
	f := &fakeModule{}
	c := newTestClient(t, f, "")
	ctx := context.Background()

	if _, err := c.Fetch(ctx); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	_, err := c.Execute(ctx, "shellypro3-841fe88e5b68:switch:0", "open", nil)
	if !errors.Is(err, command.ErrUnsupported) {
		t.Errorf("erreur = %v, attendu command.ErrUnsupported", err)
	}
}

func TestDigestAuthentication(t *testing.T) {
	f := &fakeModule{password: "secret"}

	if _, err := newTestClient(t, f, "secret").Fetch(context.Background()); err != nil {
		t.Errorf("bon mot de passe refusé: %v", err)
	}
	if _, err := newTestClient(t, f, "faux").Fetch(context.Background()); err == nil {
		t.Error("mauvais mot de passe accepté")
	}
	_, err := newTestClient(t, f, "").Fetch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "SHELLY_PASSWORD") {
		t.Errorf("sans mot de passe, attendu une erreur qui nomme SHELLY_PASSWORD, obtenu %v", err)
	}
}

func TestParseDeviceID(t *testing.T) {
	id, n, err := parseDeviceID("shellypro3-841fe88e5b68:switch:2")
	if err != nil || id != "shellypro3-841fe88e5b68" || n != 2 {
		t.Errorf("parseDeviceID = %q, %d, %v", id, n, err)
	}
	for _, bad := range []string{"shellypro3", ":switch:0", "x:switch:-1", "x:switch:a"} {
		if _, _, err := parseDeviceID(bad); err == nil {
			t.Errorf("%q aurait dû être refusé", bad)
		}
	}
}
