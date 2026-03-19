package presentation

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jvlerner/auth-system/internal/identity/application"
	"github.com/jvlerner/auth-system/internal/identity/domain"
	"go.uber.org/zap"
)

type AuthHandler struct {
	registerUC       *application.RegisterUserUseCase
	loginUC          *application.LoginUserUseCase
	rolesUC          *application.UpdateUserRolesUseCase
	confirmEmailUC   *application.ConfirmEmailUseCase
	forgotPasswordUC *application.ForgotPasswordUseCase
	resetPasswordUC  *application.ResetPasswordUseCase
	refreshUC        *application.RefreshTokenUseCase
	logoutUC         *application.LogoutUseCase
	setupMFAUC       *application.SetupMFAUseCase
	verifyMFAUC      *application.VerifyMFAUseCase
	logger           *zap.Logger
}

func NewAuthHandler(
	registerUC *application.RegisterUserUseCase, 
	loginUC *application.LoginUserUseCase, 
	rolesUC *application.UpdateUserRolesUseCase, 
	confirmEmailUC *application.ConfirmEmailUseCase,
	forgotPasswordUC *application.ForgotPasswordUseCase,
	resetPasswordUC *application.ResetPasswordUseCase,
	refreshUC *application.RefreshTokenUseCase,
	logoutUC *application.LogoutUseCase,
	setupMFAUC *application.SetupMFAUseCase,
	verifyMFAUC *application.VerifyMFAUseCase,
	logger *zap.Logger,
) *AuthHandler {
	return &AuthHandler{
		registerUC:       registerUC,
		loginUC:          loginUC,
		rolesUC:          rolesUC,
		confirmEmailUC:   confirmEmailUC,
		forgotPasswordUC: forgotPasswordUC,
		resetPasswordUC:  resetPasswordUC,
		refreshUC:        refreshUC,
		logoutUC:         logoutUC,
		setupMFAUC:       setupMFAUC,
		verifyMFAUC:      verifyMFAUC,
		logger:           logger,
	}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var cmd application.RegisterUserCommand

	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		h.logger.Warn("Failed to decode register request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := h.registerUC.Execute(r.Context(), cmd)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidEmail) || errors.Is(err, domain.ErrUserAlreadyExists) {
			h.logger.Info("Domain validation failed on register", zap.Error(err), zap.String("email", cmd.Email))
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		h.logger.Error("Internal server error during registration", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"message": "User registered successfully"}`))
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req application.LoginRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("Failed to decode login request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	res, err := h.loginUC.Execute(r.Context(), req)
	if err != nil {
		if errors.Is(err, application.ErrInvalidLogin) {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		h.logger.Error("Internal server error during login", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if res.MFARequired {
		w.WriteHeader(http.StatusAccepted) // 202 Accepted ou 206 Partial Content — 202 é comum para fluxos pendentes
	} else {
		w.WriteHeader(http.StatusOK)
	}
	json.NewEncoder(w).Encode(res)
}

func (h *AuthHandler) SetupMFA(w http.ResponseWriter, r *http.Request) {
	// Extrair userID do context (injetado pelo middleware de JWT)
	userID := r.Context().Value("user_id").(string)

	res, err := h.setupMFAUC.Execute(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to setup MFA", zap.Error(err), zap.String("user_id", userID))
		http.Error(w, "Failed to setup MFA", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func (h *AuthHandler) VerifyMFA(w http.ResponseWriter, r *http.Request) {
	var req application.VerifyMFARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	res, err := h.verifyMFAUC.Execute(r.Context(), req)
	if err != nil {
		if errors.Is(err, application.ErrInvalidTOTPCode) || errors.Is(err, application.ErrInvalidMFAToken) {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		h.logger.Error("Error during MFA verification", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func (h *AuthHandler) UpdateRoles(w http.ResponseWriter, r *http.Request) {
	userId := chi.URLParam(r, "id")
	if userId == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	var payload struct {
		Roles []string `json:"roles"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.logger.Warn("Failed to decode update roles request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	cmd := application.UpdateUserRolesCommand{
		UserID: userId,
		Roles:  payload.Roles,
	}

	err := h.rolesUC.Execute(r.Context(), cmd)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		h.logger.Error("Internal server error during update roles", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"message": "Role update command accepted"}`))
}

func (h *AuthHandler) ConfirmEmail(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	if token == "" {
		http.Error(w, "Token is required", http.StatusBadRequest)
		return
	}

	cmd := application.ConfirmEmailCommand{Token: token}
	err := h.confirmEmailUC.Execute(r.Context(), cmd)
	if err != nil {
		if errors.Is(err, application.ErrInvalidVerificationToken) || errors.Is(err, domain.ErrUserNotFound) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.logger.Error("Internal server error during email confirmation", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Email confirmed successfully"}`))
}

func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var cmd application.ForgotPasswordCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		h.logger.Warn("Failed to decode forgot password request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := h.forgotPasswordUC.Execute(r.Context(), cmd)
	if err != nil {
		h.logger.Error("Internal server error during forgot password", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"message": "If the email is valid, a reset link will be sent"}`))
}

func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var cmd application.ResetPasswordCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		h.logger.Warn("Failed to decode reset password request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := h.resetPasswordUC.Execute(r.Context(), cmd)
	if err != nil {
		if errors.Is(err, application.ErrInvalidResetToken) || errors.Is(err, domain.ErrUserNotFound) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.logger.Error("Internal server error during reset password", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Password reset successfully"}`))
}

func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var cmd application.RefreshTokenCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		h.logger.Warn("Failed to decode refresh token request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	res, err := h.refreshUC.Execute(r.Context(), cmd)
	if err != nil {
		if errors.Is(err, application.ErrInvalidRefreshToken) || errors.Is(err, domain.ErrUserNotFound) {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		h.logger.Error("Internal server error during refresh token", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var cmd application.LogoutCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		h.logger.Warn("Failed to decode logout request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := h.logoutUC.Execute(r.Context(), cmd)
	if err != nil {
		h.logger.Error("Internal server error during logout", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
