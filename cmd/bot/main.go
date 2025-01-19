package main

import (
	"log"
	"time"

	"github.com/dntjd1097/allora-checker-bot/internal/config"
	"github.com/dntjd1097/allora-checker-bot/internal/service"
)

func main() {
	// Set log format to include timestamp and file info
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Starting Allora Checker Bot...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}
	log.Println("Configuration loaded successfully")

	// Initialize Telegram bot with retry
	log.Println("Initializing Telegram bot...")
	bot, err := service.InitBot(cfg.Telegram.Token, 3)
	if err != nil {
		log.Fatalf("Error initializing bot: %v", err)
	}
	log.Println("Telegram bot initialized successfully")

	// Initialize services
	log.Println("Initializing services...")
	alloraService := service.NewAlloraService(cfg.Allora.API)
	historyService := service.NewHistoryService("history")
	log.Println("Services initialized successfully")

	// Create update config with bot and config
	updateConfig := service.NewUpdateConfig(bot, cfg)

	// Create ticker for periodic checks
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// Create telegram service
	telegramService := service.NewTelegramService(bot, cfg, alloraService, historyService)
	log.Println("Telegram service created successfully")

	// Start handling updates
	log.Println("Starting to handle updates...")
	go service.HandleUpdates(updateConfig)

	log.Println("Bot is now running. Press Ctrl+C to stop.")
	// Handle periodic rank checks
	for range ticker.C {
		log.Println("Running periodic rank check...")
		telegramService.CheckRankChanges()
	}
}
