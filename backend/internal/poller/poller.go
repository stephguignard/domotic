// Package poller orchestre les boucles de rafraîchissement des sources
// externes et consolide leurs données dans la base locale.
//
// Le frontend ne parle jamais à Netatmo ni à TaHoma : il lit l'état consolidé.
// Cela découple l'interface des latences et des quotas des API amont, et
// permet au service de continuer à répondre quand une source est indisponible.
package poller

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/stephguignard/domotic/internal/config"
	"github.com/stephguignard/domotic/internal/netatmo"
	"github.com/stephguignard/domotic/internal/store"
	"github.com/stephguignard/domotic/internal/tahoma"
)

// retentionPeriod borne l'historique des relevés. Le NAS a un gigaoctet de RAM
// et un disque partagé : laisser la table croître indéfiniment finirait par
// peser sans bénéfice pour un usage domestique.
const retentionPeriod = 90 * 24 * time.Hour

// SourceStatus résume l'état d'une source pour l'endpoint de santé.
//
// LastSuccess est un pointeur : encoding/json ne considère pas un time.Time
// zéro comme vide, et l'API exposerait sinon une date de l'an 1 tant qu'aucun
// rafraîchissement n'a eu lieu.
type SourceStatus struct {
	Enabled     bool       `json:"enabled" doc:"La source est-elle configurée ?"`
	Healthy     bool       `json:"healthy" doc:"Le dernier rafraîchissement a-t-il réussi ?"`
	LastSuccess *time.Time `json:"last_success,omitempty" doc:"Dernier rafraîchissement réussi"`
	LastError   string     `json:"last_error,omitempty" doc:"Message de la dernière erreur rencontrée"`
}

// Poller fait tourner les boucles de rafraîchissement.
type Poller struct {
	cfg     *config.Config
	store   *store.Store
	netatmo *netatmo.Client
	tahoma  *tahoma.Client
	events  *tahoma.EventListener
	log     *slog.Logger

	mu     sync.RWMutex
	status map[string]*SourceStatus
}

// New construit un poller. Les clients peuvent être nil si la source
// correspondante n'est pas configurée.
func New(cfg *config.Config, st *store.Store, nc *netatmo.Client, tc *tahoma.Client, log *slog.Logger) *Poller {
	p := &Poller{
		cfg:     cfg,
		store:   st,
		netatmo: nc,
		tahoma:  tc,
		log:     log,
		status: map[string]*SourceStatus{
			"netatmo": {Enabled: cfg.Netatmo.Enabled()},
			"tahoma":  {Enabled: cfg.Tahoma.Enabled()},
		},
	}
	if tc != nil {
		p.events = tahoma.NewEventListener(tc)
	}
	return p
}

// Status retourne une copie de l'état des sources.
func (p *Poller) Status() map[string]SourceStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	out := make(map[string]SourceStatus, len(p.status))
	for k, v := range p.status {
		out[k] = *v
	}
	return out
}

func (p *Poller) record(source string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	s := p.status[source]
	if s == nil {
		return
	}
	if err != nil {
		s.Healthy = false
		s.LastError = err.Error()
		return
	}
	now := time.Now().UTC()
	s.Healthy = true
	s.LastError = ""
	s.LastSuccess = &now
}

// Run démarre toutes les boucles configurées et rend la main quand le contexte
// est annulé et que les boucles se sont arrêtées.
func (p *Poller) Run(ctx context.Context) {
	var wg sync.WaitGroup

	if p.netatmo != nil {
		wg.Go(func() { p.runNetatmo(ctx) })
	}
	if p.tahoma != nil {
		wg.Go(func() { p.runTahomaSetup(ctx) })
		wg.Go(func() { p.runTahomaEvents(ctx) })
	}
	wg.Go(func() { p.runRetention(ctx) })

	wg.Wait()
	p.shutdown()
}

// shutdown libère les ressources détenues côté box.
func (p *Poller) shutdown() {
	if p.events == nil {
		return
	}
	// Le contexte principal est déjà annulé : en créer un neuf, borné, pour
	// libérer le listener — la box n'en tolère qu'un nombre limité.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.events.Unregister(ctx); err != nil {
		p.log.Warn("libération du listener TaHoma", "error", err)
	}
}

// runNetatmo rafraîchit les données Netatmo à intervalle régulier.
func (p *Poller) runNetatmo(ctx context.Context) {
	p.tick(ctx, p.cfg.Netatmo.PollInterval, func(ctx context.Context) {
		err := p.refreshNetatmo(ctx)
		p.record("netatmo", err)

		switch {
		case errors.Is(err, netatmo.ErrNotAuthenticated):
			// Attendu tant que le flux OAuth2 n'a pas été parcouru : inutile
			// d'alarmer à chaque tour de boucle.
			p.log.Debug("Netatmo non authentifié, rafraîchissement ignoré")
		case err != nil:
			p.log.Error("rafraîchissement Netatmo", "error", err)
		}
	})
}

