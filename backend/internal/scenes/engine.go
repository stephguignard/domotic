// Package scenes exécute les scènes : des suites d'actions et d'attentes,
// lancées à la main ou par leurs horaires.
//
// Le moteur tient dans une goroutine qui dort jusqu'à la prochaine échéance.
// Chaque exécution tourne dans sa propre goroutine ; relancer une scène en
// cours annule l'exécution précédente — le dernier ordre donné l'emporte.
package scenes

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/stephguignard/domotic/internal/command"
	"github.com/stephguignard/domotic/internal/control"
	"github.com/stephguignard/domotic/internal/store"
)

// Déclencheurs d'une exécution.
const (
	TriggerManual   = "manual"
	TriggerSchedule = "schedule"
)

// catchUpWindow borne le retard toléré sur une échéance : manquée de moins de
// cinq minutes (redémarrage, mise en veille), elle est rattrapée ; au-delà,
// ignorée. Rallumer les lumières à 3 h du matin après une coupure serait pire
// que de ne rien faire.
const catchUpWindow = 5 * time.Minute

// maxSleep borne l'attente entre deux calculs d'échéance, pour se recaler
// après un ajustement de l'horloge système.
const maxSleep = time.Hour

// ErrNotStarted signale un lancement demandé avant le démarrage du moteur.
var ErrNotStarted = errors.New("moteur de scènes non démarré")

// Engine planifie et exécute les scènes.
type Engine struct {
	store *store.Store
	ctl   *control.Controller
	place Place
	log   *slog.Logger

	// retryDelay espace une action en échec de sa relance, minute est
	// l'unité des attentes ; variables pour les tests.
	retryDelay time.Duration
	minute     time.Duration
	now        func() time.Time

	reload chan struct{}

	mu   sync.Mutex
	root context.Context
	runs map[int64]*execution
	wg   sync.WaitGroup
}

// execution est une exécution en cours ; token la distingue d'une relance.
type execution struct {
	cancel context.CancelFunc
	token  int
}

// New construit un moteur ; Run le démarre.
func New(st *store.Store, ctl *control.Controller, place Place, log *slog.Logger) *Engine {
	return &Engine{
		store:      st,
		ctl:        ctl,
		place:      place,
		log:        log,
		retryDelay: 30 * time.Second,
		minute:     time.Minute,
		now:        time.Now,
		reload:     make(chan struct{}, 1),
		runs:       map[int64]*execution{},
	}
}

// Place retourne le lieu du moteur, pour valider les horaires.
func (e *Engine) Place() Place {
	return e.place
}

// Reload signale un changement de scènes : les échéances sont recalculées.
func (e *Engine) Reload() {
	select {
	case e.reload <- struct{}{}:
	default:
	}
}

// Run fait tourner le planificateur jusqu'à l'annulation de ctx, puis attend la
// fin des exécutions en cours, marquées interrompues.
func (e *Engine) Run(ctx context.Context) {
	e.mu.Lock()
	e.root = ctx
	e.mu.Unlock()

	// Une exécution coupée par un arrêt brutal est restée « running ».
	if n, err := e.store.InterruptRunningScenes(ctx); err != nil {
		e.log.Error("scènes interrompues", "error", err)
	} else if n > 0 {
		e.log.Warn("exécutions de scènes interrompues par l'arrêt précédent", "scenes", n)
	}

	now := e.now()
	e.catchUp(ctx, now)
	cursor := now

	for {
		next, due := e.nextDue(ctx, cursor)

		sleep := maxSleep
		if !next.IsZero() {
			sleep = min(max(next.Sub(e.now()), 0), maxSleep)
		}
		timer := time.NewTimer(sleep)

		select {
		case <-ctx.Done():
			timer.Stop()
			e.wg.Wait()
			return
		case <-e.reload:
			timer.Stop()
			// Ne pas rejouer ce qui vient de partir (cursor), ni sauter une
			// échéance tombée à l'instant même, pas encore traitée.
			if floor := e.now().Add(-time.Second); floor.After(cursor) {
				cursor = floor
			}
		case <-timer.C:
			now := e.now()
			if next.IsZero() || now.Before(next) {
				continue // réveil de sécurité, rien d'échu
			}
			for _, id := range due {
				if late := now.Sub(next); late > catchUpWindow {
					e.log.Warn("échéance de scène manquée, ignorée", "scene", id, "due", next, "late", late)
					continue
				}
				if err := e.Trigger(ctx, id, TriggerSchedule); err != nil {
					e.log.Error("lancement planifié d'une scène", "scene", id, "error", err)
				}
			}
			cursor = next
		}
	}
}

