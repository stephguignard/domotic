package hue

import (
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"fmt"
	"strings"
)

// Le pont présente un certificat signé par la CA privée de Signify
// (« root-bridge »), absente des magasins système. Elle est embarquée.
// Empreinte SHA-256 vérifiée à l'intégration contre deux sources
// indépendantes, et contre le certificat d'un pont réel :
// F0:BD:8E:65:09:E8:2F:77:4D:63:BC:00:9D:53:88:C9:69:FE:3D:CF:7D:6D:54:1D:63:51:B7:2B:89:8D:8A:CF
//
//go:embed hue-root-bridge-ca.crt
var rootCA []byte

// tlsConfig authentifie le pont par sa CA et son identifiant.
//
// Pourquoi InsecureSkipVerify, contrairement à TaHoma : le certificat du pont
// porte son identifiant dans le seul Common Name, sans extension Subject
// Alternative Name. Depuis Go 1.15, la vérification standard du nom d'hôte
// ignore le CN et rejette donc tout certificat de pont, quel que soit le
// ServerName. InsecureSkipVerify ne sert ici qu'à débrayer cette vérification
// standard ; VerifyConnection la remplace par une vérification complète —
// chaîne jusqu'à la CA embarquée, usage serveur, CN égal à l'identifiant
// attendu. Rien n'est accepté que la vérification standard aurait accepté pour
// un certificat conforme.
func tlsConfig(bridgeID string, pool *x509.CertPool) (*tls.Config, error) {
	if bridgeID == "" {
		return nil, fmt.Errorf("identifiant du pont Hue manquant")
	}

	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true, //nolint:gosec // remplacée par VerifyConnection, voir ci-dessus
		VerifyConnection: func(cs tls.ConnectionState) error {
			return verifyBridge(cs.PeerCertificates, pool, bridgeID)
		},
	}, nil
}

// signifyRoots retourne le magasin réduit à la CA embarquée.
func signifyRoots() (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(rootCA) {
		return nil, fmt.Errorf("CA Hue illisible")
	}
	return pool, nil
}

// verifyBridge vérifie la chaîne présentée et l'identité du pont.
func verifyBridge(certs []*x509.Certificate, roots *x509.CertPool, bridgeID string) error {
	if len(certs) == 0 {
		return fmt.Errorf("le pont Hue n'a présenté aucun certificat")
	}
	leaf := certs[0]

	intermediates := x509.NewCertPool()
	for _, c := range certs[1:] {
		intermediates.AddCert(c)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err != nil {
		return fmt.Errorf("certificat du pont Hue non reconnu: %w", err)
	}

	// L'identifiant affiché par l'application est en majuscules, le CN en
	// minuscules.
	if !strings.EqualFold(leaf.Subject.CommonName, bridgeID) {
		return fmt.Errorf("certificat émis pour le pont %q, attendu %q", leaf.Subject.CommonName, bridgeID)
	}
	return nil
}
