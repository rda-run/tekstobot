package db

import (
	"time"
)

type AllowedPhone struct {
	ID          int       `json:"id"`
	PhoneNumber string    `json:"phoneNumber"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
}

type ProcessedMedia struct {
	ID            int       `json:"id"`
	MediaType     string    `json:"mediaType"`
	FilePath      string    `json:"filePath"`
	ExtractedText string    `json:"extractedText"`
	SenderPhone   string    `json:"senderPhone"`
	SenderName    string    `json:"senderName"`
	Status        string    `json:"status"`
	ErrorMessage  string    `json:"errorMessage"`
	CreatedAt     time.Time `json:"createdAt"`
}

type UnauthorizedAttempt struct {
	ID          int       `json:"id"`
	PhoneNumber string    `json:"phoneNumber"`
	PushName    string    `json:"pushName"`
	LastAttempt time.Time `json:"lastAttempt"`
}

type MediaUser struct {
	PhoneNumber string `json:"phoneNumber"`
	Name        string `json:"name"`
}
