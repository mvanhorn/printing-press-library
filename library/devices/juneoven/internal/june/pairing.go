package june

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha1" // #nosec G505 -- SHA-1 is mandated by June's SRP-6a pairing handshake (RFC 5054), not used for general-purpose hashing
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/nacl/secretbox"
)

// SRP-6a group: RFC 5054 8192-bit modulus, generator g=19, SHA-1 (the exact
// parameters the June app uses). Verified against app transcripts and a real oven.
var (
	srpN, _ = new(big.Int).SetString("FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74020BBEA63B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7EDEE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3DC2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F83655D23DCA3AD961C62F356208552BB9ED529077096966D670C354E4ABC9804F1746C08CA18217C32905E462E36CE3BE39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9DE2BCBF6955817183995497CEA956AE515D2261898FA051015728E5A8AAAC42DAD33170D04507A33A85521ABDF1CBA64ECFB850458DBEF0A8AEA71575D060C7DB3970F85A6E1E4C7ABF5AE8CDB0933D71E8C94E04A25619DCEE3D2261AD2EE6BF12FFA06D98A0864D87602733EC86A64521F2B18177B200CBBE117577A615D6C770988C0BAD946E208E24FA074E5AB3143DB5BFCE0FD108E4B82D120A92108011A723C12A787E6D788719A10BDBA5B2699C327186AF4E23C1A946834B6150BDA2583E9CA2AD44CE8DBBBC2DB04DE8EF92E8EFC141FBECAA6287C59474E6BC05D99B2964FA090C3A2233BA186515BE7ED1F612970CEE2D7AFB81BDD762170481CD0069127D5B05AA993B4EA988D8FDDC186FFB7DC90A6C08F4DF435C93402849236C3FAB4D27C7026C1D4DCB2602646DEC9751E763DBA37BDF8FF9406AD9E530EE5DB382F413001AEB06A53ED9027D831179727B0865A8918DA3EDBEBCF9B14ED44CE6CBACED4BB1BDB7F1447E6CC254B332051512BD7AF426FB8F401378CD2BF5983CA01C64B92ECF032EA15D1721D03F482D7CE6E74FEF6D55E702F46980C82B5A84031900B1C9E59E7C97FBEC7E8F323A97A7E36CC88BE0F1D45B7FF585AC54BD407B22B4154AACC8F6D7EBF48E1D814CC5ED20F8037E0A79715EEF29BE32806A1D58BB7C5DA76F550AA3D8A1FBFF0EB19CCB1A313D55CDA56C9EC2EF29632387FE8D76E3C0468043E8F663F4860EE12BF2D5B0B7474D6E694F91E6DBE115974A3926F12FEE5E438777CB6A932DF8CD8BEC4D073B931BA3BC832B68D9DD300741FA7BF8AFC47ED2576F6936BA424663AAB639C5AE4F5683423B4742BF1C978238F16CBE39D652DE3FDB8BEFC848AD922222E04A4037C0713EB57A81A23F0C73473FC646CEA306B4BCBC8862F8385DDFA9D4B7FA2C087E879683303ED5BDD3A062B3CF5B3A278A66D2A13F83F44F82DDF310EE074AB6A364597E899A0255DC164F31CC50846851DF9AB48195DED7EA1B1D510BD7EE74D73FAF36BC31ECFA268359046F4EB879F924009438B481C6CD7889A002ED5EE382BC9190DA6FC026E479558E4475677E9AA9E3050E2765694DFC81F56E880B96E7160C980DD98EDD3DFFFFFFFFFFFFFFFFF", 16)
	srpG    = big.NewInt(19)
)

func srpPadLen() int { return (srpN.BitLen() + 7) / 8 }

func sha1sum(chunks ...[]byte) []byte {
	h := sha1.New() // #nosec G401 -- SHA-1 required by June's SRP-6a pairing protocol; changing it breaks the oven handshake
	for _, c := range chunks {
		h.Write(c)
	}
	return h.Sum(nil)
}

func srpPad(x *big.Int) []byte {
	return x.FillBytes(make([]byte, srpPadLen()))
}

// srpServer is the SRP-6a server (the companion's role); the oven is the client.
type srpServer struct {
	salt []byte
	v    *big.Int // verifier
	b    *big.Int // server private exponent
	B    *big.Int // server public
}

func newSRPServer(password string, salt, bBytes []byte) *srpServer {
	x := new(big.Int).SetBytes(sha1sum(salt, sha1sum([]byte("user:"+password))))
	v := new(big.Int).Exp(srpG, x, srpN)
	k := new(big.Int).SetBytes(sha1sum(srpPad(srpN), srpPad(srpG)))
	b := new(big.Int).Mod(new(big.Int).SetBytes(bBytes), srpN)
	// B = (k*v + g^b) mod N
	B := new(big.Int).Exp(srpG, b, srpN)
	B.Add(B, new(big.Int).Mul(k, v))
	B.Mod(B, srpN)
	return &srpServer{salt: salt, v: v, b: b, B: B}
}

