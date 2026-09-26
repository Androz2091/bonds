package models

import "time"

// AuditEvent records operational outcomes without recording personal content,
// request bodies, URL query strings, credentials or rendered error messages.
type AuditEvent struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ActorUserID *string   `json:"actor_user_id" gorm:"type:text;index"`
	AccountID   *string   `json:"account_id" gorm:"type:text;index"`
	VaultID     *string   `json:"vault_id" gorm:"type:text;index"`
	Method      string    `json:"method" gorm:"size:8;not null"`
	Route       string    `json:"route" gorm:"size:256;not null"`
	Status      int       `json:"status" gorm:"not null;index"`
	RequestID   string    `json:"request_id" gorm:"size:64;index"`
	CreatedAt   time.Time `json:"created_at" gorm:"index"`
}
