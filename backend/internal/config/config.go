// Package config charge et valide la configuration du service depuis
// l'environnement. Toute la configuration est résolue au démarrage : un
// paramètre manquant fait échouer le lancement plutôt que de provoquer une
// erreur plus tard, au milieu d'une boucle de polling.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config regroupe l'ensemble des paramètres du service.
type Config struct {
	Port      int
	DBPath    string
	PublicURL string // URL publique du service, utilisée pour le callback OAuth2

	// Location situe la maison : fuseau des horaires de scènes, coordonnées
	// des horaires solaires.
	Location LocationConfig

	Netatmo NetatmoConfig
	Tahoma  TahomaConfig
	Shelly  ShellyConfig
	Hue     HueConfig
}

// LocationConfig situe la maison.
type LocationConfig struct {
	// TimeZone interprète les heures des horaires de scènes. Le conteneur
	// distroless n'a ni /etc/localtime ni base de fuseaux : le binaire
	// embarque la sienne (time/tzdata) et le fuseau se nomme explicitement.
	TimeZone *time.Location
	// Latitude et Longitude, en degrés décimaux (est positif), servent au
	// lever et au coucher du soleil. Sans elles, HasCoordinates est faux et
	// les horaires solaires sont refusés.
	Latitude       float64
	Longitude      float64
	HasCoordinates bool
}

// NetatmoConfig porte les paramètres de l'API cloud Netatmo.
type NetatmoConfig struct {
	ClientID     string
	ClientSecret string
	Scopes       []string
	PollInterval time.Duration
}

// Enabled indique si l'intégration Netatmo est configurée.
func (c NetatmoConfig) Enabled() bool {
	return c.ClientID != "" && c.ClientSecret != ""
}

// TahomaConfig porte les paramètres de la box Somfy TaHoma locale.
type TahomaConfig struct {
	Host  string // adresse IP de la box — voir la note sur mDNS ci-dessous
	Port  int
	PIN   string // PIN de la passerelle, ex. "1234-5678-9012"
	Token string // jeton généré depuis le mode développeur de l'app TaHoma

	// EventInterval espace les appels à /events/{id}/fetch. La box impose un
	// maximum d'un appel par seconde ; 2 s laisse une marge confortable.
	EventInterval time.Duration
}

// Enabled indique si l'intégration TaHoma est configurée.
func (c TahomaConfig) Enabled() bool {
	return c.Host != "" && c.Token != "" && c.PIN != ""
}

// ShellyConfig porte les paramètres des modules Shelly Gen2+ du réseau local.
type ShellyConfig struct {
	// Hosts liste les adresses IP des modules, pour la même raison que
	// TAHOMA_HOST : mDNS est peu fiable depuis un conteneur Docker.
	Hosts []string
	// Password est commun à tous les modules ; vide si l'authentification est
	// désactivée sur les modules.
	Password string
	// PollInterval espace les relevés d'état. Les modules répondent en local
	// en quelques millisecondes : un intervalle court ne coûte presque rien.
	PollInterval time.Duration
}

// Enabled indique si l'intégration Shelly est configurée.
func (c ShellyConfig) Enabled() bool {
	return len(c.Hosts) > 0
}

// HueConfig porte les paramètres du pont Philips Hue local (API v2).
type HueConfig struct {
	Host string // adresse IP du pont, pour la même raison que TAHOMA_HOST
	// BridgeID est l'identifiant du pont, ex. ECB5FAFFFE943A44. Son
	// certificat TLS est émis à ce nom : il sert à l'authentifier.
	BridgeID string
	// AppKey est la clé d'application, obtenue une fois avec `domotic hue-pair`.
	AppKey string
}

// Enabled indique si l'intégration Hue est configurée.
func (c HueConfig) Enabled() bool {
	return c.Host != "" && c.BridgeID != "" && c.AppKey != ""
}