// secret computes the shared secret S from the client's public A (minimal bytes).
func (s *srpServer) secret(A *big.Int) []byte {
	u := new(big.Int).SetBytes(sha1sum(srpPad(A), srpPad(s.B)))
	base := new(big.Int).Exp(s.v, u, srpN)
	base.Mul(base, A)
	base.Mod(base, srpN)
	S := new(big.Int).Exp(base, s.b, srpN)
	return S.Bytes() // minimal big-endian (asUnsignedByteArray)
}

// dammTable is SpongyCastle's quasigroup for the pairing check digit.
var dammTable = [10][10]int{
	{0, 3, 1, 7, 5, 9, 8, 6, 4, 2}, {7, 0, 9, 2, 1, 5, 4, 8, 6, 3},
	{4, 2, 0, 6, 8, 7, 1, 3, 5, 9}, {1, 7, 5, 0, 9, 8, 3, 4, 2, 6},
	{6, 1, 2, 3, 0, 4, 5, 9, 7, 8}, {3, 6, 7, 4, 2, 0, 9, 5, 8, 1},
	{5, 8, 6, 9, 7, 2, 0, 1, 3, 4}, {8, 9, 4, 5, 3, 6, 2, 0, 1, 7},
	{9, 4, 3, 8, 6, 1, 7, 2, 0, 5}, {2, 5, 8, 1, 4, 3, 6, 7, 9, 0},
}

// damm returns the Damm check digit for a numeric string.
func damm(s string) int {
	r := 0
	for _, ch := range s {
		r = dammTable[r][int(ch-'0')]
	}
	return r
}

// PairProgress is emitted during pairing so the CLI can display the code and status.
type PairProgress struct {
	Code   string // the 8-digit code to type on the oven (set once)
	Status string // human status line
}

// Pair runs the full self-pairing flow: register an anonymous companion, request
// a code, wait for the oven's SRP handshake after the user enters the code, seal
// the companion key, and persist the resulting Identity. progress receives the
// code and status updates.
func Pair(ctx context.Context, deviceName string, progress func(PairProgress)) (*Identity, error) {
	httpc := &http.Client{Timeout: 15 * time.Second}

	// 1. Register a fresh companion device.
	devID := randHex(16)
	password := randHex(16)
	token, err := registerAnon(ctx, httpc, devID, password, deviceName)
	if err != nil {
		return nil, err
	}
	progress(PairProgress{Status: "registered companion device"})

	// 2. Generate the keys the oven will trust.
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return nil, err
	}
	signPriv := ed25519.NewKeyFromSeed(seed)
	signPub := signPriv.Public().(ed25519.PublicKey)
	boxPub, _, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	// 3. Open the messaging socket and listen for the oven's A. Wait for the
	// socket to actually connect (signalled on wsReady) before requesting the
	// code, so the oven's A frame can't arrive before the reader is listening.
	aCh := make(chan *big.Int, 1)
	wsReady := make(chan struct{})
	wsCtx, wsCancel := context.WithCancel(ctx)
	defer wsCancel()
	go listenForA(wsCtx, token, aCh, wsReady, progress)
	select {
	case <-wsReady:
	case <-time.After(15 * time.Second):
		return nil, fmt.Errorf("could not open the June messaging socket")
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// 4. Request a pairing code.
	code, err := requestCode(ctx, httpc, token)
	if err != nil {
		return nil, err
	}
	base := code + fmt.Sprintf("%02d", randInt(100))
	shown := base + fmt.Sprintf("%d", damm(base))
	progress(PairProgress{Code: shown, Status: "waiting for you to enter the code on the oven"})

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	bBytes := make([]byte, 32)
	if _, err := rand.Read(bBytes); err != nil {
		return nil, err
	}
	srv := newSRPServer(shown, salt, bBytes)

	// 5. Wait for A (up to 5 minutes).
	var A *big.Int
	select {
	case A = <-aCh:
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("timed out waiting for the oven — was the code entered?")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	progress(PairProgress{Status: "received oven handshake, completing pairing"})

	// 6. Derive the seal key and encrypt companion_info.
	S := srv.secret(A)
	K := blake2b.Sum256(S)
	tz := time.Local.String()
	if tz == "" || tz == "Local" {
		tz = "America/Los_Angeles" // fall back when the host zone is unnamed
	}
	companion := map[string]string{
		"companion_id":          devID,
		"companion_name":        deviceName,
		"public_signing_key":    base64.StdEncoding.EncodeToString(signPub),
		"public_encryption_key": base64.StdEncoding.EncodeToString(boxPub[:]),
		"timezone":              tz,
		"platform":              "Android",
	}
	pj, _ := json.Marshal(companion)
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	sealed := secretbox.Seal(nil, pj, &nonce, &K)
	companionInfo := base64.StdEncoding.EncodeToString(append(nonce[:], sealed...))

	// 7. POST companion key_info.
	if err := postCompanion(ctx, httpc, token, code, srv.salt, srpPad(srv.B), companionInfo); err != nil {
		return nil, err
	}

	// 8. Poll for the associated oven_id (do NOT delete the session early).
	ovenID := ""
	for i := 0; i < 20; i++ {
		time.Sleep(3 * time.Second)
		if id, ok := fetchOvenID(ctx, httpc, token, devID); ok {
			ovenID = id
			break
		}
	}
	if ovenID == "" {
		return nil, fmt.Errorf("pairing did not complete — oven never associated")
	}

	id := &Identity{
		DeviceID:    devID,
		DeviceName:  deviceName,
		Password:    password,
		Ed25519Seed: hex.EncodeToString(seed),
		OvenID:      ovenID,
		AccessToken: token,
	}
	if err := id.Save(); err != nil {
		return nil, err
	}
	progress(PairProgress{Status: "paired"})
	return id, nil
}

func registerAnon(ctx context.Context, httpc *http.Client, devID, password, name string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"password": password, "device_id": devID, "client_id": clientID,
		"client_secret": clientSecret, "device_type": "companion", "device_name": name,
		"platform": "android", "version": appVersion, "platform_version": "34",
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, APIBase+"/2/devices/register", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)
	resp, err := httpc.Do(req)
	if err != nil {
		return "", fmt.Errorf("registering device: %w", err)
	}
	defer resp.Body.Close()
	var tr tokenResp
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", err
	}
	if tr.Token.AccessToken == "" {
		return "", fmt.Errorf("registration returned no token (HTTP %d)", resp.StatusCode)
	}
	return tr.Token.AccessToken, nil
}

