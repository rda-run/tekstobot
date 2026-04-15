package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"tekstobot/internal/ai"
	"tekstobot/internal/config"
	"tekstobot/internal/db"
	"tekstobot/internal/service"
	"tekstobot/internal/ui"
	"tekstobot/internal/whatsapp"
)

var Version = "dev"

func main() {
	cfg := config.Load()

	dbConn, err := db.InitDB(cfg)
	if err != nil {
		log.Fatalf("Database initialization failed: %v", err)
	}
	defer dbConn.Close()

	var migrationErrStr string
	if err := db.RunMigrations(dbConn); err != nil {
		log.Printf("CRITICAL: Database migrations failed: %v", err)
		migrationErrStr = err.Error()
	}

	if err := os.MkdirAll("data/media", os.ModePerm); err != nil {
		log.Fatalf("Failed to create media directory: %v", err)
	}

	repo := db.NewRepository(dbConn)

	// AI Transcriber backend
	var transcriber ai.Transcriber
	switch strings.ToLower(strings.TrimSpace(cfg.TranscriberBackend)) {
	case "", "local":
		transcriber = ai.NewWhisperClient(cfg)
	case "cloudflare":
		if strings.TrimSpace(cfg.CloudflareAccountID) == "" {
			log.Fatal("CLOUDFLARE_ACCOUNT_ID is required when TRANSCRIBER_BACKEND=cloudflare")
		}
		if strings.TrimSpace(cfg.CloudflareAPIToken) == "" {
			log.Fatal("CLOUDFLARE_API_TOKEN is required when TRANSCRIBER_BACKEND=cloudflare")
		}
		transcriber = ai.NewCloudflareClient(
			cfg.CloudflareAccountID,
			cfg.CloudflareAPIToken,
			cfg.CloudflareLanguage,
		)
	default:
		log.Fatalf(
			"invalid TRANSCRIBER_BACKEND value %q (supported: local, cloudflare)",
			fmt.Sprintf("%s", cfg.TranscriberBackend),
		)
	}

	// WhatsApp
	dsn := db.GetDSN(cfg)
	waClient, err := whatsapp.NewClient(repo, dsn, cfg.AdminPhone)
	if err != nil {
		log.Fatalf("Failed to init WhatsApp client: %v", err)
	}

	worker := service.NewWorker(repo, transcriber, waClient.WAClient)

	// UI Server
	uiServer := ui.NewServer(repo, waClient, Version, migrationErrStr)

	// Start modules only if NO migration error
	if migrationErrStr == "" {
		go worker.Start(waClient.MediaChan)

		if err := waClient.Start(); err != nil {
			log.Fatalf("Failed to start WhatsApp Client: %v", err)
		}
		defer waClient.Stop()
	} else {
		log.Println("MAINTENANCE MODE: WhatsApp and Worker modules are disabled due to database error.")
	}

	go func() {
		if err := uiServer.Start(cfg.Port); err != nil {
			log.Fatalf("UI server failed: %v", err)
		}
	}()

	log.Println("TekstoBot successfully initialized. Waiting for events...")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Println("Shutting down...")
}
