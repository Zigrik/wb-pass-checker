package main

import (
	"log"
	"os"

	"wb-passes/pkg/wbpasses"

	"github.com/joho/godotenv"
)

func main() {
	// Загружаем .env
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env file not found, using system environment variables")
	}

	config := wbpasses.Config{
		APIToken: os.Getenv("WB_API_TOKEN"),
		APIURL:   os.Getenv("WB_API_URL"),
		Port:     os.Getenv("PORT"),
	}

	if config.APIToken == "" {
		log.Fatal("❌ ERROR: WB_API_TOKEN is required")
	}
	if config.APIURL == "" {
		config.APIURL = "https://marketplace-api.wildberries.ru"
		log.Println("⚠️ Using default API URL")
	}
	if config.Port == "" {
		config.Port = "8080"
		log.Println("⚠️ Using default port 8080")
	}

	server := wbpasses.NewServer(config)
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}
