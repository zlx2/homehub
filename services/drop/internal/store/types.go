package store

import (
	"errors"
	"time"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrInvalidInput  = errors.New("invalid input")
	ErrQuotaExceeded = errors.New("storage quota exceeded")
)

type Attachment struct {
	ID           string    `json:"id"`
	ItemID       string    `json:"-"`
	OriginalName string    `json:"original_name"`
	StorageName  string    `json:"-"`
	MediaType    string    `json:"media_type"`
	Size         int64     `json:"size"`
	SHA256       []byte    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Item struct {
	ID             string       `json:"id"`
	Text           string       `json:"text,omitempty"`
	CreatorSubject string       `json:"creator_subject"`
	ActorSubject   string       `json:"actor_subject"`
	CreatedAt      time.Time    `json:"created_at"`
	ExpiresAt      time.Time    `json:"expires_at"`
	TotalSize      int64        `json:"total_size"`
	Attachments    []Attachment `json:"attachments"`
}

type PendingAttachment struct {
	TempPath     string
	OriginalName string
	MediaType    string
	Size         int64
	SHA256       []byte
}

type CreateInput struct {
	Text           string
	CreatorSubject string
	ActorSubject   string
	TTL            time.Duration
	IdempotencyKey []byte
	Attachments    []PendingAttachment
}

type ListOptions struct {
	Limit    int
	Before   time.Time
	BeforeID string
}
