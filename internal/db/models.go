package db

import (
	"time"
)

type AllowedPhone struct {
	ID          int
	PhoneNumber string
	Description string
	CreatedAt   time.Time
}

type ProcessedMedia struct {
	ID            int
	MediaType     string
	FilePath      string
	ExtractedText string
	SenderPhone   string
	Status        string
	ErrorMessage  string
	CreatedAt     time.Time
}

type UnauthorizedAttempt struct {
	ID          int
	PhoneNumber string
	PushName    string
	LastAttempt time.Time
}
