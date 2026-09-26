package handlers

import (
	"errors"

	"github.com/labstack/echo/v5"
	"github.com/naiba/bonds/internal/services"
	"github.com/naiba/bonds/pkg/response"
)

type SetPasswordRequest struct {
	Token    string `json:"token" validate:"required"`
	Password string `json:"password" validate:"required,min=8"`
}

type ConfirmEmailChangeRequest struct {
	Token string `json:"token" validate:"required"`
}

type CredentialActionHandler struct {
	service *services.CredentialActionService
}

func NewCredentialActionHandler(service *services.CredentialActionService) *CredentialActionHandler {
	return &CredentialActionHandler{service: service}
}

// Complete godoc
//
// @Summary Set a password using a one-time email link
// @Tags auth
// @Param request body SetPasswordRequest true "One-time token and new password"
// @Success 204
// @Router /auth/set-password [post]
func (h *CredentialActionHandler) Complete(c *echo.Context) error {
	var req SetPasswordRequest
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, "err.invalid_request_body", nil)
	}
	if err := validateRequest(req); err != nil {
		return response.ValidationError(c, map[string]string{"validation": err.Error()})
	}
	if err := h.service.Complete(req.Token, req.Password); err != nil {
		if errors.Is(err, services.ErrInvalidPasswordAction) {
			return response.BadRequest(c, "err.invalid_or_expired_token", nil)
		}
		return response.InternalError(c, "err.failed_to_update_password")
	}
	return response.NoContent(c)
}

// ConfirmEmailChange godoc
//
// @Summary Confirm an email change using a one-time link (both inboxes required)
// @Tags auth
// @Param request body ConfirmEmailChangeRequest true "One-time token"
// @Success 204
// @Router /auth/confirm-email-change [post]
func (h *CredentialActionHandler) ConfirmEmailChange(c *echo.Context) error {
	var req ConfirmEmailChangeRequest
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, "err.invalid_request_body", nil)
	}
	if err := validateRequest(req); err != nil {
		return response.ValidationError(c, map[string]string{"validation": err.Error()})
	}
	err := h.service.ConfirmEmailChange(req.Token)
	if errors.Is(err, services.ErrInvalidPasswordAction) {
		return response.BadRequest(c, "err.invalid_or_expired_token", nil)
	}
	if errors.Is(err, services.ErrEmailChangeConflict) {
		return response.Conflict(c, "err.email_already_exists")
	}
	if err != nil {
		return response.InternalError(c, "err.failed_to_update_user")
	}
	return response.NoContent(c)
}
