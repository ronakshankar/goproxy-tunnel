// Package cmd wires together CLI flag parsing, dependency construction,
// and graceful shutdown. It is the single composition root of the binary.
package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/wiretunnel/wiretunnel/internal/config"
	"github.com/wiretunnel/wiretunnel/internal/proxy"
	"github.com/wiretunnel/wiretunnel/pkg/logger"
)

// Execute parses flags, validates configuration, and starts the proxy server.
// It blocks until a termination signal is received or a fatal error occurs.
func Execute() {
	cfg := parseFlags()

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "invalid configuration: %v\n", err)
		os.Exit(1)
	}

	log := logger.New()
	srv := proxy.NewServer(cfg, log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("starting WireTunnel",
		"listen", fmt.Sprintf(":%d", cfg.ListenPort),
		"target", cfg.TargetAddr,
		"workers", cfg.WorkerCount,
	)

	if err := srv.Start(ctx); err != nil {
		log.Error("server exited with error", "err", err)
		os.Exit(1)
	}
}

func parseFlags() *config.Config {
	cfg := &config.Config{}

	flag.IntVar(&cfg.ListenPort, "port", 8080, "Port to listen on for incoming WebSocket connections")
	flag.StringVar(&cfg.TargetAddr, "target", "127.0.0.1:9000", "TCP address of the backend target (host:port)")
	flag.IntVar(&cfg.WorkerCount, "workers", 4, "Number of background compression workers")
	flag.StringVar(&cfg.LogDir, "logdir", "./logs", "Directory for compressed traffic logs")
	flag.IntVar(&cfg.BufferSize, "buffer", 32*1024, "Per-connection read buffer size in bytes")
	flag.IntVar(&cfg.PipelineQueueDepth, "queue-depth", 512, "Buffered channel depth for the observability pipeline")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: wiretunnel [options]\n\nOptions:\n")
		flag.PrintDefaults()
	}

	flag.Parse()
	return cfg
}