// nextDue retourne la prochaine échéance postérieure à cursor et les scènes
// qui la partagent.
func (e *Engine) nextDue(ctx context.Context, cursor time.Time) (time.Time, []int64) {
	scenes, err := e.store.ListScenes(ctx)
	if err != nil {
		e.log.Error("lecture des scènes", "error", err)
		return time.Time{}, nil
	}
	var (
		next time.Time
		due  []int64
	)
	for _, sc := range scenes {
		t, ok := e.place.NextRun(sc, cursor)
		switch {
		case !ok:
		case next.IsZero() || t.Before(next):
			next, due = t, []int64{sc.ID}
		case t.Equal(next):
			due = append(due, sc.ID)
		}
	}
	return next, due
}

// catchUp lance les scènes dont une échéance vient de passer sans exécution :
// le service était arrêté à ce moment-là.
func (e *Engine) catchUp(ctx context.Context, now time.Time) {
	scenes, err := e.store.ListScenes(ctx)
	if err != nil {
		e.log.Error("lecture des scènes", "error", err)
		return
	}
	for _, sc := range scenes {
		if due, ok := shouldCatchUp(e.place, sc, now); ok {
			e.log.Info("rattrapage d'une échéance manquée", "scene", sc.Name, "due", due)
			if err := e.Trigger(ctx, sc.ID, TriggerSchedule); err != nil {
				e.log.Error("rattrapage d'une scène", "scene", sc.ID, "error", err)
			}
		}
	}
}

// shouldCatchUp décide si une échéance récente de la scène a été manquée.
func shouldCatchUp(place Place, sc store.Scene, now time.Time) (time.Time, bool) {
	due, ok := place.lastDue(sc, now)
	if !ok || now.Sub(due) > catchUpWindow {
		return time.Time{}, false
	}
	if sc.LastRunAt != nil && !sc.LastRunAt.Before(due) {
		return time.Time{}, false // déjà exécutée depuis
	}
	return due, true
}

// Trigger lance une scène. Une exécution en cours de la même scène est
// annulée : ses attentes restantes sont abandonnées.
func (e *Engine) Trigger(ctx context.Context, id int64, trigger string) error {
	sc, err := e.store.GetScene(ctx, id)
	if err != nil {
		return err
	}

	e.mu.Lock()
	if e.root == nil || e.root.Err() != nil {
		e.mu.Unlock()
		return ErrNotStarted
	}
	token := 1
	if prev := e.runs[id]; prev != nil {
		prev.cancel()
		token = prev.token + 1
		e.log.Info("scène relancée, exécution précédente annulée", "scene", sc.Name)
	}
	runCtx, cancel := context.WithCancel(e.root)
	e.runs[id] = &execution{cancel: cancel, token: token}
	e.wg.Add(1)
	e.mu.Unlock()

	if err := e.store.StartSceneRun(ctx, id, trigger, e.now().UTC()); err != nil {
		e.log.Error("début d'exécution de scène", "scene", id, "error", err)
	}

	origin := control.Origin{Kind: store.OriginSceneManual, SceneID: sc.ID, SceneName: sc.Name}
	if trigger == TriggerSchedule {
		origin.Kind = store.OriginSceneSchedule
	}

	go func() {
		defer e.wg.Done()
		defer cancel()
		status := e.execute(runCtx, sc, origin)
		e.finish(sc, token, status)
	}()
	return nil
}

