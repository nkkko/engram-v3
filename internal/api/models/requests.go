package models

import (
	"github.com/nkkko/engram-v3/internal/api/errors"
	"github.com/nkkko/engram-v3/pkg/proto"
)

// CreateWorkUnitRequest is the request to create a work unit
type CreateWorkUnitRequest struct {
	ContextID     string                   `json:"context_id"`
	AgentID       string                   `json:"agent_id"`
	Type          proto.WorkUnitType       `json:"type"`
	Payload       string                   `json:"payload"`
	Meta          map[string]string        `json:"meta,omitempty"`
	Relationships []*proto.Relationship    `json:"relationships,omitempty"`
}

// Validate validates the request
func (r *CreateWorkUnitRequest) Validate() error {
	if r.ContextID == "" {
		return errors.ValidationError("missing_context_id", "Context ID is required")
	}
	
	if r.AgentID == "" {
		return errors.ValidationError("missing_agent_id", "Agent ID is required")
	}
	
	if r.Type != proto.WorkUnitType_MESSAGE && r.Type != proto.WorkUnitType_SYSTEM {
		return errors.ValidationError("invalid_type", "Type must be MESSAGE or SYSTEM")
	}
	
	return nil
}

// ToProto converts the request to the proto message
func (r *CreateWorkUnitRequest) ToProto() *proto.CreateWorkUnitRequest {
	return &proto.CreateWorkUnitRequest{
		ContextId:     r.ContextID,
		AgentId:       r.AgentID,
		Type:          r.Type,
		Payload:       r.Payload,
		Meta:          r.Meta,
		Relationships: r.Relationships,
	}
}

// UpdateContextRequest is the request to update a context
type UpdateContextRequest struct {
	ID          string            `json:"id"`
	DisplayName string            `json:"display_name,omitempty"`
	AddAgents   []string          `json:"add_agents,omitempty"`
	RemoveAgents []string         `json:"remove_agents,omitempty"`
	PinUnits    []string          `json:"pin_units,omitempty"`
	UnpinUnits  []string          `json:"unpin_units,omitempty"`
	Meta        map[string]string `json:"meta,omitempty"`
}

// Validate validates the request
func (r *UpdateContextRequest) Validate() error {
	if r.ID == "" {
		return errors.ValidationError("missing_id", "ID is required")
	}
	
	return nil
}

// ToProto converts the request to the proto message
func (r *UpdateContextRequest) ToProto() *proto.UpdateContextRequest {
	return &proto.UpdateContextRequest{
		Id:           r.ID,
		DisplayName:  r.DisplayName,
		AddAgents:    r.AddAgents,
		RemoveAgents: r.RemoveAgents,
		PinUnits:     r.PinUnits,
		UnpinUnits:   r.UnpinUnits,
		Meta:         r.Meta,
	}
}

// AcquireLockRequest is the request to acquire a lock
type AcquireLockRequest struct {
	ResourcePath string `json:"resource_path"`
	AgentID      string `json:"agent_id"`
	TTLSeconds   int    `json:"ttl_seconds"`
}

// Validate validates the request
func (r *AcquireLockRequest) Validate() error {
	if r.ResourcePath == "" {
		return errors.ValidationError("missing_resource_path", "Resource path is required")
	}
	
	if r.AgentID == "" {
		return errors.ValidationError("missing_agent_id", "Agent ID is required")
	}
	
	if r.TTLSeconds <= 0 {
		r.TTLSeconds = 60 // Default to 1 minute
	}
	
	return nil
}

// ToProto converts the request to the proto message
func (r *AcquireLockRequest) ToProto() *proto.AcquireLockRequest {
	return &proto.AcquireLockRequest{
		ResourcePath: r.ResourcePath,
		AgentId:      r.AgentID,
		TtlSeconds:   int32(r.TTLSeconds),
	}
}

// ReleaseLockRequest is the request to release a lock
type ReleaseLockRequest struct {
	ResourcePath string `json:"resource_path"`
	AgentID      string `json:"agent_id"`
	LockID       string `json:"lock_id"`
}

// Validate validates the request
func (r *ReleaseLockRequest) Validate() error {
	if r.ResourcePath == "" {
		return errors.ValidationError("missing_resource_path", "Resource path is required")
	}
	
	if r.AgentID == "" {
		return errors.ValidationError("missing_agent_id", "Agent ID is required")
	}
	
	if r.LockID == "" {
		return errors.ValidationError("missing_lock_id", "Lock ID is required")
	}
	
	return nil
}

// ToProto converts the request to the proto message
func (r *ReleaseLockRequest) ToProto() *proto.ReleaseLockRequest {
	return &proto.ReleaseLockRequest{
		ResourcePath: r.ResourcePath,
		AgentId:      r.AgentID,
		LockId:       r.LockID,
	}
}

// SearchRequest is the request to search for work units
type SearchRequest struct {
	Query       string            `json:"query"`
	ContextID   string            `json:"context_id,omitempty"`
	AgentIDs    []string          `json:"agent_ids,omitempty"`
	Types       []string          `json:"types,omitempty"`
	MetaFilters map[string]string `json:"meta_filters,omitempty"`
	Limit       int               `json:"limit,omitempty"`
	Offset      int               `json:"offset,omitempty"`
}

// Validate validates the request
func (r *SearchRequest) Validate() error {
	if r.Limit <= 0 || r.Limit > 1000 {
		r.Limit = 100 // Default to 100
	}
	
	if r.Offset < 0 {
		r.Offset = 0
	}
	
	return nil
}