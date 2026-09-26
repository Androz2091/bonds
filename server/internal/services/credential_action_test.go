package services

import (
	"errors"
	"strings"
	"testing"

	"github.com/naiba/bonds/internal/dto"
	"github.com/naiba/bonds/internal/models"
	"github.com/naiba/bonds/internal/testutil"
)

type capturedCredentialMailer struct {
	recipient, body string
	sent            map[string]string
	err             error
}

func (m *capturedCredentialMailer) Send(to, _, body string) error {
	m.recipient = to
	m.body = body
	if m.sent == nil {
		m.sent = make(map[string]string)
	}
	m.sent[to] = body
	return m.err
}

func TestEmailChangeNeedsBothAddressesAndPreservesVaultMembership(t *testing.T) {
	db := testutil.SetupTestDB(t)
	auth := NewAuthService(db, testutil.TestJWTConfig())
	registered, err := auth.Register(dto.RegisterRequest{Email: "old@example.test", FirstName: "Owner", LastName: "A", Password: "password123"}, "en")
	if err != nil {
		t.Fatal(err)
	}
	vault, err := NewVaultService(db).CreateVault(registered.User.AccountID, registered.User.ID, dto.CreateVaultRequest{Name: "Private"}, "en")
	if err != nil {
		t.Fatal(err)
	}
	mailer := &capturedCredentialMailer{}
	credentials := NewCredentialActionService(db, mailer, "http://example.test", nil)
	admin := NewAdminService(db, t.TempDir())
	admin.SetCredentialActions(credentials)
	if err := admin.RequestEmailChange(registered.User.ID, "new@example.test"); err != nil {
		t.Fatal(err)
	}
	oldMail := &capturedCredentialMailer{body: mailer.sent["old@example.test"]}
	newMail := &capturedCredentialMailer{body: mailer.sent["new@example.test"]}
	if err := credentials.ConfirmEmailChange(credentialToken(t, oldMail)); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Login(dto.LoginRequest{Email: "new@example.test", Password: "password123"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("one inbox was sufficient to change email: %v", err)
	}
	if err := credentials.ConfirmEmailChange(credentialToken(t, newMail)); err != nil {
		t.Fatal(err)
	}
	login, err := auth.Login(dto.LoginRequest{Email: "new@example.test", Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	if login.User.ID != registered.User.ID || login.User.AccountID != registered.User.AccountID {
		t.Fatal("identity or account changed")
	}
	if _, err := NewVaultService(db).GetVault(vault.ID, login.User.ID); err != nil {
		t.Fatalf("vault membership lost: %v", err)
	}
	var channel models.UserNotificationChannel
	if err := db.Where("user_id = ? AND type = ?", login.User.ID, "email").First(&channel).Error; err != nil {
		t.Fatal(err)
	}
	if channel.Content != "new@example.test" {
		t.Fatalf("old email notification channel still active: %s", channel.Content)
	}
}
func (m *capturedCredentialMailer) Close() {}

func credentialToken(t *testing.T, mailer *capturedCredentialMailer) string {
	t.Helper()
	part := strings.SplitN(mailer.body, "?token=", 2)
	if len(part) != 2 {
		t.Fatalf("missing password link in mail")
	}
	return strings.SplitN(part[1], "\"", 2)[0]
}

func TestAdminPasswordSetupAndResetPreserveMembership(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mailer := &capturedCredentialMailer{}
	credentials := NewCredentialActionService(db, mailer, "http://example.test", nil)
	admin := NewAdminService(db, t.TempDir())
	admin.SetCredentialActions(credentials)
	auth := NewAuthService(db, testutil.TestJWTConfig())

	created, err := admin.CreateUser(dto.AdminCreateUserRequest{Email: "Example@Example.Test", FirstName: "A"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Email != "example@example.test" || created.IsInstanceAdministrator {
		t.Fatalf("unsafe user creation: %+v", created)
	}
	if mailer.recipient != created.Email {
		t.Fatalf("password email recipient = %q", mailer.recipient)
	}
	if _, err := auth.Login(dto.LoginRequest{Email: created.Email, Password: "password123"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("account should not have an operator-selected password: %v", err)
	}
	if err := credentials.Complete(credentialToken(t, mailer), "password123"); err != nil {
		t.Fatal(err)
	}
	login, err := auth.Login(dto.LoginRequest{Email: created.Email, Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	accountID := login.User.AccountID
	if err := admin.SendPasswordReset(created.ID); err != nil {
		t.Fatal(err)
	}
	reset := credentialToken(t, mailer)
	if err := admin.SendPasswordReset(created.ID); err != nil {
		t.Fatal(err)
	}
	if err := credentials.Complete(reset, "obsolete-password"); !errors.Is(err, ErrInvalidPasswordAction) {
		t.Fatalf("superseded token accepted: %v", err)
	}
	reset = credentialToken(t, mailer)
	if err := credentials.Complete(reset, "newpassword123"); err != nil {
		t.Fatal(err)
	}
	if err := credentials.Complete(reset, "newpassword123"); !errors.Is(err, ErrInvalidPasswordAction) {
		t.Fatalf("one-time token reused: %v", err)
	}
	if _, err := auth.Login(dto.LoginRequest{Email: created.Email, Password: "password123"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password valid: %v", err)
	}
	login, err = auth.Login(dto.LoginRequest{Email: created.Email, Password: "newpassword123"})
	if err != nil || login.User.AccountID != accountID {
		t.Fatalf("account membership lost: %v", err)
	}
	var member models.AccountMembership
	if err := db.Where("user_id = ? AND account_id = ?", created.ID, accountID).First(&member).Error; err != nil {
		t.Fatal(err)
	}
}

func TestAdminCreateUserDoesNotPersistIfMailFails(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mailer := &capturedCredentialMailer{err: errors.New("SMTP unreachable")}
	admin := NewAdminService(db, t.TempDir())
	admin.SetCredentialActions(NewCredentialActionService(db, mailer, "http://example.test", nil))
	if _, err := admin.CreateUser(dto.AdminCreateUserRequest{Email: "failed@example.test", FirstName: "A"}); err == nil {
		t.Fatal("expected mail failure")
	}
	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("orphaned user after failure: %d, %v", count, err)
	}
}
