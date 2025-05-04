package domain

import (
	"fmt"

	"github.com/nkkko/engram-v3/internal/router"
)

// RouterType represents the type of event router
type RouterType string

const (
	// ChannelRouter represents the channel-based event router
	ChannelRouter RouterType = "channel"
)

// RouterConfig holds common configuration for all router implementations
type RouterConfig struct {
	// Router type
	Type RouterType

	// Specific router configuration
	Config interface{}
}

// NewEventRouter creates a new event router of the specified type
func NewEventRouter(config RouterConfig) (EventRouter, error) {
	switch config.Type {
	case ChannelRouter:
		// Convert config to channel router-specific config
		var routerConfig router.Config
		
		// Check if a specific config was provided
		if config.Config != nil {
			var ok bool
			routerConfig, ok = config.Config.(router.Config)
			if !ok {
				return nil, fmt.Errorf("invalid configuration type for channel router")
			}
		} else {
			// Use default config
			routerConfig = router.DefaultConfig()
		}
		
		return router.NewRouter(routerConfig), nil
		
	default:
		return nil, fmt.Errorf("unsupported router type: %s", config.Type)
	}
}