func requestCode(ctx context.Context, httpc *http.Client, token string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, APIBase+"/2/devices/pairing", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", UserAgent)
	resp, err := httpc.Do(req)
	if err != nil {
		return "", fmt.Errorf("requesting pairing code: %w", err)
	}
	defer resp.Body.Close()
	var pr struct {
		PIN struct {
			Code string `json:"code"`
		} `json:"pin"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return "", err
	}
	if pr.PIN.Code == "" {
		return "", fmt.Errorf("no pairing code returned (HTTP %d)", resp.StatusCode)
	}
	return pr.PIN.Code, nil
}

func postCompanion(ctx context.Context, httpc *http.Client, token, code string, salt, B []byte, companionInfo string) error {
	body, _ := json.Marshal(map[string]any{
		"key_info": map[string]string{
			"salt":           base64.StdEncoding.EncodeToString(salt),
			"B":              base64.StdEncoding.EncodeToString(B),
			"companion_info": companionInfo,
		},
	})
	url := fmt.Sprintf("%s/2/devices/pairing/%s/companion", APIBase, code)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", UserAgent)
	resp, err := httpc.Do(req)
	if err != nil {
		return fmt.Errorf("posting companion key: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("oven rejected companion key: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func fetchOvenID(ctx context.Context, httpc *http.Client, token, devID string) (string, bool) {
	url := fmt.Sprintf("%s/2/devices/%s/associated", APIBase, devID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", UserAgent)
	resp, err := httpc.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	var a struct {
		Devices []struct {
			OvenID string `json:"oven_id"`
		} `json:"devices"`
	}
	if json.NewDecoder(resp.Body).Decode(&a) != nil {
		return "", false
	}
	if len(a.Devices) > 0 && a.Devices[0].OvenID != "" {
		return a.Devices[0].OvenID, true
	}
	return "", false
}

var longB64 = regexp.MustCompile(`"([A-Za-z0-9+/=]{300,})"`)

// listenForA connects to the messaging socket and pushes the oven's SRP public A
// (a long base64 string inside a 10026 frame) to ch.
func listenForA(ctx context.Context, token string, ch chan<- *big.Int, ready chan<- struct{}, progress func(PairProgress)) {
	d := websocket.Dialer{HandshakeTimeout: 15 * time.Second, EnableCompression: false}
	h := http.Header{}
	h.Set("Authorization", "Bearer "+token)
	h.Set("User-Agent", UserAgent)
	conn, _, err := d.DialContext(ctx, WSURL, h)
	if err != nil {
		return
	}
	defer conn.Close()
	close(ready) // socket is connected; safe for the caller to request the code
	for {
		if ctx.Err() != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(310 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var env struct {
			MessageCode int             `json:"message_code"`
			Data        json.RawMessage `json:"data"`
		}
		if json.Unmarshal(msg, &env) != nil {
			continue
		}
		if env.MessageCode == 10027 {
			progress(PairProgress{Status: "oven rejected the session (10027) — ensure it's idle, door closed, on Wi-Fi, then retry"})
			return
		}
		if env.MessageCode != 10026 {
			continue
		}
		m := longB64.FindSubmatch(env.Data)
		if m == nil {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(string(m[1]))
		if err != nil {
			continue
		}
		select {
		case ch <- new(big.Int).SetBytes(raw):
		default:
		}
		return
	}
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func randInt(max int) int {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return int(uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3])) % max
}
