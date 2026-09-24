// Commande domotic : service d'agrégation domotique.
//
// Sans sous-commande, le serveur démarre. La sous-commande `openapi` écrit la
// spécification sans démarrer quoi que ce soit, ce qui permet de régénérer le
// client TypeScript du frontend depuis le Makefile ou une CI.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/danielgtaylor/huma/v2/humacli"
	"github.com/spf13/cobra"

	"github.com/stephguignard/domotic/internal/api"
	"github.com/stephguignard/domotic/internal/config"
	"github.com/stephguignard/domotic/internal/hue"
	"github.com/stephguignard/domotic/internal/netatmo"
	"github.com/stephguignard/domotic/internal/poller"
	"github.com/stephguignard/domotic/internal/shelly"
	"github.com/stephguignard/domotic/internal/store"
	"github.com/stephguignard/domotic/internal/tahoma"
	"github.com/stephguignard/domotic/web"
)

// version est remplacée au build via -ldflags "-X main.version=...".
var version = "dev"

// Options porte les paramètres exposés en ligne de commande. La configuration
// de fond vient de l'environnement, via le package config.
type Options struct {
	Debug bool `help:"Activer les logs de debug"`
}

func main() {
	cli := humacli.New(func(hooks humacli.Hooks, opts *Options) {
		svc := &service{log: newLogger(opts.Debug)}

		// Toute l'initialisation vit dans les hooks : humacli exécute cette
		// fonction pour chaque sous-commande, y compris `openapi`, qui ne doit
		// ni ouvrir la base ni joindre le réseau.
		hooks.OnStart(svc.start)
		hooks.OnStop(svc.stop)
	})

	cli.Root().AddCommand(openAPICommand(), huePairCommand())
	cli.Run()
}

// service porte l'état d'exécution du serveur entre le démarrage et l'arrêt.
type service struct {
	log *slog.Logger

	srv        *http.Server
	store      *store.Store
	cancel     context.CancelFunc
	pollerDone chan struct{}
}

func (s *service) start() {
	cfg, err := config.Load()
	if err != nil {
		s.log.Error("configuration invalide", "error", err)
		os.Exit(1)
	}

	// Contexte de fond des boucles de polling, annulé à l'arrêt.
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		cancel()
		s.log.Error("ouverture de la base", "error", err)
		os.Exit(1)
	}
	s.store = st

	deps, err := buildDeps(cfg, st, s.log)
	if err != nil {
		cancel()
		st.Close()
		s.log.Error("initialisation des intégrations", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	api.Register(humago.New(mux, api.Config(version)), deps)
	api.RegisterAuthRoutes(mux, deps)

	frontend, err := web.Handler()
	if err != nil {
		cancel()
		st.Close()
		s.log.Error("chargement du frontend embarqué", "error", err)
		os.Exit(1)
	}
	// Enregistré en dernier sur la racine : le mux de la stdlib privilégie les
	// motifs les plus spécifiques, les routes /api/ restent donc prioritaires.
	mux.Handle("/", frontend)

	s.srv = &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	s.pollerDone = make(chan struct{})
	go func() {
		defer close(s.pollerDone)
		deps.Poller.Run(ctx)
	}()

	logStartup(s.log, cfg)
	if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.log.Error("serveur HTTP", "error", err)
		os.Exit(1)
	}
}

func (s *service) stop() {
	s.log.Info("arrêt en cours")

	if s.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.srv.Shutdown(ctx); err != nil {
			s.log.Warn("arrêt du serveur HTTP", "error", err)
		}
	}

	// Laisser les boucles se terminer proprement : c'est ce qui permet au
	// listener TaHoma d'être libéré côté box.
	if s.cancel != nil {
		s.cancel()
	}
	if s.pollerDone != nil {
		select {
		case <-s.pollerDone:
		case <-time.After(10 * time.Second):
			s.log.Warn("arrêt des boucles de polling interrompu par le délai d'attente")
		}
	}

	if s.store != nil {
		if err := s.store.Close(); err != nil {
			s.log.Warn("fermeture de la base", "error", err)
		}
	}
}

// buildDeps construit les clients des sources activées. Une source non
// configurée reste nil, et les handlers le gèrent explicitement.
func buildDeps(cfg *config.Config, st *store.Store, log *slog.Logger) (api.Deps, error) {
	deps := api.Deps{Config: cfg, Store: st, Log: log}

	if cfg.Netatmo.Enabled() {
		deps.Netatmo = netatmo.NewClient(cfg, st, log.With("source", "netatmo"))
	} else {
		log.Warn("intégration Netatmo désactivée : NETATMO_CLIENT_ID et NETATMO_CLIENT_SECRET absents")
	}

	if cfg.Tahoma.Enabled() {
		tc, err := tahoma.NewClient(cfg, log.With("source", "tahoma"))
		if err != nil {
			return api.Deps{}, err
		}
		deps.Tahoma = tc
	} else {
		log.Warn("intégration TaHoma désactivée : TAHOMA_HOST, TAHOMA_PIN et TAHOMA_TOKEN absents")
	}

	if cfg.Shelly.Enabled() {
		deps.Shelly = shelly.NewClient(cfg, log.With("source", "shelly"))
	} else {
		log.Warn("intégration Shelly désactivée : SHELLY_HOSTS absent")
	}

	if cfg.Hue.Enabled() {
		hc, err := hue.NewClient(cfg.Hue, log.With("source", "hue"))
		if err != nil {
			return api.Deps{}, err
		}
		deps.Hue = hc
	} else {
		log.Warn("intégration Hue désactivée : HUE_HOST, HUE_BRIDGE_ID et HUE_APP_KEY absents")
	}

	deps.Poller = poller.New(cfg, st, poller.Clients{
		Netatmo: deps.Netatmo,
		Tahoma:  deps.Tahoma,
		Shelly:  deps.Shelly,
		Hue:     deps.Hue,
	}, log)
	return deps, nil
}

