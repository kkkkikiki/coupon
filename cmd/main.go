package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/kkkkikiki/coupon/gen/coupon/v1/couponv1connect"
	"github.com/kkkkikiki/coupon/internal/config"
	"github.com/kkkkikiki/coupon/internal/database"
	"github.com/kkkkikiki/coupon/internal/service"
)

func main() {
	ctx := context.Background()

	// Load configuration from environment variables
	cfg, err := config.Load(ctx)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Starting coupon service in %s mode", cfg.App.Environment)

	// Initialize database connections
	db, err := database.NewDB(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database connections: %v", err)
		}
	}()

	// Create coupon service with direct DB access
	couponService := service.NewCouponServer(db.Postgres)

	// Create HTTP mux
	mux := http.NewServeMux()

	// Register coupon service handler
	path, handler := couponv1connect.NewCouponServiceHandler(couponService)
	mux.Handle(path, handler)

	// Add health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		hostname, _ := os.Hostname()
		w.WriteHeader(http.StatusOK)
		response := fmt.Sprintf(`{"status":"ok","service":"coupon-system","hostname":"%s"}`, hostname)
		w.Write([]byte(response))
	})

	// Add database health check endpoint
	mux.HandleFunc("/health/db", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Postgres.PingContext(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"error","message":"postgres unavailable"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","postgres":"connected"}`))
	})

	// Add Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Create server with configuration optimized for high concurrency
	server := &http.Server{
		Addr:           cfg.Server.GetServerAddr(),
		ReadTimeout:    time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:    120 * time.Second, // Keep connections alive longer
		MaxHeaderBytes: 1 << 20,           // 1MB
		// Use h2c so we can serve HTTP/2 without TLS
		Handler: h2c.NewHandler(mux, &http2.Server{
			MaxConcurrentStreams: 1000, // Allow more concurrent streams
		}),
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting coupon service on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}
