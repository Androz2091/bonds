package database

import (
	"path/filepath"
	"testing"

	"github.com/naiba/bonds/internal/config"
	"github.com/naiba/bonds/internal/models"
)

func TestAutoMigrateBackfillsHomeMembershipOnce(t *testing.T) {
	db, err := Connect(&config.DatabaseConfig{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "legacy.db")}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	account := models.Account{}
	if err := db.Create(&account).Error; err != nil {
		t.Fatal(err)
	}
	user := models.User{AccountID: account.ID, Email: "legacy@example.test", IsAccountAdministrator: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := AutoMigrate(db); err != nil {
			t.Fatal(err)
		}
	}
	var memberships []models.AccountMembership
	if err := db.Where("account_id = ? AND user_id = ?", account.ID, user.ID).Find(&memberships).Error; err != nil {
		t.Fatal(err)
	}
	if len(memberships) != 1 || !memberships[0].IsAdmin {
		t.Fatalf("backfilled membership = %+v", memberships)
	}
}
