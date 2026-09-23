// Package netatmo implémente le client de l'API cloud Netatmo (météo et
// sécurité), authentifiée en OAuth2.
package netatmo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"github.com/stephguignard/domotic/internal/config"
	"github.com/stephguignard/domotic/internal/store"
)

const (
	authURL  = "https://api.netatmo.com/oauth2/authorize"
	tokenURL = "https://api.netatmo.com/oauth2/token"
	apiBase  = "https://api.netatmo.com/api"
)

// ErrNotAuthenticated signale qu'aucun jeton n'est encore en base : le flux
// OAuth2 doit être parcouru une première fois via /auth/netatmo.
var ErrNotAuthenticated = errors.New("netatmo: non authentifié")

// Client interroge l'API Netatmo pour le compte de l'utilisateur authentifié.
type Client struct {
	oauth *oauth2.Config
	store *store.Store
	log   *slog.Logger

	// http est construit à la volée à partir du jeton en base, de sorte qu'une
	// authentification effectuée après le démarrage soit prise en compte sans
	// redémarrer le service.
	timeout time.Duration
}

// NewClient construit un client Netatmo. Il n'effectue aucun appel réseau :
// l'authentification est résolue paresseusement au premier usage.
func NewClient(cfg *config.Config, st *store.Store, log *slog.Logger) *Client {
	return &Client{
		oauth: &oauth2.Config{
			ClientID:     cfg.Netatmo.ClientID,
			ClientSecret: cfg.Netatmo.ClientSecret,
			Scopes:       cfg.Netatmo.Scopes,
			RedirectURL:  cfg.NetatmoRedirectURL(),
			Endpoint: oauth2.Endpoint{
				AuthURL:   authURL,
				TokenURL:  tokenURL,
				AuthStyle: oauth2.AuthStyleInParams,
			},
		},
		store:   st,
		log:     log,
		timeout: 30 * time.Second,
	}
}

// AuthCodeURL retourne l'URL de consentement Netatmo pour un état donné.
func (c *Client) AuthCodeURL(state string) string {
	return c.oauth.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// Exchange échange le code d'autorisation contre un couple de jetons et le
// persiste. C'est la seule étape qui requiert une interaction navigateur.
func (c *Client) Exchange(ctx context.Context, code string) error {
	tok, err := c.oauth.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("échange du code Netatmo: %w", err)
	}
	if err := c.store.SaveToken(ctx, Provider, tok); err != nil {
		return err
	}
	c.log.Info("authentification Netatmo réussie", "expiry", tok.Expiry)
	return nil
}

// Authenticated indique si un jeton est disponible en base.
func (c *Client) Authenticated(ctx context.Context) bool {
	_, err := c.store.GetToken(ctx, Provider)
	return err == nil
}

// httpClient construit un client HTTP qui rafraîchit et persiste le jeton
// automatiquement.
func (c *Client) httpClient(ctx context.Context) (*http.Client, error) {
	tok, err := c.store.GetToken(ctx, Provider)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrNotAuthenticated
	}
	if err != nil {
		return nil, err
	}

	src := NewTokenSource(ctx, c.oauth, c.store, tok, c.log)
	client := oauth2.NewClient(ctx, src)
	client.Timeout = c.timeout
	return client, nil
}

// get appelle un endpoint de l'API et décode la réponse JSON dans out.
func (c *Client) get(ctx context.Context, path string, out any) error {
	client, err := c.httpClient(ctx)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+path, nil)
	if err != nil {
		return fmt.Errorf("netatmo %s: %w", path, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("netatmo %s: %w", path, err)
	}
	defer resp.Body.Close()

	// Plafonner la lecture : une réponse inattendue ne doit pas pouvoir
	// saturer la mémoire d'un NAS qui n'a qu'un gigaoctet.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("netatmo %s: lecture de la réponse: %w", path, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("netatmo %s: statut %d: %s", path, resp.StatusCode, truncate(body, 200))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("netatmo %s: décodage: %w", path, err)
	}
	return nil
}

