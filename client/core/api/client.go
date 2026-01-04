package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"

	"selfhostgameaccel/server/protocol"
)

// Client wraps HTTP operations against the control-plane API using the shared protocol types.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

// New creates an API client. Callers can supply a custom http.Client (for TLS customization);
// when nil, a default client with reasonable timeouts is used.
func New(rawURL string, httpClient *http.Client) (*Client, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse server url: %w", err)
	}
	if httpClient == nil {
		httpClient = DefaultHTTPClient(true)
	}
	return &Client{baseURL: parsed, httpClient: httpClient}, nil
}

// DefaultHTTPClient returns an HTTP client configured for TLS connections with optional
// certificate verification (useful for local self-signed development setups).
func DefaultHTTPClient(skipVerify bool) *http.Client {
	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: skipVerify}}
	return &http.Client{Transport: transport, Timeout: 15 * time.Second}
}

func (c *Client) doJSON(ctx context.Context, p string, reqBody any, respBody any) error {
	endpoint := *c.baseURL
	endpoint.Path = path.Join(c.baseURL.Path, p)

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	if respBody != nil {
		if err := json.NewDecoder(resp.Body).Decode(respBody); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

func (c *Client) Login(ctx context.Context, req protocol.LoginRequest) (protocol.LoginResponse, error) {
	var resp protocol.LoginResponse
	err := c.doJSON(ctx, "/auth/login", req, &resp)
	return resp, err
}

func (c *Client) Register(ctx context.Context, req protocol.RegisterRequest) (protocol.RegisterResponse, error) {
	var resp protocol.RegisterResponse
	err := c.doJSON(ctx, "/auth/register", req, &resp)
	return resp, err
}

func (c *Client) Refresh(ctx context.Context, req protocol.RefreshTokenRequest) (protocol.RefreshTokenResponse, error) {
	var resp protocol.RefreshTokenResponse
	err := c.doJSON(ctx, "/auth/refresh", req, &resp)
	return resp, err
}

func (c *Client) CreateRoom(ctx context.Context, req protocol.CreateRoomRequest) (protocol.CreateRoomResponse, error) {
	var resp protocol.CreateRoomResponse
	err := c.doJSON(ctx, "/rooms", req, &resp)
	return resp, err
}

func (c *Client) JoinRoom(ctx context.Context, req protocol.JoinRoomRequest) (protocol.JoinRoomResponse, error) {
	var resp protocol.JoinRoomResponse
	err := c.doJSON(ctx, "/rooms/join", req, &resp)
	return resp, err
}

func (c *Client) Keepalive(ctx context.Context, req protocol.Keepalive) (protocol.KeepaliveAck, error) {
	var resp protocol.KeepaliveAck
	err := c.doJSON(ctx, "/rooms/keepalive", req, &resp)
	return resp, err
}

func (c *Client) BootstrapTunnel(ctx context.Context, req protocol.TunnelOffer) (protocol.TunnelAnswer, error) {
	var resp protocol.TunnelAnswer
	err := c.doJSON(ctx, "/tunnel/bootstrap", req, &resp)
	return resp, err
}

func (c *Client) UpdateAdminRole(ctx context.Context, req protocol.AdminRoleUpdateRequest) (protocol.AdminRoleUpdateResponse, error) {
	var resp protocol.AdminRoleUpdateResponse
	err := c.doJSON(ctx, "/admin/role", req, &resp)
	return resp, err
}
