package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"rtagent/internal/domain/persistence"
	"rtagent/internal/infrastructure/persistence/sqlite/adapters"
	"rtagent/internal/runtime/events"
	"rtagent/internal/startup"
)

type Server struct {
	container *startup.RuntimeContainer
	mux       *http.ServeMux
	authToken string
}

func NewServer(container *startup.RuntimeContainer, authToken string) *Server {
	s := &Server{
		container: container,
		mux:       http.NewServeMux(),
		authToken: authToken,
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Enable Bearer Auth headers check
	authHeader := r.Header.Get("Authorization")
	if s.authToken != "" {
		expected := "Bearer " + s.authToken
		if authHeader != expected {
			s.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid authorization token")
			return
		}
	}

	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/v2/run", s.handleRun)
	s.mux.HandleFunc("/v2/runs", s.handleListRuns)
	s.mux.HandleFunc("/v2/resume", s.handleResume)
	s.mux.HandleFunc("/v2/runtime/events", s.handleEvents)
	s.mux.HandleFunc("/v2/world-state", s.handleWorldState)
	s.mux.HandleFunc("/v2/context/handles", s.handleContextHandles)
	s.mux.HandleFunc("/v2/context/materialize", s.handleMaterialize)
	s.mux.HandleFunc("/v2/approval", s.handleApproval)
	s.mux.HandleFunc("/v2/leases/acquire", s.handleLeaseAcquire)
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	var req struct {
		Objective   string `json:"objective"`
		IngressKind string `json:"ingress_kind"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON payload")
		return
	}

	runID := fmt.Sprintf("run_api_%d", time.Now().UnixNano())
	rec := persistence.RunRecord{
		RunID:         runID,
		ResumeID:      fmt.Sprintf("res_api_%d", time.Now().UnixNano()),
		UserObjective: req.Objective,
		IngressKind:   req.IngressKind,
		Status:        "running",
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}

	if err := s.container.Store.PutRun(r.Context(), rec); err != nil {
		s.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Publish KindRunCreated event
	payload := map[string]interface{}{
		"objective": rec.UserObjective,
		"ingress":   rec.IngressKind,
	}
	payloadBytes, _ := json.Marshal(payload)

	ev := events.Event{
		ID:         fmt.Sprintf("%s:000001", runID),
		RunID:      runID,
		Kind:       events.KindRunCreated,
		Sequence:   1,
		OccurredAt: time.Now().UTC(),
		Message:    "Run initialized via REST API",
		Payload:    payload,
	}

	eventRecord := persistence.RuntimeEventRecord{
		EventID:     ev.ID,
		RunID:       ev.RunID,
		Kind:        string(ev.Kind),
		Sequence:    ev.Sequence,
		OccurredAt:  ev.OccurredAt.Format(time.RFC3339),
		Message:     ev.Message,
		PayloadJSON: payloadBytes,
	}

	_ = s.container.Store.AppendRuntimeEvent(r.Context(), eventRecord)
	_ = s.container.EventBus.Publish(r.Context(), ev)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"run_id":     rec.RunID,
		"resume_id":  rec.ResumeID,
		"status":     rec.Status,
		"created_at": rec.CreatedAt,
	})
}

func (s *Server) handleResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	var req struct {
		RunID        string `json:"run_id"`
		CheckpointID string `json:"checkpoint_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON payload")
		return
	}

	checkpoint, err := s.container.Store.GetCheckpoint(r.Context(), req.RunID, req.CheckpointID)
	if err != nil || checkpoint.CheckpointID == "" {
		s.writeError(w, http.StatusNotFound, "CHECKPOINT_NOT_FOUND", fmt.Sprintf("The specified checkpoint_id '%s' does not exist.", req.CheckpointID))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"run_id":      req.RunID,
		"status":      "running",
		"active_node": checkpoint.Node,
	})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	runID := r.URL.Query().Get("run_id")
	if runID == "" {
		s.writeError(w, http.StatusBadRequest, "MISSING_PARAM", "Query param 'run_id' is required")
		return
	}

	rawSeq := r.URL.Query().Get("after_seq")
	var afterSeq int64 = 0
	if rawSeq != "" {
		if parsed, err := strconv.ParseInt(rawSeq, 10, 64); err == nil {
			afterSeq = parsed
		}
	}

	records, err := s.container.Store.ListRuntimeEvents(r.Context(), runID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	var eventsList []map[string]interface{}
	var maxSeq int64 = 0

	for _, rec := range records {
		if rec.Sequence > maxSeq {
			maxSeq = rec.Sequence
		}
		if rec.Sequence > afterSeq {
			var payload map[string]interface{}
			_ = json.Unmarshal(rec.PayloadJSON, &payload)
			eventsList = append(eventsList, map[string]interface{}{
				"event_id":    rec.EventID,
				"kind":        rec.Kind,
				"sequence":    rec.Sequence,
				"occurred_at": rec.OccurredAt,
				"message":     rec.Message,
				"payload":     payload,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"run_id":       runID,
		"max_sequence": maxSeq,
		"events":       eventsList,
	})
}

func (s *Server) handleWorldState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	runID := r.URL.Query().Get("run_id")
	if runID == "" {
		s.writeError(w, http.StatusBadRequest, "MISSING_PARAM", "Query param 'run_id' is required")
		return
	}

	partitionFilter := r.URL.Query().Get("partition")

	snapshot, err := s.container.WSBuilder.GetLatestSnapshot(r.Context(), runID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	var filtered []map[string]interface{}
	for _, entry := range snapshot {
		if partitionFilter == "" || entry.Partition == partitionFilter {
			filtered = append(filtered, map[string]interface{}{
				"id":            entry.ID,
				"partition":     entry.Partition,
				"kind":          entry.Kind,
				"subject":       entry.Subject,
				"state_json":    entry.StateJSON,
				"summary":       entry.Summary,
				"source_id":     entry.SourceID,
				"source_seq":    entry.SourceSeq,
				"confidence":    entry.Confidence,
				"evidence_refs": entry.EvidenceRefs,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"snapshot_id":  fmt.Sprintf("ws_%s_seq_%d", runID, time.Now().Unix()),
		"version":      1,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"entries":      filtered,
	})
}

func (s *Server) handleContextHandles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	runID := r.URL.Query().Get("run_id")
	if runID == "" {
		s.writeError(w, http.StatusBadRequest, "MISSING_PARAM", "Query param 'run_id' is required")
		return
	}

	handles, err := s.container.ContextRegistry.ListByRunID(r.Context(), runID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"handles": handles,
	})
}

func (s *Server) handleMaterialize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	var req struct {
		RunID    string `json:"run_id"`
		HandleID string `json:"handle_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON payload")
		return
	}

	content, err := s.container.Materializer.Materialize(r.Context(), req.HandleID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"handle_id":  req.HandleID,
		"content":    content,
		"token_size": len(content) / 4, // Simple approximation
	})
}

func (s *Server) handleApproval(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	var req struct {
		RunID        string `json:"run_id"`
		PermissionID string `json:"permission_id"`
		Decision     string `json:"decision"`
		Reason       string `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON payload")
		return
	}

	permRec, err := s.container.Store.GetPermission(r.Context(), req.PermissionID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "NOT_FOUND", "Permission record not found")
		return
	}

	granted := false
	if strings.ToLower(req.Decision) == "approve" {
		granted = true
	}

	permRec.Granted = granted
	permRec.AuthorizedBy = "http_api"
	permRec.ResolvedAt = time.Now().UTC().Format(time.RFC3339)
	permRec.PolicyWarnings = req.Reason

	if err := s.container.Store.PutPermission(r.Context(), permRec); err != nil {
		s.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"permission_id": permRec.PermissionID,
		"status":        permRec.Granted,
		"resolved_at":   permRec.ResolvedAt,
	})
}

