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
	"campusclaw/internal/mcp"
	"campusclaw/internal/openapi"
	"campusclaw/internal/tutor"
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
	mcpConfig, err := mcp.LoadConfig()
	if err != nil {
		log.Fatalf("MCP configuration error: %v", err)
	}
	mcpHandler := mcp.New(database, mcpConfig)
	tutorHandler := tutor.New(database, knowledgeService)
	tutorHandler.MCP = mcpHandler
	mux := http.NewServeMux()
	openapi.Mount(mux)
	mux.HandleFunc("GET /health", health)
	mux.Handle("GET /api/mcp/servers", authenticator.Require(http.HandlerFunc(mcpHandler.ListServers)))
	mux.Handle("POST /api/mcp/servers", authenticator.Require(http.HandlerFunc(mcpHandler.WriteServer)))
	mux.Handle("GET /api/mcp/servers/{id}", authenticator.Require(http.HandlerFunc(mcpHandler.GetServer)))
	mux.Handle("PUT /api/mcp/servers/{id}", authenticator.Require(http.HandlerFunc(mcpHandler.WriteServer)))
	mux.Handle("DELETE /api/mcp/servers/{id}", authenticator.Require(http.HandlerFunc(mcpHandler.DeleteServer)))
	mux.Handle("POST /api/mcp/servers/{id}/connect", authenticator.Require(http.HandlerFunc(mcpHandler.Connect)))
	mux.Handle("POST /api/mcp/servers/{id}/disconnect", authenticator.Require(http.HandlerFunc(mcpHandler.Disconnect)))
	mux.Handle("GET /api/mcp/servers/{id}/tools", authenticator.Require(http.HandlerFunc(mcpHandler.Tools)))
	mux.Handle("GET /api/mcp/servers/{id}/changes", authenticator.Require(http.HandlerFunc(mcpHandler.Changes)))
	mux.Handle("GET /api/mcp/audits", authenticator.Require(http.HandlerFunc(mcpHandler.Audits)))
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
	mux.Handle("GET /api/agents", authenticator.Require(http.HandlerFunc(tutorHandler.ListAgents)))
	mux.Handle("POST /api/agents", authenticator.Require(http.HandlerFunc(tutorHandler.WriteAgent)))
	mux.Handle("GET /api/agents/{id}", authenticator.Require(http.HandlerFunc(tutorHandler.GetAgent)))
	mux.Handle("PUT /api/agents/{id}", authenticator.Require(http.HandlerFunc(tutorHandler.WriteAgent)))
	mux.Handle("GET /api/skills", authenticator.Require(http.HandlerFunc(tutorHandler.ListSkills)))
	mux.Handle("POST /api/skills", authenticator.Require(http.HandlerFunc(tutorHandler.WriteSkill)))
	mux.Handle("GET /api/skills/{id}", authenticator.Require(http.HandlerFunc(tutorHandler.GetSkill)))
	mux.Handle("PUT /api/skills/{id}", authenticator.Require(http.HandlerFunc(tutorHandler.WriteSkill)))
	mux.Handle("DELETE /api/skills/{id}", authenticator.Require(http.HandlerFunc(tutorHandler.DeleteSkill)))
	mux.Handle("GET /api/conversations", authenticator.Require(http.HandlerFunc(tutorHandler.ListConversations)))
	mux.Handle("POST /api/conversations", authenticator.Require(http.HandlerFunc(tutorHandler.CreateConversation)))
	mux.Handle("GET /api/conversations/{id}", authenticator.Require(http.HandlerFunc(tutorHandler.GetConversation)))
	mux.Handle("POST /api/conversations/{id}/messages", authenticator.Require(http.HandlerFunc(tutorHandler.SendMessage)))

	server := &http.Server{
		Addr:              cfg.APIAddr,
		Handler:           securityHeaders(recoverPanic(mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
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

// @openapi {"method":"get","path":"/health","id":"get_health","summary":"公开存活检查","input":"","output":"","success":200,"codes":[],"public":true,"produces":"text/plain"}
func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	_, _ = w.Write([]byte("ok\n"))
}
