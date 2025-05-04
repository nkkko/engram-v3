package models

import (
	"github.com/nkkko/engram-v3/pkg/proto"
)

// WorkUnitResponse is the response for a work unit
type WorkUnitResponse struct {
	ID            string                 `json:"id"`
	ContextID     string                 `json:"context_id"`
	AgentID       string                 `json:"agent_id"`
	Type          string                 `json:"type"`
	Timestamp     string                 `json:"timestamp"`
	Payload       string                 `json:"payload"`
	Meta          map[string]string      `json:"meta,omitempty"`
	Relationships []*RelationshipResponse `json:"relationships,omitempty"`
}

// RelationshipResponse is the response for a relationship
type RelationshipResponse struct {
	Type   string `json:"type"`
	Target string `json:"target"`
}

// FromProto converts a proto message to the response
func WorkUnitFromProto(workUnit *proto.WorkUnit) *WorkUnitResponse {
	if workUnit == nil {
		return nil
	}
	
	// Convert timestamp to ISO 8601 format
	var timestamp string
	if workUnit.Ts != nil {
		timestamp = workUnit.Ts.AsTime().Format("2006-01-02T15:04:05Z07:00")
	}
	
	// Convert type to string
	typeStr := "UNKNOWN"
	switch workUnit.Type {
	case proto.WorkUnitType_MESSAGE:
		typeStr = "MESSAGE"
	case proto.WorkUnitType_SYSTEM:
		typeStr = "SYSTEM"
	}
	
	// Convert relationships
	var relationships []*RelationshipResponse
	for _, rel := range workUnit.Relationships {
		relationships = append(relationships, &RelationshipResponse{
			Type:   rel.Type,
			Target: rel.Target,
		})
	}
	
	return &WorkUnitResponse{
		ID:            workUnit.Id,
		ContextID:     workUnit.ContextId,
		AgentID:       workUnit.AgentId,
		Type:          typeStr,
		Timestamp:     timestamp,
		Payload:       workUnit.Payload,
		Meta:          workUnit.Meta,
		Relationships: relationships,
	}
}

// ContextResponse is the response for a context
type ContextResponse struct {
	ID          string            `json:"id"`
	DisplayName string            `json:"display_name"`
	AgentIDs    []string          `json:"agent_ids,omitempty"`
	PinnedUnits []string          `json:"pinned_units,omitempty"`
	Meta        map[string]string `json:"meta,omitempty"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

// FromProto converts a proto message to the response
func ContextFromProto(context *proto.Context) *ContextResponse {
	if context == nil {
		return nil
	}
	
	// Convert timestamps to ISO 8601 format
	var createdAt, updatedAt string
	if context.CreatedAt != nil {
		createdAt = context.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z07:00")
	}
	if context.UpdatedAt != nil {
		updatedAt = context.UpdatedAt.AsTime().Format("2006-01-02T15:04:05Z07:00")
	}
	
	return &ContextResponse{
		ID:          context.Id,
		DisplayName: context.DisplayName,
		AgentIDs:    context.AgentIds,
		PinnedUnits: context.PinnedUnits,
		Meta:        context.Meta,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}

// LockResponse is the response for a lock
type LockResponse struct {
	ResourcePath string `json:"resource_path"`
	HolderAgent  string `json:"holder_agent"`
	LockID       string `json:"lock_id"`
	ExpiresAt    string `json:"expires_at"`
}

// FromProto converts a proto message to the response
func LockFromProto(lock *proto.LockHandle) *LockResponse {
	if lock == nil {
		return nil
	}
	
	// Convert timestamp to ISO 8601 format
	var expiresAt string
	if lock.ExpiresAt != nil {
		expiresAt = lock.ExpiresAt.AsTime().Format("2006-01-02T15:04:05Z07:00")
	}
	
	return &LockResponse{
		ResourcePath: lock.ResourcePath,
		HolderAgent:  lock.HolderAgent,
		LockID:       lock.LockId,
		ExpiresAt:    expiresAt,
	}
}

// PaginationMeta contains pagination metadata
type PaginationMeta struct {
	TotalCount int    `json:"total_count"`
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// ListWorkUnitsResponse is the response for listing work units
type ListWorkUnitsResponse struct {
	WorkUnits []*WorkUnitResponse `json:"work_units"`
	Meta      PaginationMeta      `json:"meta"`
}

// SearchResponse is the response for a search
type SearchResponse struct {
	Results []*WorkUnitResponse `json:"results"`
	Meta    PaginationMeta      `json:"meta"`
}