package services

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/naiba/bonds/internal/dto"
	"github.com/naiba/bonds/internal/i18n"
	"github.com/naiba/bonds/internal/models"
	userTimezone "github.com/naiba/bonds/internal/timezone"
	"github.com/naiba/bonds/pkg/response"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrInvitationNotFound         = errors.New("invitation not found")
	ErrInvitationExpired          = errors.New("invitation expired")
	ErrUserAlreadyExists          = errors.New("user already exists in this account")
	ErrExistingUserLoginRequired  = errors.New("existing user must log in to accept invitation")
	ErrInvitationIdentityMismatch = errors.New("invitation is addressed to another user")
	ErrInvitationForbidden        = errors.New("cannot invite to this vault")
)

type InvitationService struct {
	db       *gorm.DB
	mailer   Mailer
	appURL   string
	settings *SystemSettingService
}

func NewInvitationService(db *gorm.DB, mailer Mailer, appURL string) *InvitationService {
	return &InvitationService{db: db, mailer: mailer, appURL: appURL}
}

func (s *InvitationService) SetSystemSettings(settings *SystemSettingService) {
	s.settings = settings
}

func (s *InvitationService) getAppURL() string {
	if s.settings != nil {
		return s.settings.GetWithDefault("app.url", s.appURL)
	}
	return s.appURL
}

func (s *InvitationService) Create(accountID, createdBy string, req dto.CreateInvitationRequest) (*dto.InvitationResponse, error) {
	var existing int64
	if err := s.db.Model(&models.AccountMembership{}).
		Joins("JOIN users ON users.id = account_memberships.user_id").
		Where("LOWER(users.email) = LOWER(?) AND account_memberships.account_id = ?", strings.TrimSpace(req.Email), accountID).
		Count(&existing).Error; err != nil {
		return nil, err
	}
	if existing > 0 {
		return nil, ErrUserAlreadyExists
	}
	// Account membership has two roles (administrator/member), independent of
	// vault permissions. All invitations start as members; an administrator may
	// explicitly promote a member after acceptance.
	return s.create(accountID, nil, createdBy, req.Email, models.PermissionViewer)
}

func (s *InvitationService) CreateVault(vaultID, createdBy string, req dto.CreateVaultInvitationRequest) (*dto.InvitationResponse, error) {
	var vault models.Vault
	if err := s.db.Select("account_id").First(&vault, "id = ?", vaultID).Error; err != nil {
		return nil, ErrInvitationForbidden
	}
	var count int64
	if err := s.db.Model(&models.UserVault{}).
		Where("vault_id = ? AND user_id = ? AND permission <= ?", vaultID, createdBy, models.PermissionManager).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, ErrInvitationForbidden
	}
	var existing int64
	if err := s.db.Model(&models.UserVault{}).
		Joins("JOIN users ON users.id = user_vault.user_id").
		Where("user_vault.vault_id = ? AND LOWER(users.email) = LOWER(?)", vaultID, strings.TrimSpace(req.Email)).
		Count(&existing).Error; err != nil {
		return nil, err
	}
	if existing != 0 {
		return nil, ErrUserAlreadyInVault
	}
	return s.create(vault.AccountID, &vaultID, createdBy, req.Email, req.Permission)
}

