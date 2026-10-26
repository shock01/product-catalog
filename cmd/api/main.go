package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shock01/product-catalog/internal/gen"
	"github.com/shock01/product-catalog/internal/handler"
	"github.com/shock01/product-catalog/internal/usecase"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// TODO: initialise real infrastructure (pgx pool, valkey client, bloom filter)
	svc := usecase.NewProductService(nil, nil, nil, 5*time.Minute)
	h := handler.New(svc, svc)

	r := chi.NewRouter()
	gen.HandlerFromMux(gen.NewStrictHandler(h, nil), r)

	srv := &http.Server{
		Addr:              addr(),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("api listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
	logger.Info("shutdown complete")
}

func addr() string {
	if v := os.Getenv("ADDR"); v != "" {
		return v
	}
	return ":8080"
}
