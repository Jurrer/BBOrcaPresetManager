// Package main provides the BBOrcaProfsync server entry point.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/Jurrer/BBOrcaPresetManager/internal/api"
	"github.com/Jurrer/BBOrcaPresetManager/internal/config"
	"github.com/Jurrer/BBOrcaPresetManager/internal/paths"
	"github.com/Jurrer/BBOrcaPresetManager/internal/slicer"
	"github.com/Jurrer/BBOrcaPresetManager/internal/sync"
	"github.com/Jurrer/BBOrcaPresetManager/internal/vault"
	"github.com/Jurrer/BBOrcaPresetManager/web"
)

// defaultVaultPath returns the conventional vault path inside the user's
// home directory.
func defaultVaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".bborcaprofsync", "vault")
}

// resolveUserDir picks the effective user dir for a slicer, preferring
// the config override over auto-detection.
func resolveUserDir(cfgDir, detectedDir string) string {
	if cfgDir != "" {
		return cfgDir
	}
	return detectedDir
}

// resolveUID picks the effective user_id for a slicer, preferring the
// config override over auto-detection.
func resolveUID(cfgUID, detectedUID string) string {
	if cfgUID != "" {
		return cfgUID
	}
	return detectedUID
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("bborcaprofsync: %v", err)
	}
}

func run() error {
	cfgPath := config.DefaultPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	detectedBS, detectedOrca, err := paths.DetectPaths()
	if err != nil {
		return err
	}

	bsUserDir := resolveUserDir(cfg.Slicers.BambuStudio.UserDir, detectedBS.UserDir)
	bsUID := resolveUID(cfg.Slicers.BambuStudio.UserID, detectedBS.SelectedUID)
	bsAdapter := slicer.NewBambuStudioAdapter(bsUserDir, bsUID, detectedBS.DetectedVersion)

	orUserDir := resolveUserDir(cfg.Slicers.OrcaSlicer.UserDir, detectedOrca.UserDir)
	orUID := resolveUID(cfg.Slicers.OrcaSlicer.UserID, detectedOrca.SelectedUID)
	orcaAdapter := slicer.NewOrcaSlicerAdapter(orUserDir, orUID, detectedOrca.DetectedVersion)

	vaultPath := cfg.Vault.Path
	if vaultPath == "" {
		vaultPath = defaultVaultPath()
	}
	v, err := vault.Open(vaultPath)
	if err != nil {
		return err
	}

	engine := sync.NewEngine(bsAdapter, orcaAdapter, v)

	srv := api.NewServer(
		bsAdapter, orcaAdapter,
		detectedBS, detectedOrca,
		engine, v,
		cfg, cfgPath,
	)

	mux := http.NewServeMux()
	mux.Handle("/api/", srv.Handler())
	mux.Handle("/", web.Handler())

	port := cfg.Server.Port
	if port == 0 {
		port = 9876
	}
	addr := ":" + strconv.Itoa(port)

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("bborcaprofsync: listening on %s", addr)
	log.Printf("  BS user:   %s (uid %s)", bsUserDir, bsUID)
	log.Printf("  Orca user: %s (uid %s)", orUserDir, orUID)
	log.Printf("  Vault:     %s", vaultPath)

	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-stop:
		log.Printf("bborcaprofsync: received %s, shutting down", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpServer.Shutdown(ctx)
	}
}
