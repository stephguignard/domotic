// Package command définit le contrat commun aux sources pilotables.
//
// Il vit à part pour que l'API puisse reconnaître les erreurs des clients sans
// que ceux-ci dépendent de l'API.
package command

import (
	"context"
	"errors"
)

// ErrUnsupported signale une commande que l'équipement ne sait pas exécuter.
// Les clients l'enveloppent : l'API en fait une 422 plutôt qu'une 502, la
// source n'étant pas en cause.
var ErrUnsupported = errors.New("commande non prise en charge")

// Commander pilote les équipements d'une source.
type Commander interface {
	// Execute envoie une commande à un équipement et retourne l'identifiant
	// d'exécution attribué par la source, ou une chaîne vide si elle n'en
	// attribue pas.
	Execute(ctx context.Context, deviceID, command string, params []any) (string, error)
}
