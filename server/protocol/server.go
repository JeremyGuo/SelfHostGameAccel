package protocol

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Transport string

type CipherSuite string

const (
	TransportUDP Transport = "udp"
	TransportTCP Transport = "tcp"

	CipherSuiteAES256GCM        CipherSuite = "aes-256-gcm"
	CipherSuiteChaCha20Poly1305 CipherSuite = "chacha20-poly1305"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	DeviceID string `json:"device_id"`
}

type LoginResponse struct {
	SessionToken string `json:"session_token"`
	DeviceToken  string `json:"device_token"`
}

type RegisterResponse struct {
	SessionToken string `json:"session_token"`
	DeviceToken  string `json:"device_token"`
}

type RefreshTokenRequest struct {
	DeviceToken string `json:"device_token"`
}

type RefreshTokenResponse struct {
	SessionToken string `json:"session_token"`
}

type CreateRoomRequest struct {
	Name               string    `json:"name"`
	PreferredTransport Transport `json:"preferred_transport"`
	MTU                int       `json:"mtu"`
}

type CreateRoomResponse struct {
	RoomID             string    `json:"room_id"`
	OverlaySubnet      string    `json:"overlay_subnet"`
	PreferredTransport Transport `json:"preferred_transport"`
	MTU                int       `json:"mtu"`
}

type JoinRoomRequest struct {
	RoomID       string `json:"room_id"`
	DeviceID     string `json:"device_id"`
	SessionToken string `json:"session_token"`
}

type JoinRoomResponse struct {
	VirtualIP              string    `json:"virtual_ip"`
	SessionKey             string    `json:"session_key"`
	Transport              Transport `json:"transport"`
	KeepaliveIntervalSec   int       `json:"keepalive_interval_seconds"`
	OverlaySubnetReference string    `json:"overlay_subnet,omitempty"`
}

type Keepalive struct {
	Sequence uint64 `json:"sequence"`
}

type KeepaliveAck struct {
	Sequence         uint64 `json:"sequence"`
	ServerTimeUnix   int64  `json:"server_time_unix_sec"`
	RecommendedDelay int64  `json:"recommended_delay_ms"`
}

type TunnelOffer struct {
	RoomID       string      `json:"room_id"`
	Transport    Transport   `json:"transport"`
	CipherSuite  CipherSuite `json:"cipher_suite"`
	EphemeralKey string      `json:"ephemeral_pub_key"`
}

type TunnelAnswer struct {
	Transport    Transport   `json:"transport"`
	CipherSuite  CipherSuite `json:"cipher_suite"`
	EphemeralKey string      `json:"ephemeral_pub_key"`
}

type Server struct {
	mux         *http.ServeMux
	mu          sync.Mutex
	users       map[string]userRecord
	sessions    map[string]string
	deviceBags  map[string]string
	rooms       map[string]*roomRecord
	persistPath string
}

type userRecord struct {
	Username string
	Salt     string
	Hash     string
	Device   string
}

type roomRecord struct {
	ID                 string
	Name               string
	PreferredTransport Transport
	MTU                int
	OverlaySubnet      string
	KeepaliveInterval  int
	Members            map[string]string
}

func NewServer() *Server {
	s, _ := NewServerWithStorage("")
	return s
}

// NewServerWithStorage loads state from disk when persistPath is non-empty and persists changes.
func NewServerWithStorage(persistPath string) (*Server, error) {
	s := &Server{
		mux:         http.NewServeMux(),
		users:       map[string]userRecord{},
		sessions:    map[string]string{},
		deviceBags:  map[string]string{},
		rooms:       map[string]*roomRecord{},
		persistPath: persistPath,
	}
	s.registerRoutes()

	if persistPath != "" {
		if err := s.loadFromDisk(); err != nil {
			return nil, err
		}
	} else {
		s.seedDemoUser()
	}

	return s, nil
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/auth/register", s.handleRegister)
	s.mux.HandleFunc("/auth/login", s.handleLogin)
	s.mux.HandleFunc("/auth/refresh", s.handleRefresh)
	s.mux.HandleFunc("/rooms", s.handleCreateRoom)
	s.mux.HandleFunc("/rooms/join", s.handleJoinRoom)
	s.mux.HandleFunc("/rooms/keepalive", s.handleKeepalive)
	s.mux.HandleFunc("/tunnel/bootstrap", s.handleTunnelBootstrap)
}

func (s *Server) seedDemoUser() {
	salt := randomSalt()
	hash := hashPassword("password123", salt)
	s.users["gamer"] = userRecord{Username: "gamer", Salt: salt, Hash: hash, Device: "demo-device"}
	s.deviceBags["demo-device"] = "gamer"
}

func (s *Server) loadFromDisk() error {
	state, err := loadState(s.persistPath)
	if err != nil {
		return err
	}
	if len(state.Users) == 0 {
		s.seedDemoUser()
		return nil
	}
	s.users = state.Users
	if state.DeviceBags != nil {
		s.deviceBags = state.DeviceBags
	}
	if state.Rooms != nil {
		s.rooms = state.Rooms
	}
	return nil
}

