package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/microsoft/go-mssqldb"

	"adventureworks-workload/internal/app"
	"adventureworks-workload/internal/tui"
)

func main() {
	cfg, err := app.ParseConfig(os.Args[1:], os.Getenv)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(2)
	}

	logger := newLogger(cfg)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := sql.Open("sqlserver", cfg.ConnectionString())
	if err != nil {
		logger.Error("open database handle", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	cfg.ApplyPoolSettings(db)
	if err := db.PingContext(ctx); err != nil {
		logger.Error("ping database", "error", err)
		os.Exit(1)
	}

	var report app.Report
	if cfg.TUI {
		report, err = tui.RunWithWorkload(ctx, cfg, func(runCtx context.Context, onProgress app.ProgressFunc) (app.Report, error) {
			return app.Run(runCtx, db, cfg, logger, onProgress)
		})
	} else {
		report, err = app.Run(ctx, db, cfg, logger, nil)
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("run workload", "error", err)
		os.Exit(1)
	}

	if !cfg.TUI {
		if err := app.WriteReports(report, cfg); err != nil {
			logger.Error("write reports", "error", err)
			os.Exit(1)
		}

		fmt.Println()
		fmt.Println(report.MarkdownSummary())
	}
}

func newLogger(cfg app.Config) *slog.Logger {
	if cfg.TUI {
		return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
}
