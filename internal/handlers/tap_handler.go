package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/neartap/server/internal/middleware"
	"github.com/neartap/server/internal/models"
	"github.com/neartap/server/internal/store"
)

// TapHandler groups all HTTP handlers for the /api/taps resource
type TapHandler struct {
	store *store.Store
}

// NewTapHandler creates a new TapHandler
func NewTapHandler(s *store.Store) *TapHandler {
	return &TapHandler{store: s}
}

// ── Route registration ─────────────────────────────────────────────────────────

// Routes returns a chi.Router with all tap sub-routes mounted
func (h *TapHandler) Routes(authMiddleware *middleware.FirebaseAuth) http.Handler {
	r := chi.NewRouter()

	// Public routes
	r.Get("/", h.ListTaps)             // GET  /api/taps?lat=&lng=&radius=
	r.Get("/nearby", h.GetNearby)      // GET  /api/taps/nearby?lat=&lng=&radius=
	r.Get("/{id}", h.GetTap)           // GET  /api/taps/:id

	// Protected routes (require Firebase Auth)
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)
		r.Post("/", h.CreateTap)               // POST   /api/taps
		r.Post("/{id}/confirm", h.ConfirmTap)  // POST   /api/taps/:id/confirm
		r.Post("/{id}/report", h.ReportIssue)  // POST   /api/taps/:id/report
		r.Put("/{id}", h.UpdateTap)            // PUT    /api/taps/:id
		r.Delete("/{id}", h.DeleteTap)         // DELETE /api/taps/:id
		r.Post("/{id}/resolve", h.ResolveIssues) // POST   /api/taps/:id/resolve
	})

	return r
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// ListTaps godoc
// GET /api/taps
// Returns all taps, optionally filtered by proximity (lat, lng, radius in km)
func (h *TapHandler) ListTaps(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	lat, lng, radius, hasCoords := parseCoords(r)
	if hasCoords {
		taps, err := h.store.GetNearby(ctx, lat, lng, radius)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		respond(w, http.StatusOK, models.APIResponse{Success: true, Data: taps})
		return
	}

	taps, err := h.store.GetAll(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond(w, http.StatusOK, models.APIResponse{Success: true, Data: taps})
}

// GetNearby godoc
// GET /api/taps/nearby?lat=28.61&lng=77.20&radius=5
// Returns taps within radiusKm sorted by distance
func (h *TapHandler) GetNearby(w http.ResponseWriter, r *http.Request) {
	lat, lng, radius, ok := parseCoords(r)
	if !ok {
		respondError(w, http.StatusBadRequest, "lat, lng, and radius query params are required")
		return
	}

	taps, err := h.store.GetNearby(r.Context(), lat, lng, radius)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond(w, http.StatusOK, models.APIResponse{Success: true, Data: taps})
}

// GetTap godoc
// GET /api/taps/:id
func (h *TapHandler) GetTap(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	tap, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "tap not found")
		return
	}
	respond(w, http.StatusOK, models.APIResponse{Success: true, Data: tap})
}

// CreateTap godoc
// POST /api/taps (requires auth)
// Body: CreateTapRequest JSON
func (h *TapHandler) CreateTap(w http.ResponseWriter, r *http.Request) {
	var req models.CreateTapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Address == "" {
		respondError(w, http.StatusBadRequest, "name and address are required")
		return
	}

	// Build addedBy from the auth token
	addedBy := models.UserRef{UID: "anonymous", Name: "Anonymous"}
	if token := middleware.UserFromContext(r.Context()); token != nil {
		addedBy.UID = token.UID
		if name, ok := token.Claims["name"].(string); ok {
			addedBy.Name = name
		}
		if photo, ok := token.Claims["picture"].(string); ok {
			addedBy.PhotoURL = photo
		}
	}

	tap, err := h.store.Create(r.Context(), req, addedBy)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond(w, http.StatusCreated, models.APIResponse{
		Success: true,
		Data:    tap,
		Message: "Tap added successfully",
	})
}

// ConfirmTap godoc
// POST /api/taps/:id/confirm (requires auth)
// Increments confirmation count; auto-verifies at 3+
func (h *TapHandler) ConfirmTap(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := ""
	if token := middleware.UserFromContext(r.Context()); token != nil {
		userID = token.UID
	} else {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	tap, err := h.store.Confirm(r.Context(), id, userID)
	if err != nil {
		if err.Error() == "you have already confirmed this tap" {
			respondError(w, http.StatusBadRequest, err.Error())
		} else {
			respondError(w, http.StatusNotFound, err.Error())
		}
		return
	}
	msg := "Confirmation recorded"
	if tap.IsVerified {
		msg = "Tap is now Verified! 🎉"
	}
	respond(w, http.StatusOK, models.APIResponse{Success: true, Data: tap, Message: msg})
}

// ReportIssue godoc
// POST /api/taps/:id/report (requires auth)
// Body: { "issue": "description" }
func (h *TapHandler) ReportIssue(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req models.ReportIssueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Issue == "" {
		respondError(w, http.StatusBadRequest, "issue description is required")
		return
	}

	tap, err := h.store.ReportIssue(r.Context(), id, req.Issue)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	respond(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    tap,
		Message: "Issue reported. Thank you!",
	})
}

// UpdateTap godoc
// PUT /api/taps/:id (requires auth)
func (h *TapHandler) UpdateTap(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req models.CreateTapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tap, err := h.store.Update(r.Context(), id, req)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	respond(w, http.StatusOK, models.APIResponse{Success: true, Data: tap, Message: "Tap updated successfully"})
}

// DeleteTap godoc
// DELETE /api/taps/:id (requires auth)
func (h *TapHandler) DeleteTap(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := h.store.Delete(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	respond(w, http.StatusOK, models.APIResponse{Success: true, Message: "Tap deleted successfully"})
}

// ResolveIssues godoc
// POST /api/taps/:id/resolve (requires auth)
func (h *TapHandler) ResolveIssues(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	tap, err := h.store.ResolveIssues(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	respond(w, http.StatusOK, models.APIResponse{Success: true, Data: tap, Message: "Issues resolved successfully"})
}


// ── Helpers ───────────────────────────────────────────────────────────────────

func respond(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respond(w, status, models.APIResponse{Success: false, Error: msg})
}

func parseCoords(r *http.Request) (lat, lng, radius float64, ok bool) {
	q := r.URL.Query()
	latStr := q.Get("lat")
	lngStr := q.Get("lng")
	radiusStr := q.Get("radius")
	if latStr == "" || lngStr == "" {
		return 0, 0, 0, false
	}
	lat, err1 := strconv.ParseFloat(latStr, 64)
	lng, err2 := strconv.ParseFloat(lngStr, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, 0, false
	}
	radius = 10.0 // default 10 km
	if radiusStr != "" {
		if r, err := strconv.ParseFloat(radiusStr, 64); err == nil {
			radius = r
		}
	}
	return lat, lng, radius, true
}
