package whatsapp

import (
	"context"
	"fmt"
	"log"
	"sync"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	"tekstobot/internal/db"
)

type Client struct {
	WAClient   *whatsmeow.Client
	Repo       *db.Repository
	MediaChan  chan *events.Message
	QRCode     string
	QRLock     sync.RWMutex
	AdminPhone string
}

func NewClient(repo *db.Repository, dsn string, adminPhone string) (*Client, error) {
	dbLog := waLog.Stdout("Database", "WARN", true)
	container, err := sqlstore.New(context.Background(), "postgres", dsn, dbLog)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to local wa_session postgres DB: %w", err)
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get device store: %w", err)
	}

	clientLog := waLog.Stdout("Client", "WARN", true)
	waClient := whatsmeow.NewClient(deviceStore, clientLog)

	c := &Client{
		WAClient:   waClient,
		Repo:       repo,
		MediaChan:  make(chan *events.Message, 100),
		AdminPhone: adminPhone,
	}

	waClient.AddEventHandler(c.eventHandler)

	return c, nil
}

func (c *Client) SendMessage(phone string, text string) error {
	recipient, err := types.ParseJID(phone + "@s.whatsapp.net")
	if err != nil {
		return fmt.Errorf("failed to parse recipient JID: %w", err)
	}

	msg := &waE2E.Message{
		Conversation: proto.String(text),
	}

	_, err = c.WAClient.SendMessage(context.Background(), recipient, msg)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// Notify envia uma mensagem em segundo plano, registrando falhas no log.
func (c *Client) Notify(phone string, text string) {
	go func() {
		if err := c.SendMessage(phone, text); err != nil {
			log.Printf("Failed to notify %s: %v", phone, err)
		}
	}()
}

func (c *Client) Start() error {
	if c.WAClient.Store.ID == nil {
		// No ID stored, new login
		qrChan, err := c.WAClient.GetQRChannel(context.Background())
		if err != nil {
			return fmt.Errorf("failed to get QR channel: %w", err)
		}
		
		err = c.WAClient.Connect()
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		
		go func() {
			for evt := range qrChan {
				if evt.Event == "code" {
					log.Println("QR code generated (available via UI)")
					c.QRLock.Lock()
					c.QRCode = evt.Code
					c.QRLock.Unlock()
				} else {
					log.Println("Login event:", evt.Event)
				}
			}
		}()
	} else {
		// Already logged in
		err := c.WAClient.Connect()
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		log.Println("WhatsApp Client successfully connected")
	}

	return nil
}

func (c *Client) Stop() {
	if c.WAClient != nil {
		c.WAClient.Disconnect()
	}
}

func (c *Client) ResetSession() error {
	log.Println("Resetting WhatsApp session...")
	err := c.WAClient.Logout(context.Background())
	if err != nil {
		return err
	}
	
	c.QRLock.Lock()
	c.QRCode = ""
	c.QRLock.Unlock()

	return nil
}

func (c *Client) GetQR() string {
	c.QRLock.RLock()
	defer c.QRLock.RUnlock()
	return c.QRCode
}

func (c *Client) eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		c.handleMessage(v)
	case *events.LoggedOut:
		log.Println("Received LoggedOut event. WhatsApp session was disconnected.")
		c.ResetSession()
		// Application must be restarted to regenerate QR, or a reconnect flow triggered.
	}
}

func (c *Client) handleMessage(msg *events.Message) {
	// Ignores messages from the bot itself, Groups, or Broadcast lists
	if msg.Info.IsFromMe || msg.Info.IsGroup || msg.Info.Chat.Server == "broadcast" {
		return
	}

	senderPhone := msg.Info.Sender.User
	pushName := msg.Info.PushName
	fullJID := msg.Info.Sender.String()

	allowed, err := c.Repo.IsPhoneAllowed(senderPhone)
	if err != nil {
		log.Printf("Error checking if phone is allowed for %s: %v", senderPhone, err)
		return
	}

	if !allowed {
		log.Printf("Message from non-whitelisted phone ignored: %s (Name: %s, Full JID: %s)", senderPhone, pushName, fullJID)
		needsNotification, err := c.Repo.SaveUnauthorizedAttempt(senderPhone, pushName)
		if err != nil {
			log.Printf("Error saving unauthorized attempt: %v", err)
			return
		}

		if needsNotification {
			// Notify User
			c.Notify(senderPhone, "Olá! O seu acesso está pendente e será autorizado pela nossa equipe em breve.")

			// Notify Admin
			if c.AdminPhone != "" {
				adminMsg := fmt.Sprintf("⚠️ Novo usuário aguardando autorização no TekstoBot: %s (%s). Acesse a dashboard para aprovar.", pushName, senderPhone)
				c.Notify(c.AdminPhone, adminMsg)
			}
		}
		return
	}

	log.Printf("Received message from allowed user %s", senderPhone)

	// Dispatch to worker through channel
	if msg.Message.GetAudioMessage() != nil {
		select {
		case c.MediaChan <- msg:
		default:
			log.Println("Media channel full, dropping message")
		}
	} else {
		log.Println("Ignored text/image/other message type from", senderPhone)
	}
}
