package main

import (
	"context"
	"log"

	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
	"tekstobot/internal/config"
	"tekstobot/internal/db"
)

func main() {
	cfg := config.Load()
	dsn := db.GetDSN(cfg)

	dbLog := waLog.Stdout("Database", "WARN", true)
	container, err := sqlstore.New(context.Background(), "postgres", dsn, dbLog)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		log.Fatalf("failed to get device store: %v", err)
	}

	if deviceStore != nil && deviceStore.ID != nil {
		err = deviceStore.Delete(context.Background())
		if err != nil {
			log.Fatalf("failed to delete device: %v", err)
		}
		log.Println("Successfully deleted WhatsApp session.")
	} else {
		log.Println("No WhatsApp session found.")
	}
}