func (p *Poller) refreshNetatmo(ctx context.Context) error {
	devices, measurements, err := p.netatmo.FetchWeather(ctx)
	if err != nil {
		return err
	}

	// Les caméras sont facultatives : leur absence ou un scope manquant ne
	// doit pas invalider les données météo déjà récupérées.
	cameras, err := p.netatmo.FetchSecurity(ctx)
	if err != nil {
		p.log.Warn("récupération des caméras Netatmo", "error", err)
	} else {
		devices = append(devices, cameras...)
	}

	if err := p.store.UpsertDevices(ctx, devices); err != nil {
		return err
	}
	if err := p.store.InsertMeasurements(ctx, measurements); err != nil {
		return err
	}

	p.log.Debug("Netatmo rafraîchi", "devices", len(devices), "measurements", len(measurements))
	return nil
}

// runTahomaSetup recharge l'inventaire complet de la box. Les changements
// d'état arrivent par le flux d'événements ; ce rechargement périodique ne sert
// qu'à détecter les ajouts, suppressions et renommages d'équipements.
func (p *Poller) runTahomaSetup(ctx context.Context) {
	p.tick(ctx, 15*time.Minute, func(ctx context.Context) {
		devices, err := p.tahoma.FetchDevices(ctx)
		if err == nil {
			err = p.store.UpsertDevices(ctx, devices)
		}
		p.record("tahoma", err)

		if err != nil {
			p.log.Error("rafraîchissement de l'inventaire TaHoma", "error", err)
			return
		}
		p.log.Debug("inventaire TaHoma rafraîchi", "devices", len(devices))
	})
}

// runTahomaEvents consomme le flux d'événements de la box pour refléter les
// changements d'état en quasi temps réel.
func (p *Poller) runTahomaEvents(ctx context.Context) {
	ticker := time.NewTicker(p.cfg.Tahoma.EventInterval)
	defer ticker.Stop()

	// Compteur d'échecs consécutifs, pour espacer les tentatives quand la box
	// est injoignable plutôt que de marteler le réseau toutes les deux secondes.
	failures := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		events, err := p.events.Fetch(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			failures++
			if failures == 1 || failures%30 == 0 {
				p.log.Error("lecture des événements TaHoma", "error", err, "consecutive_failures", failures)
			}
			// Palier de repli : jusqu'à une minute entre deux tentatives.
			backoff := min(time.Duration(failures)*p.cfg.Tahoma.EventInterval, time.Minute)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			continue
		}

		if failures > 0 {
			p.log.Info("flux d'événements TaHoma rétabli", "after_failures", failures)
			failures = 0
		}

		p.applyEvents(ctx, events)
	}
}

func (p *Poller) applyEvents(ctx context.Context, events []tahoma.Event) {
	now := time.Now().UTC()

	for _, e := range events {
		if e.Name != "DeviceStateChangedEvent" || e.DeviceURL == "" || len(e.DeviceStates) == 0 {
			continue
		}

		state, err := e.StatesJSON()
		if err != nil {
			p.log.Warn("état d'événement illisible", "device", e.DeviceURL, "error", err)
			continue
		}

		err = p.store.UpdateDeviceState(ctx, e.DeviceURL, state, true, now)
		if errors.Is(err, store.ErrNotFound) {
			// Équipement ajouté depuis le dernier rechargement de l'inventaire :
			// il sera connu au prochain passage de runTahomaSetup.
			p.log.Debug("événement pour un équipement inconnu", "device", e.DeviceURL)
			continue
		}
		if err != nil {
			p.log.Error("application d'un événement TaHoma", "device", e.DeviceURL, "error", err)
		}
	}
}

// runRetention purge quotidiennement les relevés trop anciens.
func (p *Poller) runRetention(ctx context.Context) {
	p.tick(ctx, 24*time.Hour, func(ctx context.Context) {
		cutoff := time.Now().UTC().Add(-retentionPeriod)
		n, err := p.store.PurgeMeasurementsBefore(ctx, cutoff)
		if err != nil {
			p.log.Error("purge des relevés", "error", err)
			return
		}
		if n > 0 {
			p.log.Info("relevés purgés", "rows", n, "before", cutoff)
		}
	})
}

// tick exécute fn immédiatement puis à chaque intervalle, jusqu'à annulation.
func (p *Poller) tick(ctx context.Context, interval time.Duration, fn func(context.Context)) {
	fn(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fn(ctx)
		}
	}
}
