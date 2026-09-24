package config

import (
	"strings"
	"testing"
	"time"
)

// isolate neutralise toutes les variables du service, pour que ces tests ne
// dépendent pas de l'environnement de la machine. Le Makefile charge .env :
// sans cette isolation, une configuration réelle ferait échouer les cas qui
// attendent une configuration vide ou partielle.
//
// Une valeur vide équivaut à une variable absente pour le package config.
func isolate(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"DOMOTIC_PORT", "DOMOTIC_DB_PATH", "DOMOTIC_PUBLIC_URL",
		"NETATMO_CLIENT_ID", "NETATMO_CLIENT_SECRET", "NETATMO_SCOPES", "NETATMO_POLL_INTERVAL",
		"TAHOMA_HOST", "TAHOMA_PORT", "TAHOMA_PIN", "TAHOMA_TOKEN", "TAHOMA_EVENT_INTERVAL",
		"SHELLY_HOSTS", "SHELLY_PASSWORD", "SHELLY_POLL_INTERVAL",
		"HUE_HOST", "HUE_BRIDGE_ID", "HUE_APP_KEY",
		"DOMOTIC_TIMEZONE", "DOMOTIC_LATITUDE", "DOMOTIC_LONGITUDE",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadDefaults(t *testing.T) {
	isolate(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("port par défaut = %d, attendu 8080", cfg.Port)
	}
	// Sans identifiants, les deux intégrations restent inertes : le service
	// démarre quand même, ce qui permet de le configurer progressivement.
	if cfg.Netatmo.Enabled() {
		t.Error("Netatmo ne devrait pas être activé sans identifiants")
	}
	if cfg.Tahoma.Enabled() {
		t.Error("TaHoma ne devrait pas être activé sans identifiants")
	}
	if cfg.Shelly.Enabled() {
		t.Error("Shelly ne devrait pas être activé sans adresse")
	}
	if cfg.Hue.Enabled() {
		t.Error("Hue ne devrait pas être activé sans pont")
	}
}

func TestLoadRejectsPartialNetatmoConfig(t *testing.T) {
	isolate(t)
	// Une configuration à moitié renseignée est presque toujours une faute de
	// frappe : mieux vaut refuser de démarrer qu'ignorer l'intégration.
	t.Setenv("NETATMO_CLIENT_ID", "abc")

	if _, err := Load(); err == nil {
		t.Error("attendu une erreur pour un NETATMO_CLIENT_SECRET manquant")
	}
}

func TestLoadRejectsPartialTahomaConfig(t *testing.T) {
	isolate(t)
	t.Setenv("TAHOMA_HOST", "192.168.1.42")
	t.Setenv("TAHOMA_PIN", "1234-5678-9012")
	// TAHOMA_TOKEN manquant

	if _, err := Load(); err == nil {
		t.Error("attendu une erreur pour un TAHOMA_TOKEN manquant")
	}
}

func TestLoadAcceptsCompleteConfig(t *testing.T) {
	isolate(t)
	t.Setenv("NETATMO_CLIENT_ID", "id")
	t.Setenv("NETATMO_CLIENT_SECRET", "secret")
	t.Setenv("TAHOMA_HOST", "192.168.1.42")
	t.Setenv("TAHOMA_PIN", "1234-5678-9012")
	t.Setenv("TAHOMA_TOKEN", "token")
	t.Setenv("DOMOTIC_PUBLIC_URL", "https://domotic.example.com/")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !cfg.Netatmo.Enabled() || !cfg.Tahoma.Enabled() {
		t.Error("les deux intégrations devraient être activées")
	}
	// L'URL de callback doit correspondre au caractère près à celle déclarée
	// sur dev.netatmo.com, barre oblique finale comprise.
	if got, want := cfg.NetatmoRedirectURL(), "https://domotic.example.com/auth/netatmo/callback"; got != want {
		t.Errorf("URL de callback = %q, attendu %q", got, want)
	}
}

func TestLoadRejectsTooFrequentPolling(t *testing.T) {
	isolate(t)
	t.Setenv("NETATMO_CLIENT_ID", "id")
	t.Setenv("NETATMO_CLIENT_SECRET", "secret")
	t.Setenv("NETATMO_POLL_INTERVAL", "10s")

	if _, err := Load(); err == nil {
		t.Error("attendu une erreur pour un intervalle de polling trop court")
	}
}

func TestLoadRejectsTooFrequentEventFetch(t *testing.T) {
	isolate(t)
	t.Setenv("TAHOMA_HOST", "192.168.1.42")
	t.Setenv("TAHOMA_PIN", "1234-5678-9012")
	t.Setenv("TAHOMA_TOKEN", "token")
	// La box rejette les appels à /events/{id}/fetch plus rapprochés qu'une seconde.
	t.Setenv("TAHOMA_EVENT_INTERVAL", "100ms")

	if _, err := Load(); err == nil {
		t.Error("attendu une erreur pour un intervalle d'événements trop court")
	}
}

func TestInvalidDurationFallsBackToDefault(t *testing.T) {
	isolate(t)
	t.Setenv("NETATMO_CLIENT_ID", "id")
	t.Setenv("NETATMO_CLIENT_SECRET", "secret")
	t.Setenv("NETATMO_POLL_INTERVAL", "pas-une-durée")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Netatmo.PollInterval != 10*time.Minute {
		t.Errorf("intervalle = %s, attendu la valeur par défaut de 10m", cfg.Netatmo.PollInterval)
	}
}

func TestShellyHostsList(t *testing.T) {
	isolate(t)
	t.Setenv("SHELLY_HOSTS", " 192.168.1.105, ,192.168.1.106 ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"192.168.1.105", "192.168.1.106"}
	if len(cfg.Shelly.Hosts) != 2 || cfg.Shelly.Hosts[0] != want[0] || cfg.Shelly.Hosts[1] != want[1] {
		t.Errorf("hôtes = %q, attendu %q", cfg.Shelly.Hosts, want)
	}
	if cfg.Shelly.PollInterval != 5*time.Second {
		t.Errorf("intervalle par défaut = %s, attendu 5s", cfg.Shelly.PollInterval)
	}
}

func TestShellyRejectsPartialConfig(t *testing.T) {
	isolate(t)
	t.Setenv("SHELLY_PASSWORD", "secret")

	if _, err := Load(); err == nil {
		t.Error("attendu une erreur pour un mot de passe sans hôte")
	}
}

func TestShellyRejectsTooShortInterval(t *testing.T) {
	isolate(t)
	t.Setenv("SHELLY_HOSTS", "192.168.1.105")
	t.Setenv("SHELLY_POLL_INTERVAL", "500ms")

	if _, err := Load(); err == nil {
		t.Error("attendu une erreur pour un intervalle inférieur à 1s")
	}
}

func TestHuePartialConfig(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		wantErr string // vide : pas d'erreur attendue
	}{
		{"complète", map[string]string{"HUE_HOST": "192.168.1.101", "HUE_BRIDGE_ID": "ECB5FAFFFE943A44", "HUE_APP_KEY": "k"}, ""},
		{"appairage à faire", map[string]string{"HUE_HOST": "192.168.1.101", "HUE_BRIDGE_ID": "ECB5FAFFFE943A44"}, "hue-pair"},
		{"identifiant manquant", map[string]string{"HUE_HOST": "192.168.1.101", "HUE_APP_KEY": "k"}, "ensemble"},
		{"hôte seul", map[string]string{"HUE_HOST": "192.168.1.101"}, "ensemble"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolate(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			_, err := Load()
			switch {
			case tc.wantErr == "" && err != nil:
				t.Errorf("erreur inattendue: %v", err)
			case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
				t.Errorf("erreur = %v, attendu une erreur contenant %q", err, tc.wantErr)
			}
		})
	}
}

func TestLocation(t *testing.T) {
	isolate(t)
	t.Setenv("DOMOTIC_TIMEZONE", "Europe/Zurich")
	t.Setenv("DOMOTIC_LATITUDE", "46.52")
	t.Setenv("DOMOTIC_LONGITUDE", "6.63")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	l := cfg.Location
	if l.TimeZone.String() != "Europe/Zurich" || !l.HasCoordinates || l.Latitude != 46.52 || l.Longitude != 6.63 {
		t.Errorf("emplacement = %+v", l)
	}
}

func TestLocationDefaultsAndErrors(t *testing.T) {
	isolate(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Location.TimeZone != time.UTC || cfg.Location.HasCoordinates {
		t.Errorf("par défaut : %+v", cfg.Location)
	}

	for name, env := range map[string]map[string]string{
		"fuseau inconnu":      {"DOMOTIC_TIMEZONE": "Mars/Olympus"},
		"latitude seule":      {"DOMOTIC_LATITUDE": "46.5"},
		"latitude hors plage": {"DOMOTIC_LATITUDE": "95", "DOMOTIC_LONGITUDE": "6"},
		"longitude illisible": {"DOMOTIC_LATITUDE": "46", "DOMOTIC_LONGITUDE": "est"},
	} {
		t.Run(name, func(t *testing.T) {
			isolate(t)
			for k, v := range env {
				t.Setenv(k, v)
			}
			if _, err := Load(); err == nil {
				t.Error("attendu une erreur")
			}
		})
	}
}
