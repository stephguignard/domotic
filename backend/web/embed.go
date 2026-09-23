// Package web embarque le frontend Angular compilé et le sert depuis le
// binaire.
//
// Servir le frontend depuis le même processus évite un conteneur nginx
// supplémentaire : sur un NAS qui ne dispose que d'un gigaoctet de RAM, c'est
// la différence entre une et deux images à faire tourner.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// dist contient le build de production Angular, copié par `make build`. Sur un
// dépôt fraîchement cloné il ne contient que .gitkeep : le répertoire doit
// exister pour que la directive embed compile, mais son contenu est un
// artefact de build, jamais versionné.
//
// Le préfixe `all:` inclut les fichiers commençant par un point ou un
// underscore, qu'embed ignorerait sinon.
//
//go:embed all:dist
var dist embed.FS

// placeholderHTML est servi tant que le frontend n'a pas été compilé. Le
// backend reste alors pleinement utilisable via son API et sa documentation.
const placeholderHTML = `<!doctype html>
<html lang="fr">
<meta charset="utf-8">
<title>Domotic — frontend non compilé</title>
<body style="font-family: system-ui, sans-serif; max-width: 40rem; margin: 4rem auto; line-height: 1.6">
  <h1>Frontend non compilé</h1>
  <p>Le build Angular n'a pas été copié dans <code>backend/web/dist/</code>. Le backend fonctionne normalement.</p>
  <p>Pour compiler et embarquer le frontend : <code>make build</code></p>
  <p>En développement, utilisez <code>make dev</code> et ouvrez <a href="http://localhost:4200">localhost:4200</a>.</p>
  <ul>
    <li><a href="/docs">Documentation de l'API</a></li>
    <li><a href="/api/health">État du service</a></li>
  </ul>
</body>
</html>
`

// Handler retourne un handler servant les fichiers statiques du frontend.
//
// Toute route inconnue retombe sur index.html : le routage d'une application
// Angular est résolu côté navigateur, et un rechargement sur /devices doit
// donc renvoyer l'application, pas une 404.
func Handler() (http.Handler, error) {
	root, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, err
	}

	if !exists(root, "index.html") {
		return placeholderHandler(), nil
	}

	files := http.FileServer(http.FS(root))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")

		if name != "" && exists(root, name) {
			// Les fichiers émis par Angular portent une empreinte dans leur nom
			// et peuvent donc être mis en cache agressivement. index.html, lui,
			// doit toujours être revalidé, sous peine de servir indéfiniment une
			// version périmée après un déploiement.
			if isFingerprinted(name) {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			files.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Cache-Control", "no-cache")
		r = r.Clone(r.Context())
		r.URL.Path = "/"
		files.ServeHTTP(w, r)
	}), nil
}

func placeholderHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		//nolint:errcheck // page statique : une écriture partielle n'appelle aucun traitement
		w.Write([]byte(placeholderHTML))
	})
}

func exists(root fs.FS, name string) bool {
	f, err := root.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()

	info, err := f.Stat()
	return err == nil && !info.IsDir()
}

// isFingerprinted reconnaît les noms de fichiers versionnés par le build
// Angular, de la forme main-PZFTEJS6.js ou chunk-Dl5xfOCj.js.
//
// L'empreinte est encodée en base36 et mêle donc chiffres et lettres des deux
// casses — la traiter comme de l'hexadécimal raterait la plupart des fichiers.
func isFingerprinted(name string) bool {
	base := path.Base(name)
	ext := path.Ext(base)
	if ext != ".js" && ext != ".css" {
		return false
	}

	stem := strings.TrimSuffix(base, ext)
	i := strings.LastIndex(stem, "-")
	if i < 0 || len(stem)-i-1 < 8 {
		return false
	}

	for _, c := range stem[i+1:] {
		isAlnum := (c >= '0' && c <= '9') ||
			(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		if !isAlnum {
			return false
		}
	}
	return true
}
