package database

import (
	"github.com/naiba/bonds/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Backfill only home-account memberships. Existing vault memberships are
// deliberately untouched; a vault member need not belong to its account.
func backfillAccountMemberships(db *gorm.DB) error {
	var users []models.User
	if err := db.Select("id, account_id, is_account_administrator").Find(&users).Error; err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for _, user := range users {
			membership := models.AccountMembership{
				AccountID: user.AccountID, UserID: user.ID, IsAdmin: user.IsAccountAdministrator,
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&membership).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
