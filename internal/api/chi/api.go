package chi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/nkkko/engram-v3/internal/api/errors"
	"github.com/nkkko/engram-v3/internal/api/models"
	"github.com/nkkko/engram-v3/internal/api/response"
	"github.com/nkkko/engram-v3/internal/api/validation"
	"github.com/nkkko/engram-v3/internal/lockmanager"
	"github.com/nkkko/engram-v3/internal/notifier"
	"github.com/nkkko/engram-v3/internal/search"
	"github.com/nkkko/engram-v3/pkg/proto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Config contains API configuration
type Config struct {
	// Server address
	Addr string

	// Timeouts
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	return Config{
		Addr:         ":8080",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}

// ChiAPI handles HTTP endpoints using Chi router
type ChiAPI struct {
	config      Config
	router      *chi.Mux
	server      *http.Server
	storage     StorageEngine
	eventRouter EventRouter
	notifier    *notifier.Notifier
	lockManager *lockmanager.LockManager
	search      *search.Search
	logger      zerolog.Logger
}

// NewChiAPI creates a new API instance with Chi router
func NewChiAPI(
	config Config,
	storage StorageEngine,
	router EventRouter,
	notifier *notifier.Notifier,
	lockManager *lockmanager.LockManager,
	search *search.Search,
) *ChiAPI {
	logger := log.With().Str("component", "api-chi").Logger()

	if config.Addr == "" {
		config.Addr = DefaultConfig().Addr
	}

	if config.ReadTimeout == 0 {
		config.ReadTimeout = DefaultConfig().ReadTimeout
	}

	if config.WriteTimeout == 0 {
		config.WriteTimeout = DefaultConfig().WriteTimeout
	}

	if config.IdleTimeout == 0 {
		config.IdleTimeout = DefaultConfig().IdleTimeout
	}

	return &ChiAPI{
		config:      config,
		storage:     storage,
		eventRouter: router,
		notifier:    notifier,
		lockManager: lockManager,
		search:      search,
		logger:      logger,
	}
}

// Start initializes and runs the API server
func (a *ChiAPI) Start(ctx context.Context) error {
	a.logger.Info().Str("addr", a.config.Addr).Msg("Starting API server with Chi router")

	// Create Chi router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Register routes
	a.registerRoutes(r)

	// Store router
	a.router = r

	// Create and configure HTTP server
	server := &http.Server{
		Addr:         a.config.Addr,
		Handler:      r,
		ReadTimeout:  a.config.ReadTimeout,
		WriteTimeout: a.config.WriteTimeout,
		IdleTimeout:  a.config.IdleTimeout,
	}
	a.server = server

	// Start server in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.Error().Err(err).Msg("API server error")
		}
	}()

	a.logger.Info().Str("addr", a.config.Addr).Msg("API server started")

	// Wait for context cancellation
	<-ctx.Done()
	return nil
}

// registerRoutes sets up all API endpoints
func (a *ChiAPI) registerRoutes(r chi.Router) {
	// Health checks
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	// Work Unit endpoints
	r.Route("/workunit", func(r chi.Router) {
		r.Post("/", a.handleCreateWorkUnit)
		r.Get("/{id}", a.handleGetWorkUnit)
	})

	r.Get("/workunits/{contextId}", a.handleListWorkUnits)

	// Context endpoints
	r.Route("/contexts", func(r chi.Router) {
		r.Patch("/{id}", a.handleUpdateContext)
		r.Get("/{id}", a.handleGetContext)
	})

	// Lock endpoints
	r.Route("/locks", func(r chi.Router) {
		r.Post("/", a.handleAcquireLock)
		r.Delete("/{resource}", a.handleReleaseLock)
		r.Get("/{resource}", a.handleGetLock)
	})

	// Search endpoint
	r.Post("/search", a.handleSearch)
}

// Helper method for sending JSON responses
func sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// Helper method for sending error responses
func sendError(w http.ResponseWriter, status int, errMsg string) {
	sendJSON(w, status, map[string]string{"error": errMsg})
}

