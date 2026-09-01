package httpapi

import (
	"errors"
	"net/http"

	"github.com/ClairCrest/asr-platform/api/internal/auth"
	"github.com/ClairCrest/asr-platform/api/internal/http/dto"
	"github.com/ClairCrest/asr-platform/api/internal/http/middleware"
)

type AuthHandler struct {
	svc *auth.Service
}

func NewAuthHandler(svc *auth.Service) *AuthHandler {
	return &AuthHandler{svc: svc}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "malformed request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "email and password are required")
		return
	}

	u, err := h.svc.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrEmailTaken) {
			WriteError(w, r, http.StatusConflict, "email_taken", "an account with this email already exists")
			return
		}
		WriteError(w, r, http.StatusInternalServerError, "internal_error", "could not register user")
		return
	}

	pair, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "internal_error", "registered but could not issue tokens")
		return
	}
	_ = u
	WriteJSON(w, http.StatusCreated, dto.TokenResponse{AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "malformed request body")
		return
	}

	pair, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			WriteError(w, r, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
			return
		}
		WriteError(w, r, http.StatusInternalServerError, "internal_error", "could not log in")
		return
	}
	WriteJSON(w, http.StatusOK, dto.TokenResponse{AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req dto.RefreshRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "malformed request body")
		return
	}

	pair, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			WriteError(w, r, http.StatusUnauthorized, "invalid_token", "invalid or expired refresh token")
			return
		}
		WriteError(w, r, http.StatusInternalServerError, "internal_error", "could not refresh token")
		return
	}
	WriteJSON(w, http.StatusOK, dto.TokenResponse{AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		WriteError(w, r, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}

	u, err := h.svc.CurrentUser(r.Context(), userID)
	if err != nil {
		WriteError(w, r, http.StatusNotFound, "user_not_found", "user not found")
		return
	}
	WriteJSON(w, http.StatusOK, dto.UserResponse{ID: u.ID, Email: u.Email, CreatedAt: u.CreatedAt})
}

func (h *AuthHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		WriteError(w, r, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}

	var req dto.CreateAPIKeyRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "malformed request body")
		return
	}
	if req.Name == "" {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "name is required")
		return
	}

	raw, key, err := h.svc.CreateAPIKey(r.Context(), userID, req.Name)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "internal_error", "could not create api key")
		return
	}
	WriteJSON(w, http.StatusCreated, dto.CreateAPIKeyResponse{ID: key.ID, Name: key.Name, Key: raw, CreatedAt: key.CreatedAt})
}

func (h *AuthHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		WriteError(w, r, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}

	keys, err := h.svc.ListAPIKeys(r.Context(), userID)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "internal_error", "could not list api keys")
		return
	}

	out := make([]dto.APIKeyResponse, 0, len(keys))
	for _, k := range keys {
		out = append(out, dto.APIKeyResponse{ID: k.ID, Name: k.Name, LastUsedAt: k.LastUsedAt, CreatedAt: k.CreatedAt})
	}
	WriteJSON(w, http.StatusOK, out)
}

func (h *AuthHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		WriteError(w, r, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}

	keyID, err := parseUUIDParam(r, "id")
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid api key id")
		return
	}

	if err := h.svc.RevokeAPIKey(r.Context(), userID, keyID); err != nil {
		if errors.Is(err, auth.ErrAPIKeyNotFound) {
			WriteError(w, r, http.StatusNotFound, "api_key_not_found", "api key not found")
			return
		}
		WriteError(w, r, http.StatusInternalServerError, "internal_error", "could not revoke api key")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