func (s *InvitationService) create(accountID string, vaultID *string, createdBy, email string, permission int) (*dto.InvitationResponse, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if permission == 0 {
		permission = models.PermissionViewer
	}
	if permission != models.PermissionManager && permission != models.PermissionEditor && permission != models.PermissionViewer {
		return nil, ErrInvitationForbidden
	}
	token := uuid.New().String()
	invitation := models.Invitation{
		AccountID:  accountID,
		VaultID:    vaultID,
		Email:      email,
		Token:      token,
		Permission: permission,
		ExpiresAt:  time.Now().Add(7 * 24 * time.Hour),
		CreatedBy:  createdBy,
	}

	inviteLink := fmt.Sprintf("%s/accept-invite?token=%s", s.getAppURL(), token)
	// Localize on the inviter's locale, not the invitee's: the invitee has no
	// account yet so no stored preference exists. The inviter at least
	// understands the language they themselves chose, and is the one who can
	// re-send a different version if needed.
	locale := i18n.Default
	var creator models.User
	if err := s.db.Select("locale").Where("id = ?", createdBy).First(&creator).Error; err == nil && creator.Locale != "" {
		locale = creator.Locale
	}
	subject := i18n.T(locale, "email.invitation.subject")
	bodyKey := "email.invitation.body"
	if vaultID != nil {
		bodyKey = "email.vault_invitation.body"
	}
	body := i18n.Tt(locale, bodyKey, map[string]string{"link": inviteLink})
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		query := tx.Where("account_id = ? AND LOWER(email) = ? AND accepted_at IS NULL", accountID, email)
		if vaultID == nil {
			query = query.Where("vault_id IS NULL")
		} else {
			query = query.Where("vault_id = ?", *vaultID)
		}
		if err := query.Delete(&models.Invitation{}).Error; err != nil {
			return err
		}
		if err := tx.Create(&invitation).Error; err != nil {
			return err
		}
		// Failure preserves any previous pending invitation and its link.
		return s.mailer.Send(email, subject, body)
	}); err != nil {
		return nil, err
	}

	resp := toInvitationResponse(&invitation)
	return &resp, nil
}

func (s *InvitationService) Accept(req dto.AcceptInvitationRequest, locale string) (*dto.InvitationResponse, error) {
	invitation, err := s.pendingInvitation(req.Token)
	if err != nil {
		return nil, err
	}
	var existing models.User
	err = s.db.Where("LOWER(email) = LOWER(?)", invitation.Email).First(&existing).Error
	if err == nil {
		return nil, ErrExistingUserLoginRequired
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	hashedStr := string(hashedPassword)
	now := time.Now()
	defaultTimezone := userTimezone.Default

	err = s.db.Transaction(func(tx *gorm.DB) error {
		homeAccountID := invitation.AccountID
		if invitation.VaultID != nil {
			// A vault-only invitation must never turn its account into the new
			// user's home account: home-account auth includes account settings.
			home := models.Account{}
			if err := tx.Create(&home).Error; err != nil {
				return err
			}
			homeAccountID = home.ID
		}
		user := models.User{
			AccountID:              homeAccountID,
			FirstName:              strPtrOrNil(req.FirstName),
			LastName:               strPtrOrNil(req.LastName),
			Email:                  invitation.Email,
			Password:               &hashedStr,
			Timezone:               &defaultTimezone,
			Locale:                 locale,
			InvitationCode:         &invitation.Token,
			InvitationAcceptedAt:   &now,
			EmailVerifiedAt:        &now,
			IsAccountAdministrator: invitation.VaultID != nil,
		}
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		if invitation.VaultID != nil {
			if err := tx.Create(&models.AccountMembership{AccountID: homeAccountID, UserID: user.ID, IsAdmin: true}).Error; err != nil {
				return err
			}
			if err := models.SeedAccountDefaults(tx, homeAccountID, user.ID, user.Email, locale); err != nil {
				return err
			}
		} else {
			if err := models.SeedUserNotificationChannel(tx, user.ID, user.Email, locale); err != nil {
				return err
			}
		}

		return s.grantInvitation(tx, invitation, &user, now)
	})
	if err != nil {
		return nil, err
	}

	resp := toInvitationResponse(invitation)
	return &resp, nil
}

func (s *InvitationService) AcceptExisting(token, userID string) (*dto.InvitationResponse, error) {
	var resp dto.InvitationResponse
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var invitation models.Invitation
		if err := tx.Where("token = ? AND accepted_at IS NULL AND expires_at > ?", token, time.Now()).First(&invitation).Error; err != nil {
			return ErrInvitationNotFound
		}
		var user models.User
		if err := tx.First(&user, "id = ?", userID).Error; err != nil {
			return ErrInvitationIdentityMismatch
		}
		if user.EmailVerifiedAt == nil || !strings.EqualFold(user.Email, invitation.Email) {
			return ErrInvitationIdentityMismatch
		}
		if err := s.grantInvitation(tx, &invitation, &user, time.Now()); err != nil {
			return err
		}
		resp = toInvitationResponse(&invitation)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *InvitationService) pendingInvitation(token string) (*models.Invitation, error) {
	var invitation models.Invitation
	if err := s.db.Where("token = ?", token).First(&invitation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvitationNotFound
		}
		return nil, err
	}
	if invitation.AcceptedAt != nil {
		return nil, ErrInvitationNotFound
	}
	if time.Now().After(invitation.ExpiresAt) {
		return nil, ErrInvitationExpired
	}
	return &invitation, nil
}

