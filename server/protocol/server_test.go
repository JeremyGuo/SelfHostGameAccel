package protocol

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

type testRig struct {
	server *httptest.Server
	client *http.Client
}

func newTestRig(t *testing.T) *testRig {
	return newTestRigWithPath(t, "")
}

func newTestRigWithPath(t *testing.T, dataPath string) *testRig {
	t.Helper()
	s, err := NewServerWithStorage(dataPath)
	if err != nil {
		t.Fatalf("server init: %v", err)
	}
	serverTLS, clientTLS, err := GenerateTLSConfigs()
	if err != nil {
		t.Fatalf("tls: %v", err)
	}
	ts := httptest.NewUnstartedServer(s.Handler())
	ts.TLS = serverTLS
	ts.StartTLS()
	client := ts.Client()
	transport := client.Transport.(*http.Transport)
	transport.TLSClientConfig = clientTLS
	return &testRig{server: ts, client: client}
}

func (r *testRig) close() {
	r.server.Close()
}

func postJSON(t *testing.T, client *http.Client, url string, reqBody any, respBody any) *http.Response {
	t.Helper()
	raw, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := client.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("post %s: %v", url, err)
	}
	if respBody != nil {
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(respBody); err != nil {
			t.Fatalf("decode %s: %v", url, err)
		}
	}
	return resp
}

func TestLoginCreateJoinFlow(t *testing.T) {
	rig := newTestRig(t)
	defer rig.close()

	var loginResp LoginResponse
	postJSON(t, rig.client, rig.server.URL+"/auth/login", LoginRequest{Username: "gamer", Password: "password123"}, &loginResp)
	if loginResp.SessionToken == "" || loginResp.DeviceToken == "" {
		t.Fatalf("expected tokens to be issued")
	}

	var roomResp CreateRoomResponse
	postJSON(t, rig.client, rig.server.URL+"/rooms", CreateRoomRequest{Name: "coop", PreferredTransport: TransportUDP, MTU: 1350, SessionToken: loginResp.SessionToken}, &roomResp)
	if roomResp.PreferredTransport != TransportUDP {
		t.Fatalf("expected UDP preference, got %s", roomResp.PreferredTransport)
	}

	var joinResp JoinRoomResponse
	postJSON(t, rig.client, rig.server.URL+"/rooms/join", JoinRoomRequest{RoomID: roomResp.RoomID, DeviceID: "device-1", SessionToken: loginResp.SessionToken}, &joinResp)
	if joinResp.Transport != roomResp.PreferredTransport {
		t.Fatalf("transport mismatch: %s", joinResp.Transport)
	}
	if joinResp.KeepaliveIntervalSec <= 0 {
		t.Fatalf("keepalive interval missing")
	}
	if joinResp.VirtualIP == "" || joinResp.SessionKey == "" {
		t.Fatalf("missing virtual IP or session key")
	}
}

func TestTunnelNegotiationSelectsTransport(t *testing.T) {
	rig := newTestRig(t)
	defer rig.close()

	var loginResp LoginResponse
	postJSON(t, rig.client, rig.server.URL+"/auth/login", LoginRequest{Username: "gamer", Password: "password123"}, &loginResp)

	var roomResp CreateRoomResponse
	postJSON(t, rig.client, rig.server.URL+"/rooms", CreateRoomRequest{Name: "pvp", PreferredTransport: TransportUDP, SessionToken: loginResp.SessionToken}, &roomResp)

	// UDP path
	offer := TunnelOffer{RoomID: roomResp.RoomID, Transport: TransportUDP, CipherSuite: CipherSuiteAES256GCM, EphemeralKey: "client-ephemeral"}
	var answer TunnelAnswer
	postJSON(t, rig.client, rig.server.URL+"/tunnel/bootstrap", offer, &answer)
	if answer.Transport != TransportUDP || answer.CipherSuite != CipherSuiteAES256GCM {
		t.Fatalf("unexpected negotiation result: %+v", answer)
	}

	// TCP fallback path
	offer.Transport = TransportTCP
	offer.CipherSuite = ""
	postJSON(t, rig.client, rig.server.URL+"/tunnel/bootstrap", offer, &answer)
	if answer.Transport != TransportTCP {
		t.Fatalf("expected tcp fallback, got %s", answer.Transport)
	}
	if answer.CipherSuite != CipherSuiteAES256GCM {
		t.Fatalf("default cipher suite not applied: %s", answer.CipherSuite)
	}
}

func TestKeepaliveEcho(t *testing.T) {
	rig := newTestRig(t)
	defer rig.close()

	var ack KeepaliveAck
	resp := postJSON(t, rig.client, rig.server.URL+"/rooms/keepalive", Keepalive{Sequence: 7}, &ack)
	if resp.TLS == nil {
		t.Fatalf("expected TLS to be used")
	}
	if ack.Sequence != 7 {
		t.Fatalf("sequence mismatch: %d", ack.Sequence)
	}
	if time.Since(time.Unix(ack.ServerTimeUnix, 0)) > time.Minute {
		t.Fatalf("server time looks stale")
	}
}

