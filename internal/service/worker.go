package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"tekstobot/internal/ai"
	"tekstobot/internal/db"
)

type Worker struct {
	Repo        *db.Repository
	Transcriber ai.Transcriber
	WAClient    *whatsmeow.Client
}

func NewWorker(repo *db.Repository, transcriber ai.Transcriber, wa *whatsmeow.Client) *Worker {
	return &Worker{
		Repo:        repo,
		Transcriber: transcriber,
		WAClient:    wa,
	}
}

func (w *Worker) Start(mediaChan <-chan *events.Message) {
	for msg := range mediaChan {
		go w.processMessage(msg)
	}
}

func (w *Worker) processMessage(msg *events.Message) {
	audioMsg := msg.Message.GetAudioMessage()
	if audioMsg == nil {
		return
	}

	senderPhone := msg.Info.Sender.User
	chat, msgID := msg.Info.Chat, msg.Info.ID

	// Notify user immediately that processing has started
	w.reply(chat, msgID, "⏳ *Processing...*")

	// 1. Download media
	filePath := filepath.Join("data", "media", fmt.Sprintf("audio_%d.ogg", time.Now().UnixNano()))
	mediaData, err := w.WAClient.Download(context.Background(), audioMsg)
	if err == nil {
		err = os.WriteFile(filePath, mediaData, 0600)
	}
	if err != nil {
		log.Printf("Failed to download media from %s: %v", senderPhone, err)
		w.reply(chat, msgID, "❌ Error: Failed to download media.")
		return
	}

	// 2. Save to database with "pending" status so it shows up in the UI immediately
	mediaID, saveErr := w.Repo.SaveProcessedMedia(&db.ProcessedMedia{
		MediaType:   "audio",
		FilePath:    filePath,
		SenderPhone: senderPhone,
		Status:      "pending",
	})
	if saveErr != nil {
		log.Printf("Failed to save media entry to db: %v", saveErr)
	}

	// 3. Transcribe
	extractedText, err := w.Transcriber.ProcessAudio(filePath)
	status, errorMsg := "completed", ""
	if err != nil {
		status, errorMsg = "error", err.Error()
		log.Printf("Whisper error: %v", err)
	}

	// 4. Update database record with result
	if mediaID != 0 {
		if updateErr := w.Repo.UpdateProcessedMedia(mediaID, extractedText, status, errorMsg); updateErr != nil {
			log.Printf("Failed to update media entry in db: %v", updateErr)
		}
	}

	// 5. Reply with the result
	if status == "completed" {
		w.reply(chat, msgID, fmt.Sprintf("📝 *Transcription:*\n\n%s", extractedText))
	} else {
		w.reply(chat, msgID, "❌ Error: Failed to transcribe audio with Whisper.")
	}
}

// reply envia um texto citando a mensagem original.
func (w *Worker) reply(chat types.JID, msgID string, text string) {
	replyMsg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String(text),
			ContextInfo: &waE2E.ContextInfo{
				StanzaID:    proto.String(msgID),
				Participant: proto.String(chat.String()),
			},
		},
	}
	if _, err := w.WAClient.SendMessage(context.Background(), chat, replyMsg); err != nil {
		log.Printf("Failed to send reply to %s: %v", chat.User, err)
	}
}
