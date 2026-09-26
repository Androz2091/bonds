package models

import "time"

// AuthActionToken is a one-time credential delivered to the user's email.
// Only a SHA-256 digest is stored; the raw credential must never appear in an
// admin response, audit row, or application log.
type AuthActionToken struct {
	ID          uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID      string     `json:"user_id" gorm:"type:text;not null;index"`
	TokenHash   string     `json:"-" gorm:"size:64;uniqueIndex;not null"`
	Purpose     string     `json:"purpose" gorm:"size:32;not null"`
	TargetEmail string     `json:"-" gorm:"size:255"`
	ExpiresAt   time.Time  `json:"expires_at" gorm:"not null"`
	UsedAt      *time.Time `json:"used_at"`
	CreatedAt   time.Time  `json:"created_at"`
}
