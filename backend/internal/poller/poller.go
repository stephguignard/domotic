// Package poller orchestre les boucles de rafraîchissement des sources
// externes et consolide leurs données dans la base locale.
//
// Le frontend ne parle jamais à Netatmo ni à TaHoma : il lit l'état consolidé.
// Cela découple l'interface des latences et des quotas des API amont, et
// permet au service de continuer à répondre quand une source est indisponible.
package poller

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"maps"
	"sync"
	"time"

	"github.com/stephguignard/domotic/internal/config"
	"github.com/stephguignard/domotic/internal/hue"
	"github.com/stephguignard/domotic/internal/netatmo"
	"github.com/stephguignard/domotic/internal/shelly"
	"github.com/stephguignard/domotic/internal/store"
	"github.com/stephguignard/domotic/internal/tahoma"
)

// retentionPeriod borne l'historique des relevés. Le NAS a un gigaoctet de RAM
// et un disque partagé : laisser la table croître indéfiniment finirait par
// peser sans bénéfice pour un usage domestique.
const retentionPeriod = 90 * 24 * time.Hour

// commandRetention borne l'historique des actions. Quelques lignes par jour
// au plus : une année reste négligeable et permet de retrouver un réglage
// saisonnier.
const commandRetention = 365 * 24 * time.Hour

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
	shelly  *shelly.Client
	hue     *hue.Client
	log     *slog.Logger

	mu     sync.RWMutex
	status map[string]*SourceStatus

	// nudges réveillent la boucle d'une source avant son prochain tour, par
	// exemple juste après une commande. Tampon d'un élément : plusieurs
	// demandes rapprochées ne valent qu'un rafraîchissement.
	nudges map[string]chan struct{}
}

// Clients regroupe les clients des sources. Un client est nil quand la source
// correspondante n'est pas configurée.
type Clients struct {
	Netatmo *netatmo.Client
	Tahoma  *tahoma.Client
	Shelly  *shelly.Client
	Hue     *hue.Client
}

// New construit un poller.
func New(cfg *config.Config, st *store.Store, c Clients, log *slog.Logger) *Poller {
	p := &Poller{
		cfg:     cfg,
		store:   st,
		netatmo: c.Netatmo,
		tahoma:  c.Tahoma,
		shelly:  c.Shelly,
		hue:     c.Hue,
		log:     log,
		status: map[string]*SourceStatus{
			"netatmo": {Enabled: cfg.Netatmo.Enabled()},
			"tahoma":  {Enabled: cfg.Tahoma.Enabled()},
			"shelly":  {Enabled: cfg.Shelly.Enabled()},
			"hue":     {Enabled: cfg.Hue.Enabled()},
		},
		nudges: map[string]chan struct{}{
			"shelly": make(chan struct{}, 1),
			"hue":    make(chan struct{}, 1),
		},
	}
	if c.Tahoma != nil {
		p.events = tahoma.NewEventListener(c.Tahoma)
	}
	return p
}

