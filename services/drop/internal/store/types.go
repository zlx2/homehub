package store

import (
	"errors"
	"time"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrQuotaExceeded = errors.New("storage quota exceeded")
	ErrInvalidInput  = errors.New("invalid input")
	ErrCodeInvalid   = errors.New("authorization code is invalid or expired")
)

type Attachment struct {
	ID           string
	ItemID       string
	OriginalName string
	StorageName  string
	MIMEType     string
	Size         int64
	CreatedAt    time.Time
}

type Item struct {
	ID          string
	TextInline  []byte
	TextStorage string
	TextSize    int64
	Source      string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	TotalSize   int64
	Attachments []Attachment
}

type PendingAttachment struct {
	TempPath     string
	OriginalName string
	MIMEType     string
	Size         int64
}

type CreateItemInput struct {
	TextTempPath   string
	TextSize       int64
	Attachments    []PendingAttachment
	Source         string
	TTL            time.Duration
	IdempotencyKey []byte
}

type ListOptions struct {
	Limit    int
	Before   time.Time
	BeforeID string
}

type Usage struct {
	UsedBytes       int64
	QuotaBytes      int64
	ItemCount       int64
	AttachmentCount int64
}

type SessionMetadata struct {
	DeviceName string
	LastIP     string
}

type TrustedSession struct {
	ID         int64
	DeviceName string
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
	LastIP     string
}

type TrafficTotals struct {
	PublicBytes    int64 `json:"public_bytes"`
	TailscaleBytes int64 `json:"tailscale_bytes"`
	HermesBytes    int64 `json:"hermes_bytes"`
	TotalBytes     int64 `json:"total_bytes"`
	Requests       int64 `json:"requests"`
}

type HourlyTraffic struct {
	Hour time.Time `json:"hour"`
	TrafficTotals
}

type TrafficReport struct {
	Last24Hours TrafficTotals   `json:"last_24_hours"`
	Last30Days  TrafficTotals   `json:"last_30_days"`
	Hourly      []HourlyTraffic `json:"hourly"`
}
