package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	configPath := flag.String("config", "./config.yaml", "path to YAML config")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	if token == "" || chatID == "" {
		slog.Error("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID must be set")
		os.Exit(1)
	}

	state, err := LoadState(cfg.StateFile)
	if err != nil {
		slog.Error("load state", "err", err)
		os.Exit(1)
	}

	tg := NewTelegramClient(token, chatID)
	proc := NewFeedProcessor(tg, state, cfg.MaxSeenPerFeed)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("starting", "feeds", len(cfg.Feeds), "interval", cfg.Interval, "state_file", cfg.StateFile)

	runCycle(ctx, proc, cfg, state)

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("shutting down")
			if err := state.Save(); err != nil {
				slog.Error("final state save", "err", err)
			}
			return
		case <-ticker.C:
			runCycle(ctx, proc, cfg, state)
		}
	}
}

func runCycle(ctx context.Context, proc *FeedProcessor, cfg *Config, state *State) {
	start := time.Now()
	for _, feed := range cfg.Feeds {
		if err := ctx.Err(); err != nil {
			return
		}
		if err := proc.Process(ctx, feed); err != nil {
			slog.Error("feed error", "feed", feed.Title, "err", err)
		}
	}
	if err := state.Save(); err != nil {
		slog.Error("state save", "err", err)
	}
	slog.Info("cycle complete", "elapsed", time.Since(start).Round(time.Millisecond))
}
