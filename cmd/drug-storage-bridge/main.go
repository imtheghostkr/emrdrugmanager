package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/imtheghostkr/emrdrugmanager/internal/app"
	"github.com/imtheghostkr/emrdrugmanager/internal/config"
)

//go:embed web/*
var webFS embed.FS

var version = "0.1.0"

func main() {
	configPath := flag.String("config", "", "config file path")
	noBrowser := flag.Bool("no-browser", false, "do not open browser on startup")
	hashToken := flag.String("hash-token", "", "print SHA-256 hash for an access token and exit")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := context.Background()

	if *hashToken != "" {
		sum := sha256.Sum256([]byte(*hashToken))
		fmt.Println(hex.EncodeToString(sum[:]))
		return
	}

	paths, err := config.ResolvePaths(*configPath)
	if err != nil {
		logger.Error("resolve paths failed", "error", err)
		os.Exit(1)
	}
	cfg, err := config.LoadOrDefault(paths.ConfigPath)
	if err != nil {
		logger.Error("load config failed", "error", err)
		os.Exit(1)
	}

	staticFS, err := fs.Sub(webFS, "web")
	if err != nil {
		logger.Error("load embedded web failed", "error", err)
		os.Exit(1)
	}

	application, err := app.New(app.Options{
		Version:  version,
		Config:   cfg,
		Paths:    paths,
		StaticFS: staticFS,
		Logger:   logger,
	})
	if err != nil {
		logger.Error("initialize app failed", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:           application.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/ui", cfg.Server.Port)
	logger.Info("starting Drug Storage Bridge", "version", version, "addr", server.Addr, "config", paths.ConfigPath)
	if !*noBrowser {
		go func() {
			time.Sleep(700 * time.Millisecond)
			if err := openBrowser(ctx, url); err != nil {
				logger.Warn("open browser failed", "error", err, "url", url)
			}
		}()
	}

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func openBrowser(ctx context.Context, url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.CommandContext(ctx, "open", url).Start()
	default:
		return exec.CommandContext(ctx, "xdg-open", url).Start()
	}
}
