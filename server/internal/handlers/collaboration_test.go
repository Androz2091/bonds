package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/naiba/bonds/internal/models"
)

func TestVaultInvitationExistingUserRequiresAcceptanceAndIsolatesAccounts(t *testing.T) {
	ts := setupTestServer(t)
	ownerToken, owner := ts.registerTestUser(t, "scoped-owner@example.com")
	guestToken, guest := ts.registerTestUser(t, "scoped-guest@example.com")
	vault := ts.createTestVault(t, ownerToken, "Private collection")
	path := fmt.Sprintf("/api/vaults/%s/settings/invitations", vault.ID)
	rec := ts.doRequest(http.MethodPost, path, `{"email":"scoped-guest@example.com","permission":300}`, ownerToken)
	if rec.Code != http.StatusCreated {
		t.Fatalf("invite failed: %d %s", rec.Code, rec.Body.String())
	}
	if rec = ts.doRequest(http.MethodGet, "/api/vaults/"+vault.ID, "", guestToken); rec.Code != http.StatusForbidden {
		t.Fatalf("invitation granted premature vault access: %d", rec.Code)
	}
	var invitation models.Invitation
	if err := ts.db.Where("vault_id = ? AND email = ?", vault.ID, guest.User.Email).First(&invitation).Error; err != nil {
		t.Fatal(err)
	}
	rec = ts.doRequest(http.MethodPost, "/api/invitations/accept-existing", fmt.Sprintf(`{"token":%q}`, invitation.Token), guestToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("accept failed: %d %s", rec.Code, rec.Body.String())
	}
	if rec = ts.doRequest(http.MethodGet, "/api/vaults/"+vault.ID, "", guestToken); rec.Code != http.StatusOK {
		t.Fatalf("accepted user cannot access vault: %d", rec.Code)
	}
	rec = ts.doRequest(http.MethodGet, "/api/vaults/"+vault.ID+"/personalize/genders", "", guestToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("vault reference values not accessible: %d", rec.Code)
	}
	if rec = ts.doRequest(http.MethodPost, "/api/auth/switch-account", fmt.Sprintf(`{"account_id":%q}`, owner.User.AccountID), guestToken); rec.Code == http.StatusOK {
		t.Fatal("vault member became account member")
	}
	var memberCount int64
	ts.db.Model(&models.AccountMembership{}).Where("account_id = ? AND user_id = ?", owner.User.AccountID, guest.User.ID).Count(&memberCount)
	if memberCount != 0 {
		t.Fatalf("vault invite leaked account permissions: %d", memberCount)
	}
	rec = ts.doRequest(http.MethodGet, "/api/admin/audit", "", ownerToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin audit unavailable: %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), vault.ID) || strings.Contains(rec.Body.String(), guest.User.Email) {
		t.Fatal("private vault ID or guest email leaked through admin audit")
	}
	rec = ts.doRequest(http.MethodGet, "/api/vaults/"+vault.ID+"/settings/audit", "", ownerToken)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "invitations") {
		t.Fatalf("vault manager cannot review vault activity: %d %s", rec.Code, rec.Body.String())
	}
}

func TestNewVaultInviteeGetsPrivateHomeAccount(t *testing.T) {
	ts := setupTestServer(t)
	ownerToken, owner := ts.registerTestUser(t, "only-vault-owner@example.com")
	vault := ts.createTestVault(t, ownerToken, "Shared vault")
	rec := ts.doRequest(http.MethodPost, fmt.Sprintf("/api/vaults/%s/settings/invitations", vault.ID),
		`{"email":"only-vault-guest@example.com","permission":300}`, ownerToken)
	if rec.Code != http.StatusCreated {
		t.Fatalf("invite failed: %d %s", rec.Code, rec.Body.String())
	}
	var invitation models.Invitation
	if err := ts.db.Where("vault_id = ?", vault.ID).First(&invitation).Error; err != nil {
		t.Fatal(err)
	}
	rec = ts.doRequest(http.MethodPost, "/api/invitations/accept", fmt.Sprintf(`{"token":%q,"first_name":"Guest","password":"secret12345"}`, invitation.Token), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("accept failed: %d %s", rec.Code, rec.Body.String())
	}
	rec = ts.doRequest(http.MethodPost, "/api/auth/login", `{"email":"only-vault-guest@example.com","password":"secret12345"}`, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", rec.Code, rec.Body.String())
	}
	var login authData
	if err := json.Unmarshal(parseResponse(t, rec).Data, &login); err != nil {
		t.Fatal(err)
	}
	if login.User.AccountID == owner.User.AccountID {
		t.Fatal("vault-only guest inherited owner's account")
	}
	if rec = ts.doRequest(http.MethodPost, "/api/auth/switch-account", fmt.Sprintf(`{"account_id":%q}`, owner.User.AccountID), login.Token); rec.Code == http.StatusOK {
		t.Fatal("vault-only guest can enter owner's account")
	}
	if rec = ts.doRequest(http.MethodGet, "/api/vaults/"+vault.ID, "", login.Token); rec.Code != http.StatusOK {
		t.Fatalf("vault inaccessible: %d", rec.Code)
	}
}