// handleCreateWorkUnit creates a new work unit
func (a *ChiAPI) handleCreateWorkUnit(w http.ResponseWriter, r *http.Request) {
	// Parse and validate request
	var req models.CreateWorkUnitRequest
	if err := validation.ParseAndValidate(r, &req); err != nil {
		a.logger.Debug().Err(err).Msg("Invalid create work unit request")
		response.Error(w, r, err)
		return
	}

	// Convert to proto request
	protoReq := req.ToProto()

	// Create work unit
	workUnit, err := a.storage.CreateWorkUnit(r.Context(), protoReq)
	if err != nil {
		a.logger.Error().Err(err).Msg("Failed to create work unit")
		response.Error(w, r, errors.InternalError("create_work_unit_failed", "Failed to create work unit"))
		return
	}

	// Convert to response model
	responseData := models.WorkUnitFromProto(workUnit)

	// Return response
	response.JSON(w, r, http.StatusCreated, responseData)
}

// handleGetWorkUnit retrieves a work unit by ID
func (a *ChiAPI) handleGetWorkUnit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, r, errors.ValidationError("missing_id", "Work unit ID is required"))
		return
	}

	// Get work unit
	workUnit, err := a.storage.GetWorkUnit(r.Context(), id)
	if err != nil {
		a.logger.Error().Err(err).Str("id", id).Msg("Failed to get work unit")
		response.Error(w, r, errors.NotFoundError("work_unit_not_found", "Work unit not found"))
		return
	}

	// Convert to response model
	responseData := models.WorkUnitFromProto(workUnit)

	// Return response
	response.JSON(w, r, http.StatusOK, responseData)
}

// handleListWorkUnits lists work units for a context
func (a *ChiAPI) handleListWorkUnits(w http.ResponseWriter, r *http.Request) {
	contextID := chi.URLParam(r, "contextId")
	if contextID == "" {
		response.Error(w, r, errors.ValidationError("missing_context_id", "Context ID is required"))
		return
	}

	// Parse pagination parameters
	limitStr := r.URL.Query().Get("limit")
	limit := 100 // Default
	if limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil && val > 0 && val <= 1000 {
			limit = val
		}
	}

	cursor := r.URL.Query().Get("cursor")
	reverse := r.URL.Query().Get("reverse") == "true"

	// List work units
	workUnits, nextCursor, err := a.storage.ListWorkUnits(r.Context(), contextID, limit, cursor, reverse)
	if err != nil {
		a.logger.Error().Err(err).Str("context_id", contextID).Msg("Failed to list work units")
		response.Error(w, r, errors.InternalError("list_work_units_failed", "Failed to list work units"))
		return
	}

	// Convert to response models
	responseData := make([]*models.WorkUnitResponse, 0, len(workUnits))
	for _, unit := range workUnits {
		responseData = append(responseData, models.WorkUnitFromProto(unit))
	}

	// Create pagination metadata
	meta := models.PaginationMeta{
		Limit:      limit,
		NextCursor: nextCursor,
	}

	// Return response with metadata
	response.WithMeta(w, r, http.StatusOK, responseData, meta)
}

// handleUpdateContext updates a context
func (a *ChiAPI) handleUpdateContext(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		sendError(w, http.StatusBadRequest, "Context ID is required")
		return
	}

	// Parse request
	var req proto.UpdateContextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Set ID from path
	req.Id = id

	// Update context
	context, err := a.storage.UpdateContext(r.Context(), &req)
	if err != nil {
		a.logger.Error().Err(err).Str("id", id).Msg("Failed to update context")
		sendError(w, http.StatusInternalServerError, "Failed to update context")
		return
	}

	// Return response
	sendJSON(w, http.StatusOK, map[string]interface{}{
		"context": context,
	})
}

// handleGetContext retrieves a context by ID
func (a *ChiAPI) handleGetContext(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		sendError(w, http.StatusBadRequest, "Context ID is required")
		return
	}

	// Get context
	context, err := a.storage.GetContext(r.Context(), id)
	if err != nil {
		a.logger.Error().Err(err).Str("id", id).Msg("Failed to get context")
		sendError(w, http.StatusNotFound, "Context not found")
		return
	}

	// Return response
	sendJSON(w, http.StatusOK, map[string]interface{}{
		"context": context,
	})
}