func (s *Server) persistLocked() error {
	if s.persistPath == "" {
		return nil
	}
	state := persistentState{Users: s.users, DeviceBags: s.deviceBags, Rooms: s.rooms}
	if err := os.MkdirAll(filepath.Dir(s.persistPath), 0o755); err != nil {
		return err
	}
	return saveState(s.persistPath, state)
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req RegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
		writeError(w, http.StatusBadRequest, errors.New("username and password required"))
		return
	}
	if req.DeviceID == "" {
		req.DeviceID = fmt.Sprintf("device-%s", newToken()[:6])
	}
	if strings.Contains(req.Username, " ") {
		writeError(w, http.StatusBadRequest, errors.New("username may not contain spaces"))
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.users[req.Username]; exists {
		writeError(w, http.StatusConflict, errors.New("user already exists"))
		return
	}
	salt := randomSalt()
	record := userRecord{Username: req.Username, Salt: salt, Hash: hashPassword(req.Password, salt), Device: req.DeviceID}
	s.users[req.Username] = record
	s.deviceBags[req.DeviceID] = req.Username
	sessionToken := newToken()
	deviceToken := newToken()
	s.sessions[sessionToken] = req.Username
	s.deviceBags[deviceToken] = req.Username
	if err := s.persistLocked(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("persist: %w", err))
		return
	}
	writeJSON(w, RegisterResponse{SessionToken: sessionToken, DeviceToken: deviceToken})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req LoginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.users[req.Username]
	if !ok || record.Hash != hashPassword(req.Password, record.Salt) {
		writeError(w, http.StatusUnauthorized, errors.New("invalid credentials"))
		return
	}
	sessionToken := newToken()
	deviceToken := newToken()
	s.sessions[sessionToken] = req.Username
	s.deviceBags[deviceToken] = req.Username
	if err := s.persistLocked(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("persist: %w", err))
		return
	}
	writeJSON(w, LoginResponse{SessionToken: sessionToken, DeviceToken: deviceToken})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req RefreshTokenRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	username, ok := s.deviceBags[req.DeviceToken]
	if !ok {
		writeError(w, http.StatusUnauthorized, errors.New("device token invalid"))
		return
	}
	sessionToken := newToken()
	s.sessions[sessionToken] = username
	writeJSON(w, RefreshTokenResponse{SessionToken: sessionToken})
}

func (s *Server) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req CreateRoomRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	roomID := fmt.Sprintf("room-%d", len(s.rooms)+1)
	subnet := fmt.Sprintf("10.0.%d.0/24", len(s.rooms)+1)
	if req.MTU == 0 {
		req.MTU = 1400
	}
	if req.PreferredTransport == "" {
		req.PreferredTransport = TransportUDP
	}
	rec := &roomRecord{
		ID:                 roomID,
		Name:               req.Name,
		PreferredTransport: req.PreferredTransport,
		MTU:                req.MTU,
		OverlaySubnet:      subnet,
		KeepaliveInterval:  15,
		Members:            map[string]string{},
	}
	s.rooms[roomID] = rec
	if err := s.persistLocked(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("persist: %w", err))
		return
	}
	writeJSON(w, CreateRoomResponse{
		RoomID:             roomID,
		OverlaySubnet:      subnet,
		PreferredTransport: rec.PreferredTransport,
		MTU:                rec.MTU,
	})
}

func (s *Server) handleJoinRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req JoinRoomRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	username, ok := s.sessions[req.SessionToken]
	if !ok {
		writeError(w, http.StatusUnauthorized, errors.New("session invalid"))
		return
	}
	room, ok := s.rooms[req.RoomID]
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("room not found"))
		return
	}
	virtualIP := fmt.Sprintf("10.0.%d.%d", len(room.Members)+1, len(room.Members)+2)
	sessionKey := newToken()
	room.Members[req.DeviceID] = username
	if err := s.persistLocked(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("persist: %w", err))
		return
	}
	writeJSON(w, JoinRoomResponse{
		VirtualIP:              virtualIP,
		SessionKey:             sessionKey,
		Transport:              room.PreferredTransport,
		KeepaliveIntervalSec:   room.KeepaliveInterval,
		OverlaySubnetReference: room.OverlaySubnet,
	})
}

func (s *Server) handleKeepalive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req Keepalive
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ack := KeepaliveAck{
		Sequence:         req.Sequence,
		ServerTimeUnix:   time.Now().Unix(),
		RecommendedDelay: 5000,
	}
	writeJSON(w, ack)
}

func (s *Server) handleTunnelBootstrap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req TunnelOffer
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	room, ok := s.rooms[req.RoomID]
	s.mu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("room not found"))
		return
	}
	transport := req.Transport
	if transport == "" {
		transport = room.PreferredTransport
	}
	cipher := req.CipherSuite
	if cipher == "" {
		cipher = CipherSuiteAES256GCM
	}
	answer := TunnelAnswer{
		Transport:    transport,
		CipherSuite:  cipher,
		EphemeralKey: req.EphemeralKey,
	}
	writeJSON(w, answer)
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeResponse(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, body any) {
	writeResponse(w, http.StatusOK, body)
}

func writeResponse(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func newToken() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func randomSalt() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return base64.StdEncoding.EncodeToString(buf)
}

func hashPassword(password, salt string) string {
	sum := sha256.Sum256([]byte(salt + ":" + password))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func NormalizeTransport(t Transport) Transport {
	switch strings.ToLower(string(t)) {
	case string(TransportUDP):
		return TransportUDP
	case string(TransportTCP):
		return TransportTCP
	default:
		return Transport("")
	}
}

func ValidateCipherSuite(c CipherSuite) CipherSuite {
	switch strings.ToLower(string(c)) {
	case string(CipherSuiteAES256GCM):
		return CipherSuiteAES256GCM
	case string(CipherSuiteChaCha20Poly1305):
		return CipherSuiteChaCha20Poly1305
	default:
		return CipherSuite("")
	}
}

var _ http.Handler = (*Server)(nil)