// FetchWeather récupère les stations météo et retourne les équipements
// normalisés ainsi que leurs relevés courants.
func (c *Client) FetchWeather(ctx context.Context) ([]store.Device, []store.Measurement, error) {
	var resp stationsDataResponse
	if err := c.get(ctx, "/getstationsdata", &resp); err != nil {
		return nil, nil, err
	}
	if resp.Error != nil {
		return nil, nil, fmt.Errorf("netatmo getstationsdata: %s (code %d)", resp.Error.Message, resp.Error.Code)
	}

	var (
		devices      []store.Device
		measurements []store.Measurement
		now          = time.Now().UTC()
	)

	for _, st := range resp.Body.Devices {
		room := st.StationName

		// La station principale est elle-même un capteur intérieur.
		d, ms := normalize(st.ID, displayName(st.ModuleName, st.StationName), kindForType(st.Type),
			room, st.Reachable, st.Dashboard, now)
		devices = append(devices, d)
		measurements = append(measurements, ms...)

		for _, m := range st.Modules {
			md, mms := normalize(m.ID, displayName(m.ModuleName, m.Type), kindForType(m.Type),
				room, m.Reachable, m.Dashboard, now)

			// La batterie n'est pas dans le dashboard mais reste utile à suivre.
			if m.BatteryPct > 0 {
				md.State = mergeState(md.State, "battery_percent", float64(m.BatteryPct))
			}

			devices = append(devices, md)
			measurements = append(measurements, mms...)
		}
	}

	return devices, measurements, nil
}

// FetchSecurity récupère les caméras Netatmo.
func (c *Client) FetchSecurity(ctx context.Context) ([]store.Device, error) {
	var resp homeDataResponse
	if err := c.get(ctx, "/gethomedata", &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("netatmo gethomedata: %s (code %d)", resp.Error.Message, resp.Error.Code)
	}

	now := time.Now().UTC()
	var devices []store.Device

	for _, h := range resp.Body.Homes {
		for _, cam := range h.Cameras {
			state, err := json.Marshal(map[string]any{
				"status":      cam.Status,
				"sd_status":   cam.SDStatus,
				"alim_status": cam.AlimStatus,
				"is_local":    cam.IsLocal,
			})
			if err != nil {
				return nil, fmt.Errorf("sérialisation de l'état de la caméra %s: %w", cam.ID, err)
			}

			devices = append(devices, store.Device{
				ID:        cam.ID,
				Source:    "netatmo",
				Name:      displayName(cam.Name, cam.Type),
				Kind:      "camera",
				Room:      h.Name,
				State:     string(state),
				Reachable: cam.Status == "on",
				UpdatedAt: now,
			})
		}
	}

	return devices, nil
}

// normalize convertit un module Netatmo en équipement et relevés unifiés.
func normalize(id, name, kind, room string, reachable bool, d dashboard, now time.Time) (store.Device, []store.Measurement) {
	metrics := d.metrics()

	state, err := json.Marshal(metrics)
	if err != nil {
		// metrics est une map[string]float64 : la sérialisation ne peut pas
		// échouer, mais on reste défensif plutôt que d'écrire du JSON invalide.
		state = []byte("{}")
	}

	device := store.Device{
		ID:        id,
		Source:    "netatmo",
		Name:      name,
		Kind:      kind,
		Room:      room,
		State:     string(state),
		Reachable: reachable,
		UpdatedAt: now,
	}

	// Netatmo horodate chaque relevé : conserver son temps plutôt que celui du
	// polling évite de créer de faux points quand la station n'a rien renvoyé
	// de neuf, la contrainte d'unicité écartant alors le doublon.
	recordedAt := now
	if d.TimeUTC > 0 {
		recordedAt = time.Unix(d.TimeUTC, 0).UTC()
	}

	measurements := make([]store.Measurement, 0, len(metrics))
	for metric, value := range metrics {
		measurements = append(measurements, store.Measurement{
			DeviceID:   id,
			Metric:     metric,
			Value:      value,
			RecordedAt: recordedAt,
		})
	}

	return device, measurements
}

// mergeState ajoute une clé à un état JSON existant.
func mergeState(state, key string, value float64) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(state), &m); err != nil || m == nil {
		m = map[string]any{}
	}
	m[key] = value
	out, err := json.Marshal(m)
	if err != nil {
		return state
	}
	return string(out)
}

func displayName(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
