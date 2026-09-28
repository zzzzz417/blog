package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"blog/internal/config"
	editorapp "blog/internal/editor/app"
	"blog/internal/media/infrastructure/local"
	"blog/internal/platform/database"
	"blog/internal/platform/logging"
	"blog/internal/post/app"
	"blog/internal/post/infrastructure/markdown"
	postsqlite "blog/internal/post/infrastructure/sqlite"
	"blog/internal/server"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := logging.New(cfg.LogLevel, cfg.LogFormat)
	slog.SetDefault(logger)

	db, hasFTS, err := database.Open(cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer db.Close()

	repo := postsqlite.NewRepository(db, hasFTS)
	source := markdown.NewStore(cfg.ContentDir)
	service := app.NewService(repo).WithEditing(source, cfg.MaxTags)
	posts, err := source.Load()
	if err != nil {
		return err
	}
	if err := service.Sync(context.Background(), posts); err != nil {
		return err
	}
	logger.Info("content synchronized", "count", len(posts), "fts5", hasFTS)
	storage, err := local.New(cfg.UploadDir)
	if err != nil {
		return err
	}
	auth := editorapp.NewService(cfg.EditorKey, cfg.JWTSecret, cfg.TokenTTL)

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.NewRouter(service, auth, storage, cfg, logger),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown failed", "error", err)
		}
	}()

	logger.Info("server listening", "addr", cfg.Addr, "database", cfg.DatabasePath)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