// handleAcquireLock acquires a lock on a resource
func (a *ChiAPI) handleAcquireLock(w http.ResponseWriter, r *http.Request) {
	// Parse and validate request
	var req models.AcquireLockRequest
	if err := validation.ParseAndValidate(r, &req); err != nil {
		a.logger.Debug().Err(err).Msg("Invalid acquire lock request")
		response.Error(w, r, err)
		return
	}

	// Convert to proto request
	protoReq := req.ToProto()

	// Acquire lock
	lock, err := a.storage.AcquireLock(r.Context(), protoReq)
	if err != nil {
		a.logger.Debug().Err(err).Str("resource_path", req.ResourcePath).Msg("Failed to acquire lock")
		response.Error(w, r, errors.ConflictError("lock_acquisition_failed", err.Error()))
		return
	}

	// Convert to response model
	responseData := models.LockFromProto(lock)

	// Return response
	response.JSON(w, r, http.StatusOK, responseData)
}

// handleReleaseLock releases a lock on a resource
func (a *ChiAPI) handleReleaseLock(w http.ResponseWriter, r *http.Request) {
	resourcePath := chi.URLParam(r, "resource")
	if resourcePath == "" {
		sendError(w, http.StatusBadRequest, "Resource path is required")
		return
	}

	// Extract agent ID and lock ID from request
	agentID := r.Header.Get("X-Agent-ID")
	if agentID == "" {
		agentID = r.URL.Query().Get("agent_id")
	}

	lockID := r.URL.Query().Get("lock_id")

	// Convert to proto request
	req := &proto.ReleaseLockRequest{
		ResourcePath: resourcePath,
		AgentId:      agentID,
		LockId:       lockID,
	}

	// Release lock
	released, err := a.storage.ReleaseLock(r.Context(), req)
	if err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	if !released {
		sendError(w, http.StatusNotFound, "Lock not found or already released")
		return
	}

	// Return response
	sendJSON(w, http.StatusOK, map[string]interface{}{
		"released": true,
	})
}

// handleGetLock retrieves information about a lock
func (a *ChiAPI) handleGetLock(w http.ResponseWriter, r *http.Request) {
	resourcePath := chi.URLParam(r, "resource")
	if resourcePath == "" {
		sendError(w, http.StatusBadRequest, "Resource path is required")
		return
	}

	// Get lock
	lock, err := a.storage.GetLock(r.Context(), resourcePath)
	if err != nil {
		sendError(w, http.StatusNotFound, "Lock not found or expired")
		return
	}

	// Return response
	sendJSON(w, http.StatusOK, map[string]interface{}{
		"lock": lock,
	})
}

// handleSearch searches for work units
func (a *ChiAPI) handleSearch(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var req struct {
		Query       string            `json:"query"`
		ContextID   string            `json:"context_id,omitempty"`
		AgentIDs    []string          `json:"agent_ids,omitempty"`
		Types       []string          `json:"types,omitempty"`
		MetaFilters map[string]string `json:"meta_filters,omitempty"`
		Limit       int               `json:"limit,omitempty"`
		Offset      int               `json:"offset,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Set default limit if not provided
	if req.Limit <= 0 || req.Limit > 1000 {
		req.Limit = 100
	}

	// Convert types to enum values
	var types []proto.WorkUnitType
	for _, typeStr := range req.Types {
		switch typeStr {
		case "MESSAGE":
			types = append(types, proto.WorkUnitType_MESSAGE)
		case "SYSTEM":
			types = append(types, proto.WorkUnitType_SYSTEM)
		}
	}

	// Perform search
	results, totalCount, err := a.search.Search(r.Context(), req.Query, req.ContextID, req.AgentIDs, types, req.MetaFilters, req.Limit, req.Offset)
	if err != nil {
		a.logger.Error().Err(err).Msg("Search failed")
		sendError(w, http.StatusInternalServerError, "Search failed")
		return
	}

	// Return response
	sendJSON(w, http.StatusOK, map[string]interface{}{
		"results":     results,
		"total_count": totalCount,
		"limit":       req.Limit,
		"offset":      req.Offset,
	})
}

// Shutdown stops the API server
func (a *ChiAPI) Shutdown(ctx context.Context) error {
	a.logger.Info().Msg("Shutting down API server")
	if a.server != nil {
		return a.server.Shutdown(ctx)
	}
	return nil
}