// Load lit la configuration depuis l'environnement et la valide.
func Load() (*Config, error) {
	cfg := &Config{
		Port:      envInt("DOMOTIC_PORT", 8080),
		DBPath:    envStr("DOMOTIC_DB_PATH", "./data/domotic.db"),
		PublicURL: strings.TrimSuffix(envStr("DOMOTIC_PUBLIC_URL", "http://localhost:8080"), "/"),
		Netatmo: NetatmoConfig{
			ClientID:     envStr("NETATMO_CLIENT_ID", ""),
			ClientSecret: envStr("NETATMO_CLIENT_SECRET", ""),
			Scopes:       strings.Fields(envStr("NETATMO_SCOPES", "read_station read_camera access_camera")),
			PollInterval: envDuration("NETATMO_POLL_INTERVAL", 10*time.Minute),
		},
		Tahoma: TahomaConfig{
			Host:          envStr("TAHOMA_HOST", ""),
			Port:          envInt("TAHOMA_PORT", 8443),
			PIN:           envStr("TAHOMA_PIN", ""),
			Token:         envStr("TAHOMA_TOKEN", ""),
			EventInterval: envDuration("TAHOMA_EVENT_INTERVAL", 2*time.Second),
		},
		Hue: HueConfig{
			Host:     envStr("HUE_HOST", ""),
			BridgeID: envStr("HUE_BRIDGE_ID", ""),
			AppKey:   envStr("HUE_APP_KEY", ""),
		},
		Shelly: ShellyConfig{
			Hosts:        envList("SHELLY_HOSTS"),
			Password:     envStr("SHELLY_PASSWORD", ""),
			PollInterval: envDuration("SHELLY_POLL_INTERVAL", 5*time.Second),
		},
	}

	if err := cfg.loadLocation(); err != nil {
		return nil, err
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("DOMOTIC_PORT invalide: %d", c.Port)
	}
	if c.DBPath == "" {
		return fmt.Errorf("DOMOTIC_DB_PATH ne peut pas être vide")
	}

	// Une configuration partielle est presque toujours une erreur de saisie :
	// mieux vaut refuser de démarrer que de faire tourner une intégration
	// silencieusement inactive.
	n := c.Netatmo
	if (n.ClientID == "") != (n.ClientSecret == "") {
		return fmt.Errorf("NETATMO_CLIENT_ID et NETATMO_CLIENT_SECRET doivent être fournis ensemble")
	}
	if n.Enabled() && n.PollInterval < time.Minute {
		// La station ne remonte ses mesures que toutes les ~10 minutes ;
		// interroger plus vite ne fait que consommer le quota d'API.
		return fmt.Errorf("NETATMO_POLL_INTERVAL doit valoir au moins 1m (valeur: %s)", n.PollInterval)
	}

	t := c.Tahoma
	set := 0
	for _, v := range []string{t.Host, t.PIN, t.Token} {
		if v != "" {
			set++
		}
	}
	if set != 0 && set != 3 {
		return fmt.Errorf("TAHOMA_HOST, TAHOMA_PIN et TAHOMA_TOKEN doivent être fournis ensemble")
	}
	if t.Enabled() && t.EventInterval < time.Second {
		// La box rejette les appels à /events/{id}/fetch plus rapprochés qu'une seconde.
		return fmt.Errorf("TAHOMA_EVENT_INTERVAL doit valoir au moins 1s (valeur: %s)", t.EventInterval)
	}

	h := c.Hue
	switch {
	case h.Enabled(), h.Host == "" && h.BridgeID == "" && h.AppKey == "":
	case h.Host != "" && h.BridgeID != "":
		// Le cas le plus probable : le pont est renseigné, l'appairage reste à faire.
		return fmt.Errorf("HUE_APP_KEY manquant : lancer `domotic hue-pair` pour l'obtenir")
	default:
		return fmt.Errorf("HUE_HOST, HUE_BRIDGE_ID et HUE_APP_KEY doivent être fournis ensemble")
	}

	sh := c.Shelly
	if !sh.Enabled() && sh.Password != "" {
		return fmt.Errorf("SHELLY_PASSWORD est renseigné sans SHELLY_HOSTS")
	}
	if sh.Enabled() && sh.PollInterval < time.Second {
		return fmt.Errorf("SHELLY_POLL_INTERVAL doit valoir au moins 1s (valeur: %s)", sh.PollInterval)
	}

	return nil
}

// loadLocation lit le fuseau et les coordonnées de la maison.
func (c *Config) loadLocation() error {
	name := envStr("DOMOTIC_TIMEZONE", "UTC")
	loc, err := time.LoadLocation(name)
	if err != nil {
		return fmt.Errorf("DOMOTIC_TIMEZONE invalide %q, attendu un nom IANA comme Europe/Zurich", name)
	}
	c.Location.TimeZone = loc

	lat, lon := os.Getenv("DOMOTIC_LATITUDE"), os.Getenv("DOMOTIC_LONGITUDE")
	if (lat == "") != (lon == "") {
		return fmt.Errorf("DOMOTIC_LATITUDE et DOMOTIC_LONGITUDE doivent être fournies ensemble")
	}
	if lat == "" {
		return nil
	}
	if c.Location.Latitude, err = strconv.ParseFloat(lat, 64); err != nil ||
		c.Location.Latitude < -90 || c.Location.Latitude > 90 {
		return fmt.Errorf("DOMOTIC_LATITUDE invalide %q, attendu des degrés décimaux entre -90 et 90", lat)
	}
	if c.Location.Longitude, err = strconv.ParseFloat(lon, 64); err != nil ||
		c.Location.Longitude < -180 || c.Location.Longitude > 180 {
		return fmt.Errorf("DOMOTIC_LONGITUDE invalide %q, attendu des degrés décimaux entre -180 et 180", lon)
	}
	c.Location.HasCoordinates = true
	return nil
}

// NetatmoRedirectURL construit l'URL de callback OAuth2, qui doit correspondre
// exactement à celle déclarée sur dev.netatmo.com.
func (c *Config) NetatmoRedirectURL() string {
	return c.PublicURL + "/auth/netatmo/callback"
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// envList lit une liste séparée par des virgules, en ignorant les espaces et
// les éléments vides.
func envList(key string) []string {
	var out []string
	for _, v := range strings.Split(os.Getenv(key), ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
