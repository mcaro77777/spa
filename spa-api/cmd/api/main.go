package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"spa-api/internal/basedatos"
	"spa-api/internal/configuracion"
	"spa-api/internal/httpapi"
	"spa-api/internal/version"
)

func main() { if err := ejecutar(); err != nil { fmt.Fprintln(os.Stderr, "spa-api:", err); os.Exit(1) } }

func ejecutar() error {
	comando := "serve"
	if len(os.Args) > 1 { comando = os.Args[1] }
	if comando == "version" { fmt.Printf("spa-api %s (%s)\n", version.Version, version.Commit); return nil }
	if comando != "serve" && comando != "check-db" { return fmt.Errorf("comando desconocido %q", comando) }
	cfg, err := configuracion.Cargar()
	if err != nil { return err }
	logger := nuevoLogger(cfg.Produccion())
	db, err := basedatos.Abrir(context.Background(), cfg)
	if err != nil { return err }
	defer db.Close()
	if comando == "check-db" { fmt.Println("PostgreSQL accesible y esquema spa instalado."); return nil }
	if err := os.MkdirAll(cfg.DirectorioUploads, 0700); err != nil { return fmt.Errorf("crear directorio de archivos: %w", err) }
	srv := &http.Server{Addr: ":" + cfg.Puerto, Handler: httpapi.NuevoServidor(cfg, db, logger), ReadHeaderTimeout: 10*time.Second, ReadTimeout: 30*time.Second, WriteTimeout: 90*time.Second, IdleTimeout: 120*time.Second, MaxHeaderBytes: 1 << 20}
	senales, detener := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer detener()
	errores := make(chan error, 1)
	go func() { logger.Info("servidor iniciado", "puerto", cfg.Puerto, "version", version.Version); errores <- srv.ListenAndServe() }()
	select {
	case err := <-errores:
		if !errors.Is(err, http.ErrServerClosed) { return err }
	case <-senales.Done():
		ctx, cancelar := context.WithTimeout(context.Background(), 15*time.Second); defer cancelar(); return srv.Shutdown(ctx)
	}
	return nil
}

func nuevoLogger(prod bool) *slog.Logger {
	opciones := &slog.HandlerOptions{Level: slog.LevelInfo}
	if prod { return slog.New(slog.NewJSONHandler(os.Stdout, opciones)) }
	return slog.New(slog.NewTextHandler(os.Stdout, opciones))
}
