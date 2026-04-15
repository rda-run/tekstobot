package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
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

type MediaJob struct {
	SenderPhone  string
	MsgID        string
	Chat         types.JID
	MediaType    string // always "audio"
	AudioMessage *waProto.AudioMessage
}

func (w *Worker) Start(mediaChan <-chan *events.Message) {
	for msg := range mediaChan {
		go w.processMessage(msg)
	}
}

func (w *Worker) processMessage(msg *events.Message) {
	senderPhone := msg.Info.Sender.User

	var mediaType string
	var audioMsg *waProto.AudioMessage

	if msg.Message.GetAudioMessage() != nil {
		mediaType = "audio"
		audioMsg = msg.Message.GetAudioMessage()
	} else {
		return
	}

	job := MediaJob{
		SenderPhone:  senderPhone,
		MsgID:        msg.Info.ID,
		Chat:         msg.Info.Chat,
		MediaType:    mediaType,
		AudioMessage: audioMsg,
	}

	var filePath string

	// Notify user immediately that processing has started
	w.replyText(job.Chat, job.MsgID, "⏳ *Processing...*")

	// 1. Download media
	var mediaData []byte
	var err error

	if job.MediaType == "audio" {
		mediaData, err = w.WAClient.Download(context.Background(), job.AudioMessage)
		if err == nil {
			fileName := fmt.Sprintf("audio_%d.ogg", time.Now().UnixNano())
			filePath = filepath.Join("data", "media", fileName)
			err = os.WriteFile(filePath, mediaData, 0644)
		}
	}

	if err != nil {
		w.replyError(job.Chat, job.MsgID, "Failed to download media.")
		return
	}

	// 2. Save to database with "pending" status so it shows up in the UI immediately
	mediaID, saveErr := w.Repo.SaveProcessedMedia(&db.ProcessedMedia{
		MediaType:   job.MediaType,
		FilePath:    filePath,
		SenderPhone: job.SenderPhone,
		Status:      "pending",
	})
	if saveErr != nil {
		log.Printf("Failed to save media entry to db: %v", saveErr)
	}

	// 3. Transcribe
	var extractedText, errorMsg, status string
	status = "completed"

	if job.MediaType == "audio" {
		extractedText, err = w.Transcriber.ProcessAudio(filePath)
		if err != nil {
			status = "error"
			errorMsg = err.Error()
			log.Printf("Whisper error: %v", err)
			w.replyError(job.Chat, job.MsgID, "Failed to transcribe audio with Whisper.")
		}
	}

	// 4. Update database record with result
	if mediaID != 0 {
		if updateErr := w.Repo.UpdateProcessedMedia(mediaID, extractedText, status, errorMsg); updateErr != nil {
			log.Printf("Failed to update media entry in db: %v", updateErr)
		}
	}

	// 5. Send successful reply
	if status == "completed" {
		replyMsg := &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text: proto.String(fmt.Sprintf("📝 *Transcription:*\n\n%s", extractedText)),
				ContextInfo: &waProto.ContextInfo{
					StanzaID:    proto.String(job.MsgID),
					Participant: proto.String(job.Chat.String()),
				},
			},
		}
		_, err = w.WAClient.SendMessage(context.Background(), job.Chat, replyMsg)
		if err != nil {
			log.Printf("Failed to send reply to %s: %v", job.SenderPhone, err)
		}
	}
}

func (w *Worker) replyError(chat types.JID, msgID string, text string) {
	replyMsg := &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(fmt.Sprintf("❌ Error: %s", text)),
			ContextInfo: &waProto.ContextInfo{
				StanzaID:    proto.String(msgID),
				Participant: proto.String(chat.String()),
			},
		},
	}
	w.WAClient.SendMessage(context.Background(), chat, replyMsg)
}

func (w *Worker) replyText(chat types.JID, msgID string, text string) {
	replyMsg := &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(text),
			ContextInfo: &waProto.ContextInfo{
				StanzaID:    proto.String(msgID),
				Participant: proto.String(chat.String()),
			},
		},
	}
	w.WAClient.SendMessage(context.Background(), chat, replyMsg)
}