// openAPICommand écrit la spécification OpenAPI sans démarrer le serveur ni
// toucher à la base.
func openAPICommand() *cobra.Command {
	var (
		output     string
		downgrade  bool
		yamlFormat bool
	)

	cmd := &cobra.Command{
		Use:   "openapi",
		Short: "Écrire la spécification OpenAPI",
		Long: "Écrit la spécification OpenAPI du service. Par défaut en 3.1 ; --downgrade produit " +
			"du 3.0.3, mieux supporté par le générateur typescript-angular.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Les handlers ne sont jamais appelés ici : seules leurs signatures
			// comptent pour dériver le schéma, d'où les dépendances vides.
			a := humago.New(http.NewServeMux(), api.Config(version))
			api.Register(a, api.Deps{})

			spec, err := marshalSpec(a, downgrade, yamlFormat)
			if err != nil {
				return err
			}

			if output == "" || output == "-" {
				_, err := cmd.OutOrStdout().Write(spec)
				return err
			}
			if err := os.WriteFile(output, spec, 0o644); err != nil {
				return fmt.Errorf("écriture de %s: %w", output, err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "spécification écrite dans %s\n", output)
			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "-", "Fichier de sortie, '-' pour la sortie standard")
	cmd.Flags().BoolVar(&downgrade, "downgrade", false, "Produire de l'OpenAPI 3.0.3 au lieu de 3.1")
	cmd.Flags().BoolVar(&yamlFormat, "yaml", false, "Produire du YAML au lieu du JSON")
	return cmd
}

// huePairCommand obtient la clé d'application du pont Hue.
//
// Elle lit HUE_HOST et HUE_BRIDGE_ID directement, sans passer par
// config.Load : celle-ci refuse précisément la configuration « pont renseigné,
// clé absente » qui est la situation normale avant l'appairage.
func huePairCommand() *cobra.Command {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "hue-pair",
		Short: "Obtenir la clé d'application du pont Hue",
		Long: "Demande une clé d'application au pont désigné par HUE_HOST et HUE_BRIDGE_ID. " +
			"Appuyer sur le bouton du pont pendant l'attente, puis reporter la clé dans HUE_APP_KEY.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			hc := config.HueConfig{Host: os.Getenv("HUE_HOST"), BridgeID: os.Getenv("HUE_BRIDGE_ID")}
			if hc.Host == "" || hc.BridgeID == "" {
				return fmt.Errorf("renseigner HUE_HOST et HUE_BRIDGE_ID avant l'appairage")
			}
			client, err := hue.NewClient(hc, newLogger(false))
			if err != nil {
				return err
			}

			out := cmd.ErrOrStderr()
			fmt.Fprintf(out, "Appuyez sur le bouton du pont %s (%s)… attente jusqu'à %s.\n", hc.BridgeID, hc.Host, timeout)

			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			hostname, _ := os.Hostname()
			key, err := client.Pair(ctx, "domotic#"+hostname, func() { fmt.Fprint(out, ".") })
			fmt.Fprintln(out)
			if err != nil {
				return err
			}

			fmt.Fprintln(out, "Clé obtenue. Ajoutez cette ligne à .env :")
			fmt.Fprintf(cmd.OutOrStdout(), "HUE_APP_KEY=%s\n", key)
			return nil
		},
	}
	cmd.Flags().DurationVar(&timeout, "timeout", time.Minute, "Durée d'attente de l'appui sur le bouton")
	return cmd
}

func marshalSpec(a huma.API, downgrade, yamlFormat bool) ([]byte, error) {
	spec := a.OpenAPI()

	switch {
	case downgrade && yamlFormat:
		return spec.DowngradeYAML()
	case yamlFormat:
		return spec.YAML()
	}

	var (
		raw []byte
		err error
	)
	if downgrade {
		raw, err = spec.Downgrade()
	} else {
		raw, err = spec.MarshalJSON()
	}
	if err != nil {
		return nil, err
	}

	// Huma produit le JSON sur une seule ligne ; l'indenter garde le fichier
	// lisible dans les diffs Git.
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return json.MarshalIndent(v, "", "  ")
}

func newLogger(debug bool) *slog.Logger {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

func logStartup(log *slog.Logger, cfg *config.Config) {
	log.Info("démarrage du service",
		"version", version,
		"port", cfg.Port,
		"db", cfg.DBPath,
		"netatmo", cfg.Netatmo.Enabled(),
		"tahoma", cfg.Tahoma.Enabled(),
		"shelly", cfg.Shelly.Enabled(),
		"hue", cfg.Hue.Enabled(),
	)
	if cfg.Netatmo.Enabled() {
		log.Info("authentification Netatmo disponible sur " + cfg.PublicURL + "/auth/netatmo")
	}
}
