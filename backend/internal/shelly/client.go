// Package shelly implémente le client des modules Shelly Gen2+ (Plus, Pro,
// Gen3, Gen4), pilotés par leur API JSON-RPC locale.
//
// Les modules n'ont ni TLS ni découverte fiable depuis un conteneur : ils sont
// joints par adresse IP, en HTTP simple. Chaque voie d'un module (switch:0,
// switch:1…) devient un équipement distinct.
package shelly

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stephguignard/domotic/internal/command"
	"github.com/stephguignard/domotic/internal/config"
	"github.com/stephguignard/domotic/internal/store"
)

// configTTL espace les relectures de la configuration d'un module. Les noms des
// voies changent rarement, alors que l'état est relu toutes les quelques
// secondes : inutile de transférer la configuration complète à chaque fois.
const configTTL = 15 * time.Minute

// Client interroge un ensemble de modules Shelly.
type Client struct {
	hosts    []string
	password string
	http     *http.Client
	log      *slog.Logger
	rpcID    atomic.Int64

	mu      sync.Mutex
	modules map[string]*module // par hôte
}

// module est ce que le client retient d'un module entre deux relevés.
type module struct {
	id       string         // ex. shellypro3-841fe88e5b68
	name     string         // nom du module, éventuellement vide
	channels map[int]string // numéro de voie → nom de la voie
	loadedAt time.Time
}

// NewClient construit un client pour les modules configurés.
func NewClient(cfg *config.Config, log *slog.Logger) *Client {
	return &Client{
		hosts:    cfg.Shelly.Hosts,
		password: cfg.Shelly.Password,
		// Un module local répond en quelques millisecondes : un délai court
		// évite qu'un module débranché ne retienne toute la boucle.
		http:    &http.Client{Timeout: 5 * time.Second},
		log:     log,
		modules: map[string]*module{},
	}
}

// Snapshot est le résultat d'un relevé de tous les modules.
type Snapshot struct {
	// Devices porte les voies des modules qui ont répondu.
	Devices []store.Device
	// Unreachable porte l'identifiant des modules connus qui n'ont pas
	// répondu : leurs voies doivent être marquées injoignables.
	Unreachable []string
}

// Fetch relève l'état de tous les modules. Un module injoignable n'empêche pas
// de relever les autres : l'erreur retournée les regroupe.
func (c *Client) Fetch(ctx context.Context) (Snapshot, error) {
	var (
		snap Snapshot
		errs []error
	)
	for _, host := range c.hosts {
		devices, err := c.fetchHost(ctx, host)
		if err != nil {
			errs = append(errs, err)
			if m := c.known(host); m != nil {
				snap.Unreachable = append(snap.Unreachable, m.id)
			}
			continue
		}
		snap.Devices = append(snap.Devices, devices...)
	}
	return snap, errors.Join(errs...)
}

func (c *Client) known(host string) *module {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.modules[host]
}

func (c *Client) fetchHost(ctx context.Context, host string) ([]store.Device, error) {
	m := c.known(host)
	if m == nil || time.Since(m.loadedAt) > configTTL {
		loaded, err := c.loadConfig(ctx, host)
		if err != nil {
			return nil, err
		}
		m = loaded
		c.mu.Lock()
		c.modules[host] = m
		c.mu.Unlock()
	}

	var status map[string]json.RawMessage
	if _, err := c.call(ctx, host, "Shelly.GetStatus", nil, &status); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	var devices []store.Device
	for key, raw := range status {
		n, ok := channelNumber(key)
		if !ok {
			continue
		}
		var sw switchStatus
		if err := json.Unmarshal(raw, &sw); err != nil {
			return nil, fmt.Errorf("shelly %s: état de %s illisible: %w", host, key, err)
		}

		state := map[string]any{"on": sw.Output}
		if sw.Temperature != nil {
			state["device_temperature"] = sw.Temperature.C
		}
		encoded, err := json.Marshal(state)
		if err != nil {
			return nil, fmt.Errorf("shelly %s: sérialisation de l'état de %s: %w", host, key, err)
		}

		devices = append(devices, store.Device{
			ID:        deviceID(m.id, n),
			Source:    "shelly",
			Name:      m.channelName(n),
			Kind:      "switch",
			State:     string(encoded),
			Reachable: true,
			UpdatedAt: now,
		})
	}
	// L'ordre d'une map n'est pas stable : trier garde les journaux et les
	// tests lisibles.
	slices.SortFunc(devices, func(a, b store.Device) int { return strings.Compare(a.ID, b.ID) })
	return devices, nil
}

// loadConfig relit l'identité du module et le nom de ses voies.
func (c *Client) loadConfig(ctx context.Context, host string) (*module, error) {
	var cfg map[string]json.RawMessage
	src, err := c.call(ctx, host, "Shelly.GetConfig", nil, &cfg)
	if err != nil {
		return nil, err
	}
	if src == "" {
		return nil, fmt.Errorf("shelly %s: réponse sans identifiant de module", host)
	}

	m := &module{id: src, channels: map[int]string{}, loadedAt: time.Now()}

	var sys struct {
		Device struct {
			Name *string `json:"name"`
		} `json:"device"`
	}
	if raw, ok := cfg["sys"]; ok && json.Unmarshal(raw, &sys) == nil && sys.Device.Name != nil {
		m.name = *sys.Device.Name
	}

	for key, raw := range cfg {
		n, ok := channelNumber(key)
		if !ok {
			continue
		}
		var sw struct {
			Name *string `json:"name"`
		}
		if err := json.Unmarshal(raw, &sw); err == nil && sw.Name != nil {
			m.channels[n] = *sw.Name
		}
	}
	return m, nil
}