// Nudge demande un rafraîchissement immédiat d'une source. Sans effet pour une
// source dont l'état arrive déjà par un flux d'événements.
func (p *Poller) Nudge(source string) {
	ch := p.nudges[source]
	if ch == nil {
		return
	}
	select {
	case ch <- struct{}{}:
	default: // un rafraîchissement est déjà demandé
	}
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
	if p.shelly != nil {
		wg.Go(func() { p.runShelly(ctx) })
	}
	if p.hue != nil {
		wg.Go(func() { p.runHueInventory(ctx) })
		wg.Go(func() { p.runHueEvents(ctx) })
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
	p.tick(ctx, p.cfg.Netatmo.PollInterval, nil, func(ctx context.Context) {
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
	p.tick(ctx, 15*time.Minute, nil, func(ctx context.Context) {
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

// runShelly relève l'état des modules Shelly à intervalle court. Les modules
// proposent aussi un WebSocket de notifications, mais il demanderait une
// dépendance de plus pour un gain de quelques secondes : sur le LAN, un relevé
// toutes les cinq secondes ne coûte presque rien, et une commande déclenche de
// toute façon un relevé immédiat (Nudge).
func (p *Poller) runShelly(ctx context.Context) {
	// Compteur d'échecs consécutifs : à cet intervalle, un module débranché
	// produirait sinon une ligne de journal toutes les cinq secondes.
	failures := 0

	p.tick(ctx, p.cfg.Shelly.PollInterval, p.nudges["shelly"], func(ctx context.Context) {
		err := p.refreshShelly(ctx)
		p.record("shelly", err)

		switch {
		case err != nil && ctx.Err() == nil:
			failures++
			if failures == 1 || failures%60 == 0 {
				p.log.Error("relevé Shelly", "error", err, "consecutive_failures", failures)
			}
		case err == nil && failures > 0:
			p.log.Info("relevé Shelly rétabli", "after_failures", failures)
			failures = 0
		}
	})
}

func (p *Poller) refreshShelly(ctx context.Context) error {
	snap, fetchErr := p.shelly.Fetch(ctx)

	// Les modules qui ont répondu sont enregistrés même si d'autres ont
	// échoué : une panne isolée ne doit pas figer toute la source.
	if err := p.store.UpsertDevices(ctx, snap.Devices); err != nil {
		return err
	}
	for _, id := range snap.Unreachable {
		if err := p.store.MarkUnreachable(ctx, "shelly", shelly.ModulePrefix(id)); err != nil {
			return err
		}
	}
	return fetchErr
}

// runHueInventory recharge les lumières du pont, leurs pièces et leur
// joignabilité. Comme pour TaHoma, les changements d'état arrivent par le flux
// d'événements ; ce rechargement rattrape les ajouts, renommages, changements
// de pièce, et ce que le flux aurait manqué pendant une coupure.
func (p *Poller) runHueInventory(ctx context.Context) {
	p.tick(ctx, 15*time.Minute, p.nudges["hue"], func(ctx context.Context) {
		devices, err := p.hue.FetchDevices(ctx)
		if err == nil {
			err = p.store.UpsertDevices(ctx, devices)
		}
		p.record("hue", err)

		if err != nil {
			if ctx.Err() == nil {
				p.log.Error("rafraîchissement de l'inventaire Hue", "error", err)
			}
			return
		}
		p.log.Debug("inventaire Hue rafraîchi", "devices", len(devices))
	})
}

// runHueEvents suit le flux d'événements du pont et le rouvre quand il tombe.
func (p *Poller) runHueEvents(ctx context.Context) {
	failures := 0
	for {
		connected := false
		err := p.hue.Stream(ctx, func(events []hue.Event) {
			if !connected {
				connected = true
				if failures > 0 {
					p.log.Info("flux d'événements Hue rétabli", "after_failures", failures)
					failures = 0
				}
			}
			p.applyHueEvents(ctx, events)
		})
		if ctx.Err() != nil {
			return
		}

		// Des événements ont pu être perdus pendant la coupure : relire
		// l'inventaire dès que possible.
		p.Nudge("hue")

		if errors.Is(err, hue.ErrIdle) {
			p.log.Debug("flux d'événements Hue rouvert après inactivité")
			continue
		}

		failures++
		if failures == 1 || failures%30 == 0 {
			p.log.Warn("flux d'événements Hue interrompu", "error", err, "consecutive_failures", failures)
		}
		// Palier de repli : jusqu'à une minute entre deux tentatives.
		backoff := min(time.Duration(failures)*2*time.Second, time.Minute)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
	}
}

func (p *Poller) applyHueEvents(ctx context.Context, events []hue.Event) {
	now := time.Now().UTC()

	for _, e := range events {
		if e.Type != "update" {
			// Ajout ou suppression : l'inventaire sait reconstruire les pièces
			// et les noms, que l'événement ne porte pas.
			p.Nudge("hue")
			continue
		}
		for _, r := range e.Data {
			switch r.Type {
			case "light":
				p.mergeHueState(ctx, r, now)
			case "zigbee_connectivity", "room", "device":
				// La joignabilité et les pièces se rattachent aux appareils,
				// pas aux lumières : l'inventaire fait la correspondance.
				p.Nudge("hue")
			}
		}
	}
}

// mergeHueState fusionne un état partiel dans l'état enregistré : un événement
// « update » ne porte que les grandeurs qui ont changé.
func (p *Poller) mergeHueState(ctx context.Context, r hue.EventResource, at time.Time) {
	changes := r.LightState()
	if len(changes) == 0 {
		return // changement de couleur ou d'effet, non suivi
	}

	d, err := p.store.GetDevice(ctx, r.ID)
	if errors.Is(err, store.ErrNotFound) {
		p.Nudge("hue") // lumière apparue depuis le dernier inventaire
		return
	}
	if err != nil {
		p.log.Error("lecture d'une lumière Hue", "device", r.ID, "error", err)
		return
	}

	state := map[string]any{}
	if err := json.Unmarshal([]byte(d.State), &state); err != nil {
		state = map[string]any{} // état illisible : repartir des seules nouveautés
	}
	maps.Copy(state, changes)

	encoded, err := json.Marshal(state)
	if err != nil {
		p.log.Error("sérialisation de l'état Hue", "device", r.ID, "error", err)
		return
	}
	// Un événement prouve que la lumière répond.
	if err := p.store.UpdateDeviceState(ctx, r.ID, string(encoded), true, at); err != nil {
		p.log.Error("application d'un événement Hue", "device", r.ID, "error", err)
	}
}

// runRetention purge quotidiennement les relevés et les actions trop anciens.
func (p *Poller) runRetention(ctx context.Context) {
	p.tick(ctx, 24*time.Hour, nil, func(ctx context.Context) {
		cutoff := time.Now().UTC().Add(-retentionPeriod)
		n, err := p.store.PurgeMeasurementsBefore(ctx, cutoff)
		if err != nil {
			p.log.Error("purge des relevés", "error", err)
			return
		}
		if n > 0 {
			p.log.Info("relevés purgés", "rows", n, "before", cutoff)
		}

		cutoff = time.Now().UTC().Add(-commandRetention)
		n, err = p.store.PurgeCommandsBefore(ctx, cutoff)
		if err != nil {
			p.log.Error("purge de l'historique des actions", "error", err)
			return
		}
		if n > 0 {
			p.log.Info("historique des actions purgé", "rows", n, "before", cutoff)
		}
	})
}

// tick exécute fn immédiatement puis à chaque intervalle, jusqu'à annulation.
// Un signal sur nudge avance le tour suivant ; nil désactive ce réveil.
func (p *Poller) tick(ctx context.Context, interval time.Duration, nudge <-chan struct{}, fn func(context.Context)) {
	fn(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-nudge:
			// Repartir d'un intervalle plein : le tour anticipé remplace le
			// prochain plutôt que de s'y ajouter.
			ticker.Reset(interval)
		}
		fn(ctx)
	}
}
