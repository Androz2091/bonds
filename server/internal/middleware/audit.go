package middleware

import (
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/naiba/bonds/internal/models"
	"gorm.io/gorm"
)

// Audit stores route templates, never raw paths: invitation tokens, search
// queries and user-supplied text must not leak into operator-visible logs.
func Audit(db *gorm.DB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			// Never persist or print caller-controlled request IDs (including control characters).
			requestID := uuid.NewString()
			c.Response().Header().Set("X-Request-ID", requestID)
			err := next(c)
			if !strings.HasPrefix(c.Path(), "/api/") {
				return err
			}
			status := http.StatusOK
			if resp, unwrapErr := echo.UnwrapResponse(c.Response()); unwrapErr == nil && resp.Status != 0 {
				status = resp.Status
			} else if err != nil {
				status = http.StatusInternalServerError
			}
			method := c.Request().Method
			privateRoute := !strings.HasPrefix(c.Path(), "/api/admin/") && !strings.HasPrefix(c.Path(), "/api/auth/")
			vaultOperation := strings.HasPrefix(c.Path(), "/api/vaults/:vault_id/") &&
				GetVaultAccountID(c) != "" && method != http.MethodGet && status < http.StatusInternalServerError
			if privateRoute && !vaultOperation && status < http.StatusInternalServerError {
				return err
			}
			if method == http.MethodGet && status < http.StatusInternalServerError {
				return err
			}
			var actor *string
			if !privateRoute || vaultOperation {
				actor = optionalAuditID(GetUserID(c))
			}
			var vaultID *string
			if vaultOperation {
				vaultID = optionalAuditID(c.Param("vault_id"))
			}
			record := models.AuditEvent{
				ActorUserID: actor, VaultID: vaultID,
				Method: method, Route: c.Path(), Status: status, RequestID: requestID,
			}
			if writeErr := db.Create(&record).Error; writeErr != nil {
				log.Printf("[audit] failed to persist event request_id=%s: %v", requestID, writeErr)
			}
			if !privateRoute && status < http.StatusInternalServerError {
				log.Printf("[audit] status=%d method=%s route=%s request_id=%s", status, method, c.Path(), requestID)
			}
			if status >= http.StatusInternalServerError {
				log.Printf("[api] error status=%d method=%s route=%s request_id=%s", status, method, c.Path(), requestID)
			}
			return err
		}
	}
}

func optionalAuditID(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
