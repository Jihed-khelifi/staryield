package handler

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/service"
)

// SessionResponse is returned by sign-in endpoints.
type SessionResponse struct {
	User            *service.UserView `json:"user"`
	CSRFToken       string            `json:"csrf_token"`
	AccessExpiresAt time.Time         `json:"access_expires_at"`
}

func (h *Handler) session(c echo.Context, res *service.AuthResult, status int) error {
	h.setSession(c, res)
	csrf := h.issueCSRF(c)
	me, err := h.svc.Users.Me(c.Request().Context(), res.UserID)
	if err != nil {
		return err
	}
	return c.JSON(status, SessionResponse{User: me, CSRFToken: csrf, AccessExpiresAt: res.AccessExpires})
}

// CSRFToken issues a CSRF token (call before the first unsafe request).
func (h *Handler) CSRFToken(c echo.Context) error {
	return ok(c, map[string]string{"csrf_token": h.issueCSRF(c)})
}

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	AcceptTerms bool   `json:"accept_terms"`
	BirthDate   string `json:"birth_date"`
	Locale      string `json:"locale"`
}

// Register creates a client account (FR-02).
func (h *Handler) Register(c echo.Context) error {
	var req registerRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	res, err := h.svc.Auth.Register(c.Request().Context(), service.RegisterInput{
		Email: req.Email, Password: req.Password, DisplayName: req.DisplayName,
		AcceptTerms: req.AcceptTerms, BirthDate: req.BirthDate, Locale: req.Locale,
	})
	if err != nil {
		return err
	}
	return h.session(c, res, http.StatusCreated)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login signs a user in (FR-02).
func (h *Handler) Login(c echo.Context) error {
	var req loginRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	res, err := h.svc.Auth.Login(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		return err
	}
	return h.session(c, res, http.StatusOK)
}

// Refresh rotates the refresh token cookie.
func (h *Handler) Refresh(c echo.Context) error {
	res, err := h.svc.Auth.Refresh(c.Request().Context(), refreshCookie(c, h.cookies.Names.Refresh))
	if err != nil {
		if appErr, isApp := apperror.As(err); isApp && appErr.Status == http.StatusUnauthorized {
			h.clearSession(c)
		}
		return err
	}
	h.setSession(c, res)
	return ok(c, map[string]any{"access_expires_at": res.AccessExpires, "csrf_token": h.issueCSRF(c)})
}

// Logout ends this device's session.
func (h *Handler) Logout(c echo.Context) error {
	if err := h.svc.Auth.Logout(c.Request().Context(), refreshCookie(c, h.cookies.Names.Refresh)); err != nil {
		return err
	}
	h.clearSession(c)
	return noContent(c)
}

// LogoutAll ends every session of the user.
func (h *Handler) LogoutAll(c echo.Context) error {
	if err := h.svc.Auth.LogoutAll(c.Request().Context(), principal(c).UserID); err != nil {
		return err
	}
	h.clearSession(c)
	return noContent(c)
}

type forgotRequest struct {
	Email  string `json:"email"`
	Locale string `json:"locale"`
}

// ForgotPassword always answers 202 so accounts cannot be enumerated.
func (h *Handler) ForgotPassword(c echo.Context) error {
	var req forgotRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	if err := h.svc.Auth.ForgotPassword(c.Request().Context(), req.Email, req.Locale); err != nil {
		return err
	}
	return c.JSON(http.StatusAccepted, map[string]string{"status": "If an account exists for this email, a reset link is on its way."})
}

type resetRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// ResetPassword sets a new password from an e-mailed token.
func (h *Handler) ResetPassword(c echo.Context) error {
	var req resetRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	if err := h.svc.Auth.ResetPassword(c.Request().Context(), req.Token, req.Password); err != nil {
		return err
	}
	h.clearSession(c)
	return noContent(c)
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ChangePassword updates the password and re-issues this session.
func (h *Handler) ChangePassword(c echo.Context) error {
	var req changePasswordRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	res, err := h.svc.Auth.ChangePassword(c.Request().Context(), principal(c).UserID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		return err
	}
	return h.session(c, res, http.StatusOK)
}

// Me returns the signed-in user.
func (h *Handler) Me(c echo.Context) error {
	me, err := h.svc.Users.Me(c.Request().Context(), principal(c).UserID)
	if err != nil {
		return err
	}
	return ok(c, me)
}

// UpdateProfile edits the profile (FR-08).
func (h *Handler) UpdateProfile(c echo.Context) error {
	var req service.ProfileUpdate
	if err := bind(c, &req); err != nil {
		return err
	}
	me, err := h.svc.Users.UpdateProfile(c.Request().Context(), principal(c).UserID, req)
	if err != nil {
		return err
	}
	return ok(c, me)
}

// UploadAvatar accepts a multipart "avatar" file.
func (h *Handler) UploadAvatar(c echo.Context) error {
	r := c.Request()
	r.Body = http.MaxBytesReader(c.Response(), r.Body, h.maxAvatarBytes+64*1024)
	if err := r.ParseMultipartForm(h.maxAvatarBytes + 64*1024); err != nil {
		return decodeMultipart(err)
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	file, header, err := r.FormFile("avatar")
	if err != nil {
		return apperror.Validation(map[string]string{"avatar": "Choose an image to upload."})
	}
	defer file.Close()
	if header.Size > h.maxAvatarBytes {
		return apperror.PayloadTooLarge("Choose an image smaller than 1 MB.")
	}
	data, err := io.ReadAll(io.LimitReader(file, h.maxAvatarBytes+1))
	if err != nil {
		return apperror.Validation(map[string]string{"avatar": "The file could not be read."})
	}
	me, err := h.svc.Users.UploadAvatar(r.Context(), principal(c).UserID, data)
	if err != nil {
		return err
	}
	return ok(c, me)
}

func decodeMultipart(err error) error {
	var maxBytes *http.MaxBytesError
	if errors.As(err, &maxBytes) {
		return apperror.PayloadTooLarge("Choose an image smaller than 1 MB.")
	}
	return apperror.BadRequest("Upload the image as multipart/form-data.")
}

// DeleteAvatar removes the profile picture.
func (h *Handler) DeleteAvatar(c echo.Context) error {
	me, err := h.svc.Users.DeleteAvatar(c.Request().Context(), principal(c).UserID)
	if err != nil {
		return err
	}
	return ok(c, me)
}
