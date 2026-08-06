package db

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Open returns nil when no DATABASE_URL is configured. This keeps local UI/API
// development useful while ensuring production health reports the missing store.
func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, nil
	}

	database, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(50)
	database.SetMaxIdleConns(10)
	database.SetConnMaxIdleTime(5 * time.Minute)

	if err := database.PingContext(ctx); err != nil {
		database.Close()
		return nil, err
	}
	return database, nil
}