func (s *Server) handleLeaseAcquire(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	var req struct {
		Resource         string `json:"resource"`
		HolderActivityID string `json:"holder_activity_id"`
		TTLSeconds       int    `json:"ttl_seconds"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON payload")
		return
	}

	ttl := time.Duration(req.TTLSeconds) * time.Second
	leaseID, err := s.container.LeaseManager.Acquire(r.Context(), req.Resource, req.HolderActivityID, ttl)
	if err != nil {
		// Return 409 Conflict on lease conflict
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error_code": "LEASE_CONFLICT",
			"message":    err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"lease_id":   leaseID,
		"resource":   req.Resource,
		"expires_at": time.Now().UTC().Add(ttl).Format(time.RFC3339),
	})
}

func (s *Server) writeError(w http.ResponseWriter, status int, code string, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error_code": code,
		"message":    msg,
	})
}

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	var models []adapters.RunModel
	err := s.container.DB.WithContext(r.Context()).Order("created_at DESC").Find(&models).Error
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	var runs []map[string]interface{}
	for _, m := range models {
		runs = append(runs, map[string]interface{}{
			"run_id":     m.RunID,
			"resume_id":  m.ResumeID,
			"status":     m.Status,
			"created_at": m.CreatedAt.Format(time.RFC3339),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"runs": runs,
	})
}
