package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"selfhostgameaccel/client/core/api"
	"selfhostgameaccel/server/protocol"
)

func main() {
	serverAddr := flag.String("server", "https://localhost:8443", "control plane URL")
	skipVerify := flag.Bool("insecure", true, "skip TLS verification (development only)")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	httpClient := api.DefaultHTTPClient(*skipVerify)
	client, err := api.New(*serverAddr, httpClient)
	if err != nil {
		log.Fatalf("client init: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch strings.ToLower(args[0]) {
	case "register":
		username := envOr("USERNAME", "gamer")
		password := envOr("PASSWORD", "password123")
		deviceID := envOr("DEVICE_ID", "device-1")
		resp, err := client.Register(ctx, protocol.RegisterRequest{Username: username, Password: password, DeviceID: deviceID})
		exit(resp, err)
	case "login":
		username := envOr("USERNAME", "gamer")
		password := envOr("PASSWORD", "password123")
		resp, err := client.Login(ctx, protocol.LoginRequest{Username: username, Password: password})
		exit(resp, err)
	case "create-room":
		name := envOr("ROOM_NAME", "coop")
		resp, err := client.CreateRoom(ctx, protocol.CreateRoomRequest{Name: name, PreferredTransport: protocol.TransportUDP, MTU: 1350})
		exit(resp, err)
	case "join-room":
		if len(args) < 2 {
			log.Fatalf("join-room requires room id argument")
		}
		session := envOr("SESSION_TOKEN", "")
		if session == "" {
			log.Fatalf("SESSION_TOKEN env var must be set")
		}
		resp, err := client.JoinRoom(ctx, protocol.JoinRoomRequest{RoomID: args[1], DeviceID: envOr("DEVICE_ID", "device-1"), SessionToken: session})
		exit(resp, err)
	case "keepalive":
		resp, err := client.Keepalive(ctx, protocol.Keepalive{Sequence: 1})
		exit(resp, err)
	case "bootstrap":
		if len(args) < 2 {
			log.Fatalf("bootstrap requires room id argument")
		}
		offer := protocol.TunnelOffer{RoomID: args[1], Transport: protocol.TransportUDP, CipherSuite: protocol.CipherSuiteAES256GCM, EphemeralKey: "client-ephemeral"}
		resp, err := client.BootstrapTunnel(ctx, offer)
		exit(resp, err)
	default:
		usage()
	}
}

func usage() {
	fmt.Println("vpn-client usage:")
	fmt.Println("  vpn-client [flags] <command> [args]")
	fmt.Println("Commands:")
	fmt.Println("  register                # create a new user via USERNAME/PASSWORD/DEVICE_ID")
	fmt.Println("  login                   # authenticate using USERNAME/PASSWORD env vars")
	fmt.Println("  create-room             # create room named ROOM_NAME (env)")
	fmt.Println("  join-room <room-id>     # join with SESSION_TOKEN env and DEVICE_ID")
	fmt.Println("  keepalive               # send a keepalive ping")
	fmt.Println("  bootstrap <room-id>     # request tunnel parameters")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func exit(resp any, err error) {
	if err != nil {
		log.Fatalf("command failed: %v", err)
	}
	fmt.Printf("%+v\n", resp)
}
