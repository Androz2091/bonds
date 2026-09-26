package services

import (
	"fmt"
	"testing"
	"time"

	"github.com/naiba/bonds/internal/dto"
	"github.com/naiba/bonds/internal/models"
	"github.com/naiba/bonds/internal/testutil"
)

func setupInvitationTest(t *testing.T) (*InvitationService, string, string) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	cfg := testutil.TestJWTConfig()
	authSvc := NewAuthService(db, cfg)

	resp, err := authSvc.Register(dto.RegisterRequest{
		FirstName: "Test",
		LastName:  "User",
		Email:     "invite-test@example.com",
		Password:  "password123",
	}, "en")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	mailer := &NoopMailer{}
	svc := NewInvitationService(db, mailer, "http://localhost:8080")
	return svc, resp.User.AccountID, resp.User.ID
}

func TestCreateInvitation(t *testing.T) {
	svc, accountID, userID := setupInvitationTest(t)

	inv, err := svc.Create(accountID, userID, dto.CreateInvitationRequest{
		Email:      "invited@example.com",
		Permission: 300,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if inv.Email != "invited@example.com" {
		t.Errorf("Expected email 'invited@example.com', got '%s'", inv.Email)
	}
	if inv.Permission != 300 {
		t.Errorf("Expected permission 300, got %d", inv.Permission)
	}
	if inv.ID == 0 {
		t.Error("Expected non-zero ID")
	}
	if inv.AcceptedAt != nil {
		t.Error("Expected AcceptedAt to be nil")
	}
	if inv.ExpiresAt.Before(time.Now()) {
		t.Error("Expected ExpiresAt to be in the future")
	}
}

func TestAcceptInvitation(t *testing.T) {
	svc, accountID, userID := setupInvitationTest(t)

	inv, err := svc.Create(accountID, userID, dto.CreateInvitationRequest{
		Email:      "accept@example.com",
		Permission: 200,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var invitation models.Invitation
	if err := svc.db.First(&invitation, inv.ID).Error; err != nil {
		t.Fatalf("Failed to load invitation: %v", err)
	}

	accepted, err := svc.Accept(dto.AcceptInvitationRequest{
		Token:     invitation.Token,
		FirstName: "New",
		LastName:  "User",
		Password:  "newpassword123",
	}, "en")
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}
	if accepted.AcceptedAt == nil {
		t.Error("Expected AcceptedAt to be set")
	}

	var user models.User
	if err := svc.db.Where("email = ?", "accept@example.com").First(&user).Error; err != nil {
		t.Fatalf("Expected user to be created: %v", err)
	}
	if user.AccountID != accountID {
		t.Errorf("Expected account_id '%s', got '%s'", accountID, user.AccountID)
	}
	if user.InvitationCode == nil || *user.InvitationCode != invitation.Token {
		t.Error("Expected InvitationCode to match token")
	}
	if user.InvitationAcceptedAt == nil {
		t.Error("Expected InvitationAcceptedAt to be set")
	}
}

func TestAcceptExpiredInvitation(t *testing.T) {
	svc, accountID, userID := setupInvitationTest(t)

	inv, err := svc.Create(accountID, userID, dto.CreateInvitationRequest{
		Email:      "expired@example.com",
		Permission: 300,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	svc.db.Model(&models.Invitation{}).Where("id = ?", inv.ID).
		Update("expires_at", time.Now().Add(-1*time.Hour))

	var invitation models.Invitation
	svc.db.First(&invitation, inv.ID)

	_, err = svc.Accept(dto.AcceptInvitationRequest{
		Token:     invitation.Token,
		FirstName: "Late",
		Password:  "password123",
	}, "en")
	if err != ErrInvitationExpired {
		t.Fatalf("Expected ErrInvitationExpired, got: %v", err)
	}
}

func TestAcceptAlreadyAccepted(t *testing.T) {
	svc, accountID, userID := setupInvitationTest(t)

	inv, err := svc.Create(accountID, userID, dto.CreateInvitationRequest{
		Email:      "double@example.com",
		Permission: 300,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var invitation models.Invitation
	svc.db.First(&invitation, inv.ID)

	_, err = svc.Accept(dto.AcceptInvitationRequest{
		Token:     invitation.Token,
		FirstName: "First",
		Password:  "password123",
	}, "en")
	if err != nil {
		t.Fatalf("First accept failed: %v", err)
	}

	_, err = svc.Accept(dto.AcceptInvitationRequest{
		Token:     invitation.Token,
		FirstName: "Second",
		Password:  "password123",
	}, "en")
	if err != ErrInvitationNotFound {
		t.Fatalf("Expected ErrInvitationNotFound, got: %v", err)
	}
}

func TestListInvitations(t *testing.T) {
	svc, accountID, userID := setupInvitationTest(t)

	for i := 0; i < 3; i++ {
		_, err := svc.Create(accountID, userID, dto.CreateInvitationRequest{
			Email:      fmt.Sprintf("list%d@example.com", i),
			Permission: 300,
		})
		if err != nil {
			t.Fatalf("Create %d failed: %v", i, err)
		}
	}

	list, meta, err := svc.List(accountID, 0, 0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("Expected 3 invitations, got %d", len(list))
	}
	if meta.Total != 3 {
		t.Errorf("Expected meta.Total=3, got %d", meta.Total)
	}
}

func TestListInvitations_Pagination(t *testing.T) {
	svc, accountID, userID := setupInvitationTest(t)

	for i := 0; i < 5; i++ {
		_, err := svc.Create(accountID, userID, dto.CreateInvitationRequest{
			Email:      fmt.Sprintf("page%d@example.com", i),
			Permission: 300,
		})
		if err != nil {
			t.Fatalf("Create %d failed: %v", i, err)
		}
	}

	page1, meta1, err := svc.List(accountID, 1, 2)
	if err != nil {
		t.Fatalf("List page1 failed: %v", err)
	}
	if len(page1) != 2 {
		t.Errorf("Expected 2 invitations on page 1, got %d", len(page1))
	}
	if meta1.Total != 5 {
		t.Errorf("Expected total=5, got %d", meta1.Total)
	}
	if meta1.TotalPages != 3 {
		t.Errorf("Expected total_pages=3, got %d", meta1.TotalPages)
	}
}

func TestDeleteInvitation(t *testing.T) {
	svc, accountID, userID := setupInvitationTest(t)

	inv, err := svc.Create(accountID, userID, dto.CreateInvitationRequest{
		Email:      "delete@example.com",
		Permission: 300,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := svc.Delete(inv.ID, accountID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	list, _, err := svc.List(accountID, 0, 0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("Expected 0 invitations after delete, got %d", len(list))
	}
}

func TestAcceptInvitation_DoesNotGrantUnselectedVaultAccess(t *testing.T) {
	db := testutil.SetupTestDB(t)
	cfg := testutil.TestJWTConfig()
	authSvc := NewAuthService(db, cfg)
	vaultSvc := NewVaultService(db)

	resp, err := authSvc.Register(dto.RegisterRequest{
		FirstName: "Owner",
		LastName:  "User",
		Email:     "vault-owner@example.com",
		Password:  "password123",
	}, "en")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	v1, err := vaultSvc.CreateVault(resp.User.AccountID, resp.User.ID, dto.CreateVaultRequest{Name: "Vault A"}, "en")
	if err != nil {
		t.Fatalf("CreateVault A failed: %v", err)
	}
	v2, err := vaultSvc.CreateVault(resp.User.AccountID, resp.User.ID, dto.CreateVaultRequest{Name: "Vault B"}, "en")
	if err != nil {
		t.Fatalf("CreateVault B failed: %v", err)
	}

	mailer := &NoopMailer{}
	invSvc := NewInvitationService(db, mailer, "http://localhost:8080")

	inv, err := invSvc.Create(resp.User.AccountID, resp.User.ID, dto.CreateInvitationRequest{
		Email:      "invited-vault@example.com",
		Permission: models.PermissionEditor,
	})
	if err != nil {
		t.Fatalf("Create invitation failed: %v", err)
	}

	var invitation models.Invitation
	if err := db.First(&invitation, inv.ID).Error; err != nil {
		t.Fatalf("Load invitation failed: %v", err)
	}

	_, err = invSvc.Accept(dto.AcceptInvitationRequest{
		Token:     invitation.Token,
		FirstName: "Invited",
		LastName:  "Person",
		Password:  "password123",
	}, "en")
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	var newUser models.User
	if err := db.Where("email = ?", "invited-vault@example.com").First(&newUser).Error; err != nil {
		t.Fatalf("New user not found: %v", err)
	}

	var userVaults []models.UserVault
	if err := db.Where("user_id = ?", newUser.ID).Find(&userVaults).Error; err != nil {
		t.Fatalf("UserVault query failed: %v", err)
	}
	if len(userVaults) != 0 {
		t.Fatalf("account invitation must not grant vault access, got %d memberships", len(userVaults))
	}

	var contactCount int64
	if err := db.Model(&models.Contact{}).Where("vault_id IN ?", []string{v1.ID, v2.ID}).Count(&contactCount).Error; err != nil {
		t.Fatalf("Count contacts failed: %v", err)
	}
	if contactCount != 0 {
		t.Fatalf("accepting invitation created %d contact(s), want 0", contactCount)
	}
	var membership models.AccountMembership
	if err := db.Where("account_id = ? AND user_id = ?", resp.User.AccountID, newUser.ID).First(&membership).Error; err != nil {
		t.Fatalf("account invitation did not grant account membership: %v", err)
	}
}

func TestCreateInvitationDuplicateEmail(t *testing.T) {
	svc, accountID, userID := setupInvitationTest(t)

	_, err := svc.Create(accountID, userID, dto.CreateInvitationRequest{
		Email:      "invite-test@example.com",
		Permission: 300,
	})
	if err != ErrUserAlreadyExists {
		t.Fatalf("Expected ErrUserAlreadyExists, got: %v", err)
	}
}

func TestExistingUserAcceptsAccountAndVaultWithoutNewCredentials(t *testing.T) {
	db := testutil.SetupTestDB(t)
	auth := NewAuthService(db, testutil.TestJWTConfig())
	vaultSvc := NewVaultService(db)
	owner, err := auth.Register(dto.RegisterRequest{FirstName: "Owner", Email: "owner@example.test", Password: "password123"}, "en")
	if err != nil {
		t.Fatal(err)
	}
	guest, err := auth.Register(dto.RegisterRequest{FirstName: "Guest", Email: "guest@example.test", Password: "guestpass123"}, "en")
	if err != nil {
		t.Fatal(err)
	}
	vault, err := vaultSvc.CreateVault(owner.User.AccountID, owner.User.ID, dto.CreateVaultRequest{Name: "Private"}, "en")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewInvitationService(db, &NoopMailer{}, "http://localhost:8080")
	accountInvite, err := svc.Create(owner.User.AccountID, owner.User.ID, dto.CreateInvitationRequest{Email: guest.User.Email})
	if err != nil {
		t.Fatal(err)
	}
	var accountRecord models.Invitation
	if err := db.First(&accountRecord, accountInvite.ID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Accept(dto.AcceptInvitationRequest{Token: accountRecord.Token, FirstName: "Wrong", Password: "password123"}, "en"); err != ErrExistingUserLoginRequired {
		t.Fatalf("expected existing identity to require login, got %v", err)
	}
	if _, err := svc.AcceptExisting(accountRecord.Token, owner.User.ID); err != ErrInvitationIdentityMismatch {
		t.Fatalf("other identity may not accept invite: %v", err)
	}
	if _, err := svc.AcceptExisting(accountRecord.Token, guest.User.ID); err != nil {
		t.Fatal(err)
	}
	var memberships []models.AccountMembership
	if err := db.Where("user_id = ?", guest.User.ID).Find(&memberships).Error; err != nil {
		t.Fatal(err)
	}
	if len(memberships) != 2 {
		t.Fatalf("expected existing and invited account, got %d", len(memberships))
	}
	if err := vaultSvc.CheckUserVaultAccess(guest.User.ID, vault.ID, models.PermissionViewer); err == nil {
		t.Fatal("account membership must not grant access to private vault")
	}
	vaultInvite, err := svc.CreateVault(vault.ID, owner.User.ID, dto.CreateVaultInvitationRequest{Email: guest.User.Email, Permission: models.PermissionViewer})
	if err != nil {
		t.Fatal(err)
	}
	var vaultRecord models.Invitation
	if err := db.First(&vaultRecord, vaultInvite.ID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AcceptExisting(vaultRecord.Token, guest.User.ID); err != nil {
		t.Fatal(err)
	}
	if err := vaultSvc.CheckUserVaultAccess(guest.User.ID, vault.ID, models.PermissionViewer); err != nil {
		t.Fatalf("vault invite should grant scoped access: %v", err)
	}
	if _, err := auth.Login(dto.LoginRequest{Email: guest.User.Email, Password: "guestpass123"}); err != nil {
		t.Fatalf("the existing password must remain valid: %v", err)
	}
	var users int64
	if err := db.Model(&models.User{}).Where("email = ?", guest.User.Email).Count(&users).Error; err != nil || users != 1 {
		t.Fatalf("identity was duplicated: %d, %v", users, err)
	}
}
