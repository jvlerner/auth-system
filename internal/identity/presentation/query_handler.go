package presentation

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jvlerner/auth-system/internal/identity/application"
	"github.com/jvlerner/auth-system/internal/identity/domain"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type QueryHandler struct {
	profileUC *application.GetUserProfileUseCase
	logger    *zap.Logger
}

func NewQueryHandler(profileUC *application.GetUserProfileUseCase, logger *zap.Logger) *QueryHandler {
	return &QueryHandler{
		profileUC: profileUC,
		logger:    logger,
	}
}

func (h *QueryHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Missing user id parameter", http.StatusBadRequest)
		return
	}

	query := application.GetUserProfileQuery{ID: id}
	
	dto, err := h.profileUC.Execute(r.Context(), query)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		h.logger.Error("Internal server error querying profile", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(dto)
}