// finish consigne l'issue d'une exécution, sauf si une relance l'a remplacée.
func (e *Engine) finish(sc store.Scene, token int, status string) {
	e.mu.Lock()
	current := e.runs[sc.ID]
	superseded := current == nil || current.token != token
	if !superseded {
		delete(e.runs, sc.ID)
	}
	shuttingDown := e.root.Err() != nil
	e.mu.Unlock()

	if superseded {
		return // la relance écrira sa propre issue
	}
	if shuttingDown && status == "" {
		status = store.SceneInterrupted
	}
	if status == "" {
		return
	}
	// Contexte détaché : à l'arrêt du service, l'issue doit encore être écrite.
	if err := e.store.FinishSceneRun(context.Background(), sc.ID, status); err != nil {
		e.log.Error("fin d'exécution de scène", "scene", sc.ID, "error", err)
	}
	e.log.Info("scène exécutée", "scene", sc.Name, "status", status)
}

// execute déroule les étapes. Elle retourne l'issue, ou une chaîne vide si
// l'exécution a été annulée avant son terme.
func (e *Engine) execute(ctx context.Context, sc store.Scene, origin control.Origin) string {
	var (
		mu       sync.Mutex
		ok, fail int
		retries  sync.WaitGroup
	)
	count := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		if err == nil {
			ok++
		} else {
			fail++
		}
	}

	for _, step := range sc.Steps {
		if ctx.Err() != nil {
			break
		}
		switch step.Type {
		case "wait":
			select {
			case <-ctx.Done():
			case <-time.After(time.Duration(step.WaitMinutes) * e.minute):
			}

		case "action":
			devices, err := e.store.ListDevices(ctx, store.DeviceFilter{})
			if err != nil {
				e.log.Error("lecture des équipements", "scene", sc.Name, "error", err)
				continue
			}
			targets, _ := Resolve(step, devices)
			for _, d := range targets {
				_, err := e.ctl.Send(ctx, d, step.Command, step.Parameters, origin)
				if err == nil || !retryable(err) || ctx.Err() != nil {
					count(err)
					continue
				}
				// La suite de la scène n'attend pas la relance.
				retries.Add(1)
				go func(d store.Device, step store.SceneStep) {
					defer retries.Done()
					select {
					case <-ctx.Done():
						count(ctx.Err())
					case <-time.After(e.retryDelay):
						_, err := e.ctl.Send(ctx, d, step.Command, step.Parameters, origin)
						count(err)
					}
				}(d, step)
			}
		}
	}
	retries.Wait()

	if ctx.Err() != nil {
		return ""
	}
	switch {
	case ok+fail == 0:
		e.log.Warn("scène sans équipement à piloter", "scene", sc.Name)
		return store.SceneFailed
	case fail == 0:
		return store.SceneSuccess
	case ok == 0:
		return store.SceneFailed
	default:
		return store.ScenePartial
	}
}

// retryable distingue un échec passager de la source (box injoignable) d'un
// refus qu'une relance ne changerait pas.
func retryable(err error) bool {
	return !errors.Is(err, command.ErrUnsupported) &&
		!errors.Is(err, control.ErrNotControllable) &&
		!errors.Is(err, control.ErrNotConfigured)
}

// SceneView est une scène telle que l'API la présente.
type SceneView struct {
	store.Scene
	NextRunAt     *time.Time `json:"next_run_at,omitempty" doc:"Prochain déclenchement planifié"`
	Running       bool       `json:"running" doc:"La scène est-elle en cours d'exécution ?"`
	TouchesRelays bool       `json:"touches_relays" doc:"La scène pilote-t-elle un relais ? Son lancement manuel demande alors confirmation"`
}

// Describe complète une scène de ce que seul le moteur sait.
func (e *Engine) Describe(sc store.Scene, devices []store.Device) SceneView {
	v := SceneView{Scene: sc, TouchesRelays: touchesRelays(sc, devices)}
	if t, ok := e.place.NextRun(sc, e.now()); ok {
		v.NextRunAt = &t
	}
	e.mu.Lock()
	v.Running = e.runs[sc.ID] != nil
	e.mu.Unlock()
	return v
}
