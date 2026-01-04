package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"selfhostgameaccel/server/protocol"
)

func main() {
	addr := flag.String("addr", ":8443", "listen address for the control plane")
	dataPath := flag.String("data", "", "path to persist server state (JSON)")
	flag.Parse()

	serverTLS, _, err := protocol.GenerateTLSConfigs()
	if err != nil {
		log.Fatalf("failed to generate TLS config: %v", err)
	}

	server, err := protocol.NewServerWithStorage(resolveDataPath(*dataPath))
	if err != nil {
		log.Fatalf("init server: %v", err)
	}

	srv := &http.Server{
		Addr:      *addr,
		Handler:   server,
		TLSConfig: serverTLS,
	}

	go func() {
		log.Printf("vpn-server listening on https://%s", srv.Addr)
		if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	log.Println("shutdown signal received, stopping server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("failed to shutdown: %v", err)
	}
	log.Println("server stopped")
}

func resolveDataPath(raw string) string {
	if raw == "" {
		return ""
	}
	if filepath.IsAbs(raw) {
		return raw
	}
	wd, err := os.Getwd()
	if err != nil {
		return raw
	}
	return filepath.Join(wd, raw)
}
