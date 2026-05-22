package main

import (
    "context"
    "flag"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "go.uber.org/zap"
    "golang.org/x/time/rate"

    "github.com/VKaralyov/discord-bot-api/pkg/api"
)

func main() {
    port := flag.String("port", "8080", "server port")
    queueSize := flag.Int("queue", 100, "request queue size")
    workers := flag.Int("workers", 4, "number of worker goroutines")
    ratePerSec := flag.Float64("rate", 5.0, "requests per second")
    flag.Parse()

    // Init structured logger
    cfg := zap.NewProductionConfig()
    if os.Getenv("LOG_LEVEL") == "debug" {
        cfg = zap.NewDevelopmentConfig()
    }
    logger, _ := cfg.Build()
    defer logger.Sync()

    mux := http.NewServeMux()

    limiter := rate.NewLimiter(rate.Limit(*ratePerSec), int(*ratePerSec*2)+1)
    api.Init(*queueSize, *workers, limiter, logger)
    api.Register(mux, logger)

    srv := &http.Server{
        Addr:         ":" + *port,
        Handler:      mux,
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 30 * time.Second,
        IdleTimeout:  120 * time.Second,
    }

    // Start server
    go func() {
        logger.Sugar().Infof("starting server on %s", srv.Addr)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            logger.Fatal("server error", zap.Error(err))
        }
    }()

    // Graceful shutdown
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
    <-stop

    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()
    logger.Info("shutting down server")
    if err := srv.Shutdown(ctx); err != nil {
        logger.Fatal("shutdown failed", zap.Error(err))
    }
    logger.Info("server stopped")
}

