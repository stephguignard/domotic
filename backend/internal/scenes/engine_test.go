package scenes

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stephguignard/domotic/internal/command"
	"github.com/stephguignard/domotic/internal/control"
	"github.com/stephguignard/domotic/internal/store"
)

// fakeSource enregistre les commandes reçues et échoue à la demande.
type fakeSource struct {
	mu       sync.Mutex
	calls    []string
	failures map[string]int // équipement → nombre d'échecs avant succès (-1 : toujours)
}

func (f *fakeSource) Execute(_ context.Context, id, cmd string, _ []any) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, id+":"+cmd)
	switch n := f.failures[id]; {
	case n < 0:
		return "", errors.New("box injoignable")
	case n > 0:
		f.failures[id] = n - 1
		return "", errors.New("box injoignable")
	}
	return "", nil
}

func (f *fakeSource) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

var _ command.Commander = (*fakeSource)(nil)

type fixture struct {
	store  *store.Store
	source *fakeSource
	engine *Engine
	cancel context.CancelFunc
	done   chan struct{}
}

// newFixture démarre un moteur sur une base jetable, avec un salon composé
// d'une lampe Hue, d'une prise Hue et d'un volet TaHoma.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	now := time.Now().UTC()
	if err := st.UpsertDevices(ctx, []store.Device{
		{ID: "lamp", Source: "hue", Name: "Lampe", Kind: "light", Room: "Salon", State: `{"on":false,"brightness":10}`, Reachable: true, UpdatedAt: now},
		{ID: "plug", Source: "hue", Name: "Prise", Kind: "light", Room: "Salon", State: `{"on":false}`, Reachable: true, UpdatedAt: now},
		{ID: "shutter", Source: "tahoma", Name: "Volet", Kind: "shutter", Room: "Salon", State: `{}`, Reachable: true, UpdatedAt: now},
	}); err != nil {
		t.Fatalf("UpsertDevices: %v", err)
	}

	src := &fakeSource{failures: map[string]int{}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctl := control.New(st, map[string]command.Commander{"hue": src, "tahoma": src}, nil, log)
	e := New(st, ctl, Place{TimeZone: time.UTC}, log)
	e.retryDelay = 20 * time.Millisecond
	e.minute = 50 * time.Millisecond

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { e.Run(runCtx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })

	// Attendre que le moteur ait pris son contexte.
	deadline := time.Now().Add(time.Second)
	for {
		e.mu.Lock()
		started := e.root != nil
		e.mu.Unlock()
		if started || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	return &fixture{store: st, source: src, engine: e, cancel: cancel, done: done}
}

func action(cmd string, params []any, devices, rooms []string) store.SceneStep {
	return store.SceneStep{Type: "action", Command: cmd, Parameters: params,
		Targets: &store.SceneTargets{Devices: devices, Rooms: rooms, Kinds: []string{}}}
}

func (f *fixture) create(t *testing.T, steps ...store.SceneStep) store.Scene {
	t.Helper()
	sc, err := f.store.CreateScene(context.Background(), store.SceneSpec{Name: "Test", Steps: steps, Schedules: []store.SceneSchedule{}})
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	return sc
}

// waitStatus attend l'issue finale de la dernière exécution.
func (f *fixture) waitStatus(t *testing.T, id int64) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		sc, err := f.store.GetScene(context.Background(), id)
		if err == nil && sc.LastStatus != "" && sc.LastStatus != store.SceneRunning {
			return sc.LastStatus
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("la scène ne s'est pas terminée")
	return ""
}

func TestSceneTargetsCompatibleDevicesOnly(t *testing.T) {
	f := newFixture(t)
	sc := f.create(t, action("setBrightness", []any{40.0}, nil, []string{"Salon"}))

	if err := f.engine.Trigger(context.Background(), sc.ID, TriggerManual); err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if got := f.waitStatus(t, sc.ID); got != store.SceneSuccess {
		t.Errorf("issue = %q", got)
	}
	// La prise n'a pas de luminosité, le volet n'est pas une lumière.
	if calls := f.source.Calls(); len(calls) != 1 || calls[0] != "lamp:setBrightness" {
		t.Errorf("commandes = %v", calls)
	}

	entries, _ := f.store.ListCommands(context.Background(), store.CommandLogFilter{SceneID: sc.ID})
	if len(entries) != 1 || entries[0].Origin != store.OriginSceneManual || entries[0].SceneName != "Test" {
		t.Errorf("historique = %+v", entries)
	}
}

func TestSceneRetriesFailedActionOnce(t *testing.T) {
	f := newFixture(t)
	f.source.failures["lamp"] = 1
	sc := f.create(t,
		action("on", nil, []string{"lamp"}, nil),
		action("close", nil, []string{"shutter"}, nil),
	)

	if err := f.engine.Trigger(context.Background(), sc.ID, TriggerSchedule); err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if got := f.waitStatus(t, sc.ID); got != store.SceneSuccess {
		t.Errorf("issue = %q, attendu success après relance", got)
	}
	// Le volet n'attend pas la relance de la lampe.
	calls := f.source.Calls()
	want := []string{"lamp:on", "shutter:close", "lamp:on"}
	if len(calls) != 3 || calls[0] != want[0] || calls[1] != want[1] || calls[2] != want[2] {
		t.Errorf("commandes = %v, attendu %v", calls, want)
	}
	entries, _ := f.store.ListCommands(context.Background(), store.CommandLogFilter{SceneID: sc.ID})
	if len(entries) != 3 || entries[0].Origin != store.OriginSceneSchedule {
		t.Errorf("historique = %+v", entries)
	}
}

func TestScenePartialWhenAnActionKeepsFailing(t *testing.T) {
	f := newFixture(t)
	f.source.failures["lamp"] = -1
	sc := f.create(t, action("on", nil, []string{"lamp"}, nil), action("open", nil, nil, []string{"Salon"}))

	_ = f.engine.Trigger(context.Background(), sc.ID, TriggerManual)
	if got := f.waitStatus(t, sc.ID); got != store.ScenePartial {
		t.Errorf("issue = %q, attendu partial", got)
	}
}

func TestSceneWithoutReachableTargetFails(t *testing.T) {
	f := newFixture(t)
	sc := f.create(t, action("setColor", []any{"#ff0000"}, []string{"plug", "disparu"}, nil))

	_ = f.engine.Trigger(context.Background(), sc.ID, TriggerManual)
	if got := f.waitStatus(t, sc.ID); got != store.SceneFailed {
		t.Errorf("issue = %q, attendu failed", got)
	}
	if calls := f.source.Calls(); len(calls) != 0 {
		t.Errorf("commandes = %v, attendu aucune", calls)
	}
}

func TestRelaunchCancelsRunningExecution(t *testing.T) {
	f := newFixture(t)
	sc := f.create(t,
		action("on", nil, []string{"lamp"}, nil),
		store.SceneStep{Type: "wait", WaitMinutes: 4}, // 200 ms avec l'unité de test
		action("off", nil, []string{"lamp"}, nil),
	)

	ctx := context.Background()
	_ = f.engine.Trigger(ctx, sc.ID, TriggerManual)
	time.Sleep(50 * time.Millisecond) // pendant l'attente
	_ = f.engine.Trigger(ctx, sc.ID, TriggerManual)

	if got := f.waitStatus(t, sc.ID); got != store.SceneSuccess {
		t.Errorf("issue = %q", got)
	}
	// La première exécution s'arrête à l'attente : son « off » ne part jamais.
	calls := f.source.Calls()
	want := []string{"lamp:on", "lamp:on", "lamp:off"}
	if len(calls) != 3 || calls[0] != want[0] || calls[1] != want[1] || calls[2] != want[2] {
		t.Errorf("commandes = %v, attendu %v", calls, want)
	}
}

func TestShutdownMarksRunningSceneInterrupted(t *testing.T) {
	f := newFixture(t)
	sc := f.create(t, action("on", nil, []string{"lamp"}, nil), store.SceneStep{Type: "wait", WaitMinutes: 100})

	_ = f.engine.Trigger(context.Background(), sc.ID, TriggerManual)
	time.Sleep(30 * time.Millisecond)
	f.cancel()
	<-f.done

	got, _ := f.store.GetScene(context.Background(), sc.ID)
	if got.LastStatus != store.SceneInterrupted {
		t.Errorf("issue = %q, attendu interrupted", got.LastStatus)
	}
	if err := f.engine.Trigger(context.Background(), sc.ID, TriggerManual); !errors.Is(err, ErrNotStarted) {
		t.Errorf("lancement après arrêt : %v", err)
	}
}
