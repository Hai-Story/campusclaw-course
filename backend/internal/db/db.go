package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"time"

	"campusclaw/internal/config"
	"github.com/go-sql-driver/mysql"
)

func OpenWithRetry(ctx context.Context, cfg config.Config) (*sql.DB, error) {
	mysqlConfig := mysql.Config{
		User:                 cfg.DBUser,
		Passwd:               cfg.DBPassword,
		Net:                  "tcp",
		Addr:                 net.JoinHostPort(cfg.DBHost, cfg.DBPort),
		DBName:               cfg.DBName,
		ParseTime:            true,
		Loc:                  time.UTC,
		AllowNativePasswords: true,
	}
	dsn := mysqlConfig.FormatDSN()
	database, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(20)
	database.SetMaxIdleConns(5)
	database.SetConnMaxLifetime(5 * time.Minute)

	deadline := time.Now().Add(90 * time.Second)
	for {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err = database.PingContext(pingCtx)
		cancel()
		if err == nil {
			return database, nil
		}
		if time.Now().After(deadline) {
			_ = database.Close()
			return nil, fmt.Errorf("database not ready before timeout: %w", err)
		}
		log.Printf("database not ready; retrying: %v", err)
		select {
		case <-ctx.Done():
			_ = database.Close()
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
