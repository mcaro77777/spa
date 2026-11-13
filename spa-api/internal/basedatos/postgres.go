package basedatos

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"spa-api/internal/configuracion"
)

func Abrir(ctx context.Context, cfg configuracion.Configuracion) (*sql.DB, error) {
	db, err := sql.Open("pgx", cfg.DSN)
	if err != nil { return nil, fmt.Errorf("abrir PostgreSQL: %w", err) }
	db.SetMaxOpenConns(cfg.MaxConexiones)
	db.SetMaxIdleConns(cfg.MaxConexiones / 2)
	db.SetConnMaxLifetime(30 * time.Minute)
	pingCtx, cancelar := context.WithTimeout(ctx, 8*time.Second)
	defer cancelar()
	if err := db.PingContext(pingCtx); err != nil { db.Close(); return nil, fmt.Errorf("conectar PostgreSQL: %w", err) }
	var existe bool
	if err := db.QueryRowContext(pingCtx, `SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema='spa' AND table_name='usuarios')`).Scan(&existe); err != nil || !existe {
		db.Close()
		if err != nil { return nil, fmt.Errorf("verificar esquema spa: %w", err) }
		return nil, errors.New("el esquema spa no esta instalado; ejecute scripts/crear_esquema_spa.sql")
	}
	return db, nil
}

