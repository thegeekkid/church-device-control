package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/pborges/tapo"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
)

func TestKasaSetupAccountFallback(t *testing.T) {
	userHash := sha1.Sum([]byte("owner@example.org"))
	passHash := sha1.Sum([]byte("kasaSetup"))
	auth := sha256.Sum256(append(userHash[:], passHash[:]...))
	remote := bytes.Repeat([]byte{42}, 16)
	var local []byte
	var mu sync.Mutex
	var authenticated bool
	relay := 1
	hash := func(b ...[]byte) []byte {
		var v []byte
		for _, p := range b {
			v = append(v, p...)
		}
		h := sha256.Sum256(v)
		return h[:]
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		body, _ := io.ReadAll(r.Body)
		switch r.URL.Path {
		case "/app/handshake1":
			local = append([]byte(nil), body...)
			w.Header().Set("Set-Cookie", "TP_SESSIONID=test-session")
			w.Write(append(append([]byte(nil), remote...), hash(local, remote, auth[:])...))
		case "/app/handshake2":
			if !bytes.Equal(body, hash(remote, local, auth[:])) {
				w.WriteHeader(403)
				return
			}
			authenticated = true
		case "/app/request":
			if !authenticated {
				w.WriteHeader(403)
				return
			}
			seq, _ := strconv.ParseInt(r.URL.Query().Get("seq"), 10, 32)
			encoded := make([]byte, 4)
			binary.BigEndian.PutUint32(encoded, uint32(seq))
			key := hash([]byte("lsk"), local, remote, auth[:])
			ivSeed := hash([]byte("iv"), local, remote, auth[:])
			iv := append(append([]byte(nil), ivSeed[:12]...), encoded...)
			sign := hash([]byte("ldk"), local, remote, auth[:])
			block, _ := aes.NewCipher(key[:16])
			if len(body) < 48 || !bytes.Equal(body[:32], hash(sign[:28], encoded, body[32:])) {
				t.Error("invalid command signature")
				w.WriteHeader(400)
				return
			}
			command := make([]byte, len(body)-32)
			cipher.NewCBCDecrypter(block, iv).CryptBlocks(command, body[32:])
			command = command[:len(command)-int(command[len(command)-1])]
			var req struct {
				System struct {
					Set *struct{ State int } `json:"set_relay_state"`
				}
			}
			if e := json.Unmarshal(command, &req); e != nil {
				t.Error(e)
				w.WriteHeader(400)
				return
			}
			plain := []byte(fmt.Sprintf(`{"system":{"get_sysinfo":{"err_code":0,"alias":"Test HS103","model":"HS103","deviceId":"hs103-test","relay_state":%d}}}`, relay))
			if req.System.Set != nil {
				relay = req.System.Set.State
				plain = []byte(`{"system":{"set_relay_state":{"err_code":0}}}`)
			}
			pad := 16 - len(plain)%16
			plain = append(plain, bytes.Repeat([]byte{byte(pad)}, pad)...)
			encrypted := make([]byte, len(plain))
			cipher.NewCBCEncrypter(block, iv).CryptBlocks(encrypted, plain)
			w.Write(append(hash(sign[:28], encoded, encrypted), encrypted...))
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	_, port, _ := net.SplitHostPort(server.Listener.Addr().String())
	n, _ := strconv.Atoi(port)
	old, oldSetup := connectKasa, setupKasaClient
	defer func() { connectKasa = old; setupKasaClient = oldSetup }()
	connectKasa = func(ip string) (*tapo.Strip, error) {
		return tapo.New(ip, tapo.WithHTTPPort(n), tapo.WithDisableLegacy(), tapo.WithCredentials("Owner@Example.org", "normal-account-password"))
	}
	setupKasaClient = func(ip, u string) (*tapo.Strip, error) {
		return tapo.New(ip, tapo.WithHTTPPort(n), tapo.WithDisableLegacy(), tapo.WithCredentials(u, "kasaSetup"))
	}
	secrets.Lock()
	previous := secrets.User
	secrets.User = "Owner@Example.org"
	secrets.Unlock()
	defer func() { secrets.Lock(); secrets.User = previous; secrets.Unlock() }()
	tv, e := inspectKasa("127.0.0.1")
	if e != nil || tv.Power != "PowerOn" || tv.Model != "HS103" {
		t.Fatalf("%+v %v", tv, e)
	}
	for _, c := range []struct{ cmd, power string }{{"PowerOff", "PowerOff"}, {"PowerOn", "PowerOn"}} {
		if e := devicePower(tv, c.cmd); e != nil {
			t.Fatal(e)
		}
		got, e := inspectKasa(tv.IP)
		if e != nil || got.Power != c.power {
			t.Fatalf("%s: %+v %v", c.cmd, got, e)
		}
	}

}
