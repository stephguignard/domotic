// Package control pilote les équipements et consigne chaque action dans
// l'historique.
//
// C'est le point de passage unique des commandes : l'API y envoie celles de
// l'interface, le moteur de scènes les siennes. Tous deux obtiennent ainsi les
// mêmes règles (sources pilotables, relevé anticipé) et le même historique.
package control

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/stephguignard/domotic/internal/command"
	"github.com/stephguignard/domotic/internal/store"
)

var (
	// ErrNotControllable signale un équipement d'une source en lecture seule.
	ErrNotControllable = errors.New("source en lecture seule")
	// ErrNotConfigured signale une source pilotable mais non configurée.
	ErrNotConfigured = errors.New("intégration non configurée")
)

// controllable liste les sources dont les équipements se pilotent, qu'elles
// soient configurées ou non.
var controllable = map[string]bool{"tahoma": true, "hue": true, "shelly": true}

// Origin décrit ce qui a déclenché une action, pour l'historique.
type Origin struct {
	Kind      string // store.OriginInterface, OriginSceneManual, OriginSceneSchedule
	SceneID   int64
	SceneName string
}

// Interface est l'origine des actions faites depuis l'interface.
var Interface = Origin{Kind: store.OriginInterface}

// Controller pilote les équipements des sources configurées.
type Controller struct {
	store      *store.Store
	commanders map[string]command.Commander
	nudge      func(source string)
	log        *slog.Logger
}

// New construit un contrôleur. commanders ne contient que les sources
// configurées ; nudge, facultatif, avance le relevé d'une source après une
// commande.
func New(st *store.Store, commanders map[string]command.Commander, nudge func(string), log *slog.Logger) *Controller {
	if commanders == nil {
		commanders = map[string]command.Commander{}
	}
	return &Controller{store: st, commanders: commanders, nudge: nudge, log: log}
}

// Controllable indique si les équipements de la source se pilotent.
func Controllable(source string) bool {
	return controllable[source]
}

// Send transmet une commande à un équipement et la consigne, réussie ou non.
//
// Les erreurs enveloppent ErrNotControllable, ErrNotConfigured ou
// command.ErrUnsupported quand la commande n'a pas pu partir ; toute autre
// erreur vient de la source.
func (c *Controller) Send(ctx context.Context, device store.Device, cmd string, params []any, origin Origin) (string, error) {
	execID, err := c.send(ctx, device, cmd, params)

	reason := ""
	if err != nil {
		reason = err.Error()
	}
	c.Record(ctx, device, cmd, params, reason, origin)

	if err == nil && c.nudge != nil {
		// Sans flux d'événements, la source ne remonterait le nouvel état
		// qu'au prochain tour de polling : le demander tout de suite.
		c.nudge(device.Source)
	}
	return execID, err
}

func (c *Controller) send(ctx context.Context, device store.Device, cmd string, params []any) (string, error) {
	if !Controllable(device.Source) {
		return "", fmt.Errorf("%w : les équipements %s ne sont pas pilotables", ErrNotControllable, device.Source)
	}
	commander := c.commanders[device.Source]
	if commander == nil {
		return "", fmt.Errorf("%w : %s", ErrNotConfigured, device.Source)
	}
	return commander.Execute(ctx, device.ID, cmd, params)
}

// Record consigne une action dans l'historique. reason vide signifie une
// réussite. Un échec d'écriture est journalisé sans être remonté : l'action a
// déjà eu lieu, la signaler en erreur inciterait à la refaire.
func (c *Controller) Record(ctx context.Context, device store.Device, cmd string, params []any, reason string, origin Origin) {
	entry := store.CommandLogEntry{
		DeviceID:   device.ID,
		DeviceName: device.Name,
		DeviceKind: device.Kind,
		Source:     device.Source,
		Command:    cmd,
		Parameters: params,
		Success:    reason == "",
		Error:      reason,
		Origin:     origin.Kind,
		SceneName:  origin.SceneName,
		CreatedAt:  time.Now().UTC(),
	}
	if origin.SceneID != 0 {
		id := origin.SceneID
		entry.SceneID = &id
	}
	// Détaché de l'annulation de l'appelant : une scène relancée annule son
	// contexte, mais ce qui a déjà été tenté doit rester consigné.
	if err := c.store.RecordCommand(context.WithoutCancel(ctx), entry); err != nil && c.log != nil {
		c.log.Warn("historique de commande non enregistré", "device", device.ID, "command", cmd, "error", err)
	}
}
