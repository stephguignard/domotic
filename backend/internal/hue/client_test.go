package hue

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"errors"
	"io"
	"log"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stephguignard/domotic/internal/command"
	"github.com/stephguignard/domotic/internal/config"
)

const testBridgeID = "ECB5FAFFFE943A44"

// testPKI imite la PKI Signify : une CA, et un certificat de pont qui ne porte
// son identifiant que dans le CN, sans SAN.
type testPKI struct {
	roots *x509.CertPool
	ca    *x509.Certificate
	caKey *ecdsa.PrivateKey
}

func newTestPKI(t *testing.T) *testPKI {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Country: []string{"NL"}, Organization: []string{"Philips Hue"}, CommonName: "root-bridge"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	ca, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	return &testPKI{roots: pool, ca: ca, caKey: key}
}

// leaf émet un certificat de pont pour le CN donné.
func (p *testPKI) leaf(t *testing.T, cn string) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{Country: []string{"NL"}, Organization: []string{"Philips Hue"}, CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, p.ca, &key.PublicKey, p.caKey)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// fakeBridge sert les ressources CLIP v2 d'un salon à deux lampes.
type fakeBridge struct {
	mu   sync.Mutex
	puts map[string]string // chemin → corps
}

func (f *fakeBridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("hue-application-key") != "cle" && r.URL.Path != "/api" {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"errors":[{"description":"unauthorized user"}],"data":[]}`))
		return
	}
	reply := func(data string) {
		w.Write([]byte(`{"errors":[],"data":` + data + `}`))
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/light":
		reply(`[
			{"id":"l-1","type":"light","owner":{"rid":"d-1","rtype":"device"},"metadata":{"name":"Plafonnier"},
			 "on":{"on":true},"dimming":{"brightness":42.5}},
			{"id":"l-2","type":"light","owner":{"rid":"d-2","rtype":"device"},"metadata":{"name":"Lampe"},
			 "on":{"on":false}}
		]`)
	case r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/room":
		reply(`[{"id":"r-1","metadata":{"name":"Salon"},"children":[{"rid":"d-1","rtype":"device"}]}]`)
	case r.Method == http.MethodGet && r.URL.Path == "/clip/v2/resource/zigbee_connectivity":
		reply(`[{"id":"z-2","owner":{"rid":"d-2","rtype":"device"},"status":"connectivity_issue"}]`)
	case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/clip/v2/resource/light/"):
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.puts[r.URL.Path] = string(body)
		f.mu.Unlock()
		reply(`[{"rid":"l-1","rtype":"light"}]`)
	default:
		http.NotFound(w, r)
	}
}

// startBridge démarre un faux pont présentant un certificat émis pour cn.
func startBridge(t *testing.T, pki *testPKI, cn string, h http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewUnstartedServer(h)
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{pki.leaf(t, cn)}}
	// Les refus de handshake sont le comportement attendu de certains tests.
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
}

func testClient(t *testing.T, srv *httptest.Server, roots *x509.CertPool, bridgeID string) *Client {
	t.Helper()
	c, err := newClient(config.HueConfig{
		Host:     strings.TrimPrefix(srv.URL, "https://"),
		BridgeID: bridgeID,
		AppKey:   "cle",
	}, roots, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	return c
}

func TestEmbeddedCAIsReadable(t *testing.T) {
	if _, err := signifyRoots(); err != nil {
		t.Fatal(err)
	}
}

func TestTLSAcceptsBridgeWithCommonNameOnly(t *testing.T) {
	pki := newTestPKI(t)
	// CN en minuscules, identifiant configuré en majuscules : comme en vrai.
	srv := startBridge(t, pki, strings.ToLower(testBridgeID), &fakeBridge{puts: map[string]string{}})

	if _, err := testClient(t, srv, pki.roots, testBridgeID).FetchDevices(context.Background()); err != nil {
		t.Fatalf("pont légitime refusé: %v", err)
	}
}

func TestTLSRejectsOtherBridge(t *testing.T) {
	pki := newTestPKI(t)
	srv := startBridge(t, pki, "0017880000000000", &fakeBridge{puts: map[string]string{}})

	_, err := testClient(t, srv, pki.roots, testBridgeID).FetchDevices(context.Background())
	if err == nil || !strings.Contains(err.Error(), "certificat émis pour le pont") {
		t.Fatalf("attendu un refus sur l'identifiant, obtenu %v", err)
	}
}

func TestTLSRejectsOtherCA(t *testing.T) {
	pki, other := newTestPKI(t), newTestPKI(t)
	srv := startBridge(t, other, strings.ToLower(testBridgeID), &fakeBridge{puts: map[string]string{}})

	_, err := testClient(t, srv, pki.roots, testBridgeID).FetchDevices(context.Background())
	if err == nil || !strings.Contains(err.Error(), "non reconnu") {
		t.Fatalf("attendu un refus sur la CA, obtenu %v", err)
	}
}

func TestFetchDevicesMapsRoomsAndReachability(t *testing.T) {
	pki := newTestPKI(t)
	srv := startBridge(t, pki, strings.ToLower(testBridgeID), &fakeBridge{puts: map[string]string{}})

	devices, err := testClient(t, srv, pki.roots, testBridgeID).FetchDevices(context.Background())
	if err != nil {
		t.Fatalf("FetchDevices: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("attendu 2 lumières, obtenu %d", len(devices))
	}

	ceiling, lamp := devices[0], devices[1]
	if ceiling.Source != "hue" || ceiling.Kind != "light" || ceiling.Name != "Plafonnier" {
		t.Errorf("plafonnier mal converti: %+v", ceiling)
	}
	if ceiling.Room != "Salon" || !ceiling.Reachable {
		t.Errorf("plafonnier : pièce %q, joignable %v", ceiling.Room, ceiling.Reachable)
	}
	if ceiling.State != `{"brightness":42.5,"on":true}` {
		t.Errorf("état = %s", ceiling.State)
	}
	// Lampe hors de toute pièce, liaison Zigbee dégradée, sans variateur.
	if lamp.Room != "" || lamp.Reachable || lamp.State != `{"on":false}` {
		t.Errorf("lampe mal convertie: %+v", lamp)
	}
}

func TestExecuteBodies(t *testing.T) {
	pki := newTestPKI(t)
	bridge := &fakeBridge{puts: map[string]string{}}
	srv := startBridge(t, pki, strings.ToLower(testBridgeID), bridge)
	c := testClient(t, srv, pki.roots, testBridgeID)
	ctx := context.Background()

	cases := []struct {
		cmd    string
		params []any
		want   string
	}{
		{"on", nil, `{"on":{"on":true}}`},
		{"off", nil, `{"on":{"on":false}}`},
		{"setBrightness", []any{60.0}, `{"dimming":{"brightness":60},"on":{"on":true}}`},
		{"setBrightness", []any{0.0}, `{"on":{"on":false}}`},
	}
	for _, tc := range cases {
		if _, err := c.Execute(ctx, "l-1", tc.cmd, tc.params); err != nil {
			t.Fatalf("%s: %v", tc.cmd, err)
		}
		if got := bridge.puts["/clip/v2/resource/light/l-1"]; got != tc.want {
			t.Errorf("%s %v : corps %s, attendu %s", tc.cmd, tc.params, got, tc.want)
		}
	}

	for _, bad := range []struct {
		cmd    string
		params []any
	}{{"open", nil}, {"setBrightness", nil}, {"setBrightness", []any{150.0}}, {"setBrightness", []any{"fort"}}} {
		if _, err := c.Execute(ctx, "l-1", bad.cmd, bad.params); !errors.Is(err, command.ErrUnsupported) {
			t.Errorf("%s %v : erreur %v, attendu command.ErrUnsupported", bad.cmd, bad.params, err)
		}
	}
}

func TestRejectedKeyNamesPairing(t *testing.T) {
	pki := newTestPKI(t)
	srv := startBridge(t, pki, strings.ToLower(testBridgeID), &fakeBridge{puts: map[string]string{}})
	c := testClient(t, srv, pki.roots, testBridgeID)
	c.appKey = "perimee"

	_, err := c.FetchDevices(context.Background())
	if err == nil || !strings.Contains(err.Error(), "hue-pair") {
		t.Errorf("attendu une erreur qui renvoie à hue-pair, obtenu %v", err)
	}
}

func TestPairWaitsForLinkButton(t *testing.T) {
	pairRetry = time.Millisecond
	t.Cleanup(func() { pairRetry = 2 * time.Second })

	pki := newTestPKI(t)
	attempts := 0
	srv := startBridge(t, pki, strings.ToLower(testBridgeID), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		attempts++
		if attempts < 2 {
			w.Write([]byte(`[{"error":{"type":101,"address":"","description":"link button not pressed"}}]`))
			return
		}
		w.Write([]byte(`[{"success":{"username":"cle-` + body["devicetype"] + `"}}]`))
	}))
	c := testClient(t, srv, pki.roots, testBridgeID)

	waits := 0
	key, err := c.Pair(context.Background(), "domotic#test", func() { waits++ })
	if err != nil {
		t.Fatalf("Pair: %v", err)
	}
	if key != "cle-domotic#test" || waits != 1 {
		t.Errorf("clé %q après %d attente(s)", key, waits)
	}
}

func TestReadEvents(t *testing.T) {
	stream := strings.Join([]string{
		": hi",
		"",
		"id: 1700000000:0",
		`data: [{"creationtime":"2026-09-24T10:00:00Z","id":"e-1","type":"update","data":[` +
			`{"id":"l-1","type":"light","owner":{"rid":"d-1","rtype":"device"},"on":{"on":false}},` +
			`{"id":"l-2","type":"light","dimming":{"brightness":12.6}}]}]`,
		"",
		"data: pas du json",
		"",
		`data: [{"id":"e-2","type":"delete","data":[{"id":"l-3","type":"light"}]}]`,
		"",
		"",
	}, "\n") // la dernière ligne vide clôt le message

	var got []Event
	lines := 0
	err := readEvents(strings.NewReader(stream), func(events []Event) {
		got = append(got, events...)
	}, func() { lines++ })
	if err != nil {
		t.Fatalf("readEvents: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("attendu 2 événements (le message illisible ignoré), obtenu %d", len(got))
	}
	up := got[0]
	if up.Type != "update" || len(up.Data) != 2 {
		t.Fatalf("événement mal lu: %+v", up)
	}
	if s := up.Data[0].LightState(); len(s) != 1 || s["on"] != false {
		t.Errorf("état partiel de l-1 = %v, attendu seulement on=false", s)
	}
	if s := up.Data[1].LightState(); len(s) != 1 || s["brightness"] != 12.6 {
		t.Errorf("état partiel de l-2 = %v, attendu seulement brightness", s)
	}
	if got[1].Type != "delete" || got[1].Data[0].ID != "l-3" {
		t.Errorf("suppression mal lue: %+v", got[1])
	}
	if lines == 0 {
		t.Error("alive n'a jamais été appelée")
	}
}

func TestStreamDeliversEventsThenReportsIdle(t *testing.T) {
	idleTimeout = 200 * time.Millisecond
	t.Cleanup(func() { idleTimeout = 5 * time.Minute })

	pki := newTestPKI(t)
	srv := startBridge(t, pki, strings.ToLower(testBridgeID), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/eventstream/clip/v2" || r.Header.Get("hue-application-key") != "cle" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(`data: [{"type":"update","data":[{"id":"l-1","type":"light","on":{"on":true}}]}]` + "\n\n"))
		w.(http.Flusher).Flush()
		<-r.Context().Done() // puis plus rien, comme un pont silencieux
	}))

	var got []Event
	err := testClient(t, srv, pki.roots, testBridgeID).Stream(context.Background(), func(events []Event) {
		got = append(got, events...)
	})
	if !errors.Is(err, ErrIdle) {
		t.Fatalf("erreur = %v, attendu ErrIdle", err)
	}
	if len(got) != 1 || got[0].Data[0].ID != "l-1" {
		t.Errorf("événements reçus: %+v", got)
	}
}

func TestStreamReturnsNilOnCancel(t *testing.T) {
	pki := newTestPKI(t)
	srv := startBridge(t, pki, strings.ToLower(testBridgeID), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := testClient(t, srv, pki.roots, testBridgeID).Stream(ctx, func([]Event) {}); err != nil {
		t.Errorf("annulation par l'appelant : attendu nil, obtenu %v", err)
	}
}
