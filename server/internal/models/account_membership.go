package models

import "time"

// AccountMembership grants access to account-level settings. It does not grant
// access to any vault: vault access is exclusively represented by UserVault.
type AccountMembership struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	AccountID string    `json:"account_id" gorm:"type:text;not null;uniqueIndex:idx_account_member"`
	UserID    string    `json:"user_id" gorm:"type:text;not null;uniqueIndex:idx_account_member;index"`
	IsAdmin   bool      `json:"is_admin" gorm:"not null;default:false"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