// channelName retourne le nom de la voie, ou à défaut un nom construit depuis
// celui du module.
func (m *module) channelName(n int) string {
	if name := strings.TrimSpace(m.channels[n]); name != "" {
		return name
	}
	base := m.name
	if base == "" {
		base = m.id
	}
	return fmt.Sprintf("%s voie %d", base, n+1)
}

// Execute pilote une voie. Les commandes reprennent le vocabulaire unifié du
// frontend : on, off, toggle.
func (c *Client) Execute(ctx context.Context, id, cmd string, _ []any) (string, error) {
	moduleID, n, err := parseDeviceID(id)
	if err != nil {
		return "", err
	}
	host := c.hostOf(moduleID)
	if host == "" {
		return "", fmt.Errorf("module Shelly %s inconnu : il n'a pas encore été relevé", moduleID)
	}

	var (
		method string
		params = map[string]any{"id": n}
	)
	switch cmd {
	case "on", "off":
		method = "Switch.Set"
		params["on"] = cmd == "on"
	case "toggle":
		method = "Switch.Toggle"
	default:
		return "", fmt.Errorf("%w par une voie Shelly: %s", command.ErrUnsupported, cmd)
	}

	if _, err := c.call(ctx, host, method, params, nil); err != nil {
		return "", err
	}
	c.log.Info("commande Shelly envoyée", "device", id, "command", cmd)
	return "", nil
}

func (c *Client) hostOf(moduleID string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	for host, m := range c.modules {
		if m.id == moduleID {
			return host
		}
	}
	return ""
}

// call exécute une méthode JSON-RPC sur un module et décode son résultat dans
// out. Elle retourne l'identifiant du module, porté par le champ src.
func (c *Client) call(ctx context.Context, host, method string, params, out any) (string, error) {
	body, err := json.Marshal(rpcRequest{ID: c.rpcID.Add(1), Method: method, Params: params})
	if err != nil {
		return "", fmt.Errorf("shelly %s %s: encodage: %w", host, method, err)
	}

	resp, err := c.post(ctx, host, body, "")
	if err != nil {
		return "", fmt.Errorf("shelly %s %s: %w", host, method, err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		if c.password == "" {
			return "", fmt.Errorf("shelly %s: le module exige une authentification, renseigner SHELLY_PASSWORD", host)
		}
		ch, err := parseChallenge(resp.Header.Get("WWW-Authenticate"))
		if err != nil {
			return "", fmt.Errorf("shelly %s: %w", host, err)
		}
		auth := ch.authorization(http.MethodPost, "/rpc", c.password, newCnonce())
		if resp, err = c.post(ctx, host, body, auth); err != nil {
			return "", fmt.Errorf("shelly %s %s: %w", host, method, err)
		}
		if resp.StatusCode == http.StatusUnauthorized {
			resp.Body.Close()
			return "", fmt.Errorf("shelly %s: mot de passe refusé", host)
		}
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("shelly %s %s: lecture de la réponse: %w", host, method, err)
	}

	var rpc rpcResponse
	if err := json.Unmarshal(payload, &rpc); err != nil {
		return "", fmt.Errorf("shelly %s %s: statut %d, réponse illisible: %w", host, method, resp.StatusCode, err)
	}
	if rpc.Error != nil {
		return "", fmt.Errorf("shelly %s %s: erreur %d: %s", host, method, rpc.Error.Code, rpc.Error.Message)
	}
	if out != nil {
		if err := json.Unmarshal(rpc.Result, out); err != nil {
			return "", fmt.Errorf("shelly %s %s: décodage: %w", host, method, err)
		}
	}
	return rpc.Src, nil
}

func (c *Client) post(ctx context.Context, host string, body []byte, auth string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+host+"/rpc", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	return c.http.Do(req)
}

type rpcRequest struct {
	ID     int64  `json:"id"`
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
}

type rpcResponse struct {
	ID     int64           `json:"id"`
	Src    string          `json:"src"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// switchStatus est l'état d'une voie dans Shelly.GetStatus.
type switchStatus struct {
	Output      bool `json:"output"`
	Temperature *struct {
		C float64 `json:"tC"`
	} `json:"temperature"`
}

// channelNumber reconnaît une clé de composant « switch:N ».
func channelNumber(key string) (int, bool) {
	rest, ok := strings.CutPrefix(key, "switch:")
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(rest)
	return n, err == nil && n >= 0
}

// deviceID construit l'identifiant unifié d'une voie.
func deviceID(moduleID string, n int) string {
	return fmt.Sprintf("%s:switch:%d", moduleID, n)
}

// parseDeviceID décompose un identifiant construit par deviceID.
func parseDeviceID(id string) (string, int, error) {
	moduleID, rest, ok := strings.Cut(id, ":switch:")
	if !ok || moduleID == "" {
		return "", 0, fmt.Errorf("identifiant d'équipement Shelly invalide: %s", id)
	}
	n, err := strconv.Atoi(rest)
	if err != nil || n < 0 {
		return "", 0, fmt.Errorf("identifiant d'équipement Shelly invalide: %s", id)
	}
	return moduleID, n, nil
}

// ModulePrefix retourne le préfixe commun aux identifiants des voies d'un module.
func ModulePrefix(moduleID string) string {
	return moduleID + ":"
}