func TestRegisterPersistsState(t *testing.T) {
	dataPath := filepath.Join(t.TempDir(), "state.json")
	rig := newTestRigWithPath(t, dataPath)
	defer rig.close()

	var regResp RegisterResponse
	postJSON(t, rig.client, rig.server.URL+"/auth/register", RegisterRequest{Username: "nova", Password: "warp123", DeviceID: "rig-device"}, &regResp)
	if regResp.SessionToken == "" || regResp.DeviceToken == "" {
		t.Fatalf("register response missing tokens")
	}

	var roomResp CreateRoomResponse
	postJSON(t, rig.client, rig.server.URL+"/rooms", CreateRoomRequest{Name: "alpha", SessionToken: regResp.SessionToken}, &roomResp)

	var joinResp JoinRoomResponse
	postJSON(t, rig.client, rig.server.URL+"/rooms/join", JoinRoomRequest{RoomID: roomResp.RoomID, DeviceID: "rig-device", SessionToken: regResp.SessionToken}, &joinResp)
	if joinResp.VirtualIP == "" {
		t.Fatalf("expected virtual ip")
	}

	rig.close()

	restarted := newTestRigWithPath(t, dataPath)
	defer restarted.close()
	var loginResp LoginResponse
	postJSON(t, restarted.client, restarted.server.URL+"/auth/login", LoginRequest{Username: "nova", Password: "warp123"}, &loginResp)
	if loginResp.SessionToken == "" {
		t.Fatalf("login should succeed after restart")
	}
	postJSON(t, restarted.client, restarted.server.URL+"/rooms/join", JoinRoomRequest{RoomID: roomResp.RoomID, DeviceID: "rig-device", SessionToken: loginResp.SessionToken}, &joinResp)
	if joinResp.OverlaySubnetReference == "" {
		t.Fatalf("room metadata should persist")
	}
}

func TestFirstRegisteredUserIsAdmin(t *testing.T) {
	dataPath := filepath.Join(t.TempDir(), "state.json")
	rig := newTestRigWithPath(t, dataPath)
	defer rig.close()

	var regResp RegisterResponse
	postJSON(t, rig.client, rig.server.URL+"/auth/register", RegisterRequest{Username: "leader", Password: "secret"}, &regResp)

	var roomResp CreateRoomResponse
	resp := postJSON(t, rig.client, rig.server.URL+"/rooms", CreateRoomRequest{Name: "admin-only", SessionToken: regResp.SessionToken}, &roomResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected admin to create room, got %d", resp.StatusCode)
	}
}

func TestNonAdminCannotCreateRoom(t *testing.T) {
	rig := newTestRig(t)
	defer rig.close()

	var regResp RegisterResponse
	postJSON(t, rig.client, rig.server.URL+"/auth/register", RegisterRequest{Username: "member", Password: "pw"}, &regResp)

	resp := postJSON(t, rig.client, rig.server.URL+"/rooms", CreateRoomRequest{Name: "restricted", SessionToken: regResp.SessionToken}, &CreateRoomResponse{})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected forbidden for non-admin, got %d", resp.StatusCode)
	}
}

func TestAdminCanGrantAndRevoke(t *testing.T) {
	rig := newTestRig(t)
	defer rig.close()

	var adminLogin LoginResponse
	postJSON(t, rig.client, rig.server.URL+"/auth/login", LoginRequest{Username: "gamer", Password: "password123"}, &adminLogin)

	var regResp RegisterResponse
	postJSON(t, rig.client, rig.server.URL+"/auth/register", RegisterRequest{Username: "candidate", Password: "pw"}, &regResp)

	// Grant admin rights
	var roleResp AdminRoleUpdateResponse
	postJSON(t, rig.client, rig.server.URL+"/admin/role", AdminRoleUpdateRequest{SessionToken: adminLogin.SessionToken, TargetUser: "candidate", Grant: true}, &roleResp)
	if !roleResp.IsAdmin {
		t.Fatalf("expected candidate to be admin after grant")
	}

	// Verify elevated permissions allow room creation
	resp := postJSON(t, rig.client, rig.server.URL+"/rooms", CreateRoomRequest{Name: "elevated", SessionToken: regResp.SessionToken}, &CreateRoomResponse{})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected granted admin to create room, got %d", resp.StatusCode)
	}

	// Revoke admin rights
	postJSON(t, rig.client, rig.server.URL+"/admin/role", AdminRoleUpdateRequest{SessionToken: adminLogin.SessionToken, TargetUser: "candidate", Grant: false}, &roleResp)
	if roleResp.IsAdmin {
		t.Fatalf("expected candidate to lose admin role")
	}

	resp = postJSON(t, rig.client, rig.server.URL+"/rooms", CreateRoomRequest{Name: "blocked", SessionToken: regResp.SessionToken}, &CreateRoomResponse{})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected revoked admin to be blocked, got %d", resp.StatusCode)
	}
}