func (s *InvitationService) grantInvitation(tx *gorm.DB, invitation *models.Invitation, user *models.User, now time.Time) error {
	claim := tx.Model(&models.Invitation{}).
		Where("id = ? AND accepted_at IS NULL AND expires_at > ?", invitation.ID, now).
		Update("accepted_at", now)
	if claim.Error != nil {
		return claim.Error
	}
	if claim.RowsAffected != 1 {
		return ErrInvitationNotFound
	}
	if invitation.VaultID != nil {
		var vault models.Vault
		if err := tx.Select("account_id").First(&vault, "id = ?", *invitation.VaultID).Error; err != nil || vault.AccountID != invitation.AccountID {
			return ErrInvitationNotFound
		}
		var count int64
		if err := tx.Model(&models.UserVault{}).Where("vault_id = ? AND user_id = ?", *invitation.VaultID, user.ID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			if err := tx.Create(&models.UserVault{UserID: user.ID, VaultID: *invitation.VaultID, Permission: invitation.Permission}).Error; err != nil {
				return err
			}
			if err := scheduleAllVaultUserRemindersForNewMember(tx, *invitation.VaultID, user.ID, now); err != nil {
				return err
			}
		}
	} else {
		membership := models.AccountMembership{AccountID: invitation.AccountID, UserID: user.ID}
		if err := tx.Where("account_id = ? AND user_id = ?", invitation.AccountID, user.ID).
			FirstOrCreate(&membership).Error; err != nil {
			return err
		}
	}
	invitation.AcceptedAt = &now
	return nil
}

func (s *InvitationService) List(accountID string, page, perPage int) ([]dto.InvitationResponse, response.Meta, error) {
	query := s.db.Where("account_id = ? AND vault_id IS NULL", accountID)
	return s.list(query, page, perPage)
}

func (s *InvitationService) ListVault(vaultID string, page, perPage int) ([]dto.InvitationResponse, response.Meta, error) {
	return s.list(s.db.Where("vault_id = ?", vaultID), page, perPage)
}

func (s *InvitationService) list(query *gorm.DB, page, perPage int) ([]dto.InvitationResponse, response.Meta, error) {

	var total int64
	if err := query.Model(&models.Invitation{}).Count(&total).Error; err != nil {
		return nil, response.Meta{}, err
	}

	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	var invitations []models.Invitation
	if err := query.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&invitations).Error; err != nil {
		return nil, response.Meta{}, err
	}
	result := make([]dto.InvitationResponse, len(invitations))
	for i, inv := range invitations {
		result[i] = toInvitationResponse(&inv)
	}

	meta := response.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: int(math.Ceil(float64(total) / float64(perPage))),
	}
	return result, meta, nil
}

func (s *InvitationService) Delete(id uint, accountID string) error {
	var invitation models.Invitation
	if err := s.db.Where("id = ? AND account_id = ? AND vault_id IS NULL", id, accountID).First(&invitation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvitationNotFound
		}
		return err
	}
	if invitation.AcceptedAt != nil {
		return ErrInvitationNotFound
	}
	return s.db.Delete(&invitation).Error
}

func (s *InvitationService) DeleteVault(id uint, vaultID string) error {
	var invitation models.Invitation
	if err := s.db.Where("id = ? AND vault_id = ? AND accepted_at IS NULL", id, vaultID).First(&invitation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvitationNotFound
		}
		return err
	}
	return s.db.Delete(&invitation).Error
}

func (s *InvitationService) Preview(token string) (*dto.InvitationResponse, error) {
	invitation, err := s.pendingInvitation(token)
	if err != nil {
		return nil, err
	}
	resp := toInvitationResponse(invitation)
	return &resp, nil
}

func toInvitationResponse(inv *models.Invitation) dto.InvitationResponse {
	return dto.InvitationResponse{
		ID:         inv.ID,
		Email:      inv.Email,
		VaultID:    inv.VaultID,
		Permission: inv.Permission,
		ExpiresAt:  inv.ExpiresAt,
		AcceptedAt: inv.AcceptedAt,
		CreatedAt:  inv.CreatedAt,
	}
}
