package services

import (
	"testing"

	"github.com/naiba/bonds/internal/dto"
	"github.com/naiba/bonds/internal/models"
	"github.com/naiba/bonds/internal/testutil"
)

func setupAccountCancelTest(t *testing.T) (*AccountCancelService, string, string, *VaultService) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	cfg := testutil.TestJWTConfig()
	authSvc := NewAuthService(db, cfg)
	vaultSvc := NewVaultService(db)

	resp, err := authSvc.Register(dto.RegisterRequest{
		FirstName: "Test", LastName: "User",
		Email: "cancel-test@example.com", Password: "password123",
	}, "en")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	return NewAccountCancelService(db), resp.User.ID, resp.User.AccountID, vaultSvc
}

func TestAccountCancelSuccess(t *testing.T) {
	svc, userID, accountID, vaultSvc := setupAccountCancelTest(t)

	_, err := vaultSvc.CreateVault(accountID, userID, dto.CreateVaultRequest{Name: "V1"}, "en")
	if err != nil {
		t.Fatalf("CreateVault failed: %v", err)
	}

	if err := svc.Cancel(userID, accountID, "password123"); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	var count int64
	svc.db.Model(&models.User{}).Where("account_id = ?", accountID).Count(&count)
	if count != 0 {
		t.Errorf("Expected 0 users after cancel, got %d", count)
	}
}

func TestAccountCancelWrongPassword(t *testing.T) {
	svc, userID, accountID, _ := setupAccountCancelTest(t)

	err := svc.Cancel(userID, accountID, "wrongpassword")
	if err != ErrPasswordMismatch {
		t.Errorf("Expected ErrPasswordMismatch, got %v", err)
	}
}

func TestAccountCancelKeepsIdentityAndOtherAccount(t *testing.T) {
	svc, userID, oldAccountID, _ := setupAccountCancelTest(t)
	other := models.Account{}
	if err := svc.db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Create(&models.AccountMembership{AccountID: other.ID, UserID: userID, IsAdmin: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.Cancel(userID, oldAccountID, "password123"); err != nil {
		t.Fatal(err)
	}
	var user models.User
	if err := svc.db.First(&user, "id = ?", userID).Error; err != nil {
		t.Fatal(err)
	}
	if user.AccountID != other.ID || !user.IsAccountAdministrator {
		t.Fatalf("identity not rehomed: %+v", user)
	}
	var memberships int64
	svc.db.Model(&models.AccountMembership{}).Where("user_id = ?", userID).Count(&memberships)
	if memberships != 1 {
		t.Fatalf("expected surviving membership: %d", memberships)
	}
}
