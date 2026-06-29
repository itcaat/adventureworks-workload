package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/microsoft/go-mssqldb"

	"adventureworks-workload/internal/app"
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

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
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

	report, err := app.Run(ctx, db, cfg, logger)
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("run workload", "error", err)
		os.Exit(1)
	}

	if err := app.WriteReports(report, cfg); err != nil {
		logger.Error("write reports", "error", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println(report.MarkdownSummary())
}
