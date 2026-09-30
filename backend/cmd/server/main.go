package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"campusclaw/internal/auth"
	"campusclaw/internal/config"
	appdb "campusclaw/internal/db"
	"campusclaw/internal/knowledge"
	"campusclaw/internal/materials"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	database, err := appdb.OpenWithRetry(rootCtx, cfg)
	if err != nil {
		log.Fatalf("database connection error: %v", err)
	}
	defer database.Close()
	if err := appdb.Migrate(rootCtx, database); err != nil {
		log.Fatalf("database migration error: %v", err)
	}
	if err := appdb.Seed(rootCtx, database, cfg); err != nil {
		log.Fatalf("database seed error: %v", err)
	}
	knowledgeService := knowledge.New(database, cfg)
	if err := knowledgeService.Backfill(rootCtx); err != nil {
		log.Fatalf("knowledge backfill error: %v", err)
	}
	if len(os.Args) > 1 {
		if len(os.Args) == 2 && os.Args[1] == "reindex-all" {
			if err := knowledgeService.RebuildAll(rootCtx); err != nil {
				log.Fatalf("reindex failed: %v", err)
			}
			log.Print("all current-version knowledge entries queued for reindex")
			return
		}
		log.Fatalf("unknown command: %s", os.Args[1])
	}
	go knowledgeService.Run(rootCtx)

	authenticator, err := auth.New(database, cfg.SessionSecret, cfg.SessionTTL, cfg.LoginMaxFailures, cfg.LoginLock)
	if err != nil {
		log.Fatalf("authentication setup error: %v", err)
	}
	materialHandler := materials.New(database, cfg.UploadDir, cfg.MaxUploadBytes, cfg.IndexVersion)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("POST /api/login", authenticator.Login)
	mux.Handle("POST /api/logout", authenticator.Require(http.HandlerFunc(authenticator.Logout)))
	mux.Handle("GET /api/me", authenticator.Require(http.HandlerFunc(authenticator.Me)))
	mux.Handle("GET /api/materials", authenticator.Require(http.HandlerFunc(materialHandler.List)))
	mux.Handle("POST /api/materials", authenticator.Require(http.HandlerFunc(materialHandler.Upload)))
	mux.Handle("GET /api/materials/{id}", authenticator.Require(http.HandlerFunc(materialHandler.Detail)))
	mux.Handle("GET /api/materials/{id}/file", authenticator.Require(http.HandlerFunc(materialHandler.Download)))
	mux.Handle("DELETE /api/materials/{id}", authenticator.Require(http.HandlerFunc(knowledgeService.DeleteMaterialHTTP)))
	mux.Handle("POST /api/materials/{id}/reindex", authenticator.Require(http.HandlerFunc(knowledgeService.ReindexHTTP)))
	mux.Handle("GET /api/knowledge/search", authenticator.Require(http.HandlerFunc(knowledgeService.SearchHTTP)))
	mux.Handle("POST /api/ask", authenticator.Require(http.HandlerFunc(knowledgeService.AskHTTP)))

	server := &http.Server{
		Addr:              cfg.APIAddr,
		Handler:           securityHeaders(recoverPanic(mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		log.Printf("CampusClaw API listening on %s", cfg.APIAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	<-rootCtx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown error: %v", err)
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("panic recovered: %v", recovered)
				http.Error(w, `{"error":"服务暂时不可用"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)
}
