package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func testScene() SceneSpec {
	return SceneSpec{
		Name:            "Soirée",
		ShowOnDashboard: true,
		Steps: []SceneStep{
			{Type: "action", Command: "close", Targets: &SceneTargets{Devices: []string{}, Rooms: []string{"Salon"}, Kinds: []string{}}},
			{Type: "wait", WaitMinutes: 5},
			{Type: "action", Command: "setBrightness", Parameters: []any{30.0},
				Targets: &SceneTargets{Devices: []string{"l-1"}, Rooms: []string{}, Kinds: []string{}}},
		},
		Schedules: []SceneSchedule{{Enabled: true, Days: []int{1, 2, 3, 4, 5}, At: "time", Time: "19:30"}},
	}
}

func TestSceneLifecycle(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	created, err := s.CreateScene(ctx, testScene())
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	if created.ID == 0 || created.Name != "Soirée" || len(created.Steps) != 3 || created.Steps[2].Parameters[0] != 30.0 {
		t.Fatalf("scène relue: %+v", created)
	}
	if created.Steps[1].WaitMinutes != 5 || created.Schedules[0].Time != "19:30" || created.LastStatus != "" {
		t.Errorf("détails relus: %+v", created)
	}

	at := time.Now().UTC().Truncate(time.Second)
	if err := s.StartSceneRun(ctx, created.ID, "manual", at); err != nil {
		t.Fatalf("StartSceneRun: %v", err)
	}

	spec := testScene()
	spec.Name = "Soirée calme"
	updated, err := s.UpdateScene(ctx, created.ID, spec)
	if err != nil {
		t.Fatalf("UpdateScene: %v", err)
	}
	// Modifier la scène ne doit pas effacer l'exécution en cours.
	if updated.Name != "Soirée calme" || updated.LastStatus != SceneRunning || updated.LastRunAt == nil {
		t.Errorf("après modification: %+v", updated)
	}

	// Un arrêt du service pendant l'exécution la laisse « running » : le
	// démarrage suivant la marque interrompue.
	if n, err := s.InterruptRunningScenes(ctx); err != nil || n != 1 {
		t.Errorf("InterruptRunningScenes = %d, %v", n, err)
	}
	if got, _ := s.GetScene(ctx, created.ID); got.LastStatus != SceneInterrupted {
		t.Errorf("statut = %q, attendu interrupted", got.LastStatus)
	}

	all, err := s.ListScenes(ctx)
	if err != nil || len(all) != 1 {
		t.Fatalf("ListScenes: %d, %v", len(all), err)
	}

	if err := s.DeleteScene(ctx, created.ID); err != nil {
		t.Fatalf("DeleteScene: %v", err)
	}
	if _, err := s.GetScene(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("après suppression: %v", err)
	}
	if err := s.DeleteScene(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("double suppression: %v", err)
	}
}
