package stowage

import (
	"errors"
	"fmt"
	"sync"

	"github.com/wyw14/cry-96/internal/model"
)

// DefaultEffectiveLengthM and DefaultSafetyGapM are used for chambers that
// have not received an explicit configuration yet.
const (
	DefaultEffectiveLengthM = 280.0
	DefaultSafetyGapM       = 5.0
)

// Registry holds the configurable loading geometry per chamber.
type Registry struct {
	mu      sync.RWMutex
	configs map[string]model.StowageConfig
}

func NewRegistry(chamberIDs ...string) *Registry {
	registry := &Registry{configs: make(map[string]model.StowageConfig)}
	for _, chamberID := range chamberIDs {
		registry.configs[chamberID] = model.StowageConfig{
			ChamberID:        chamberID,
			EffectiveLengthM: DefaultEffectiveLengthM,
			SafetyGapM:       DefaultSafetyGapM,
		}
	}
	return registry
}

// Set validates and stores a chamber configuration.
func (r *Registry) Set(config model.StowageConfig) error {
	if err := config.Valid(); err != nil {
		return err
	}
	r.mu.Lock()
	r.configs[config.ChamberID] = config
	r.mu.Unlock()
	return nil
}

func (r *Registry) Get(chamberID string) (model.StowageConfig, error) {
	if chamberID == "" {
		return model.StowageConfig{}, errors.New("chamber id is required")
	}
	r.mu.RLock()
	config, ok := r.configs[chamberID]
	r.mu.RUnlock()
	if !ok {
		return model.StowageConfig{}, fmt.Errorf("no stowage configuration for chamber %q", chamberID)
	}
	return config, nil
}

// List returns configurations in the order of the requested chamber ids,
// appending any additionally configured chambers afterwards.
func (r *Registry) List(orderedIDs []string) []model.StowageConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := make(map[string]bool, len(orderedIDs))
	result := make([]model.StowageConfig, 0, len(r.configs))
	for _, chamberID := range orderedIDs {
		if config, ok := r.configs[chamberID]; ok {
			result = append(result, config)
			seen[chamberID] = true
		}
	}
	for chamberID, config := range r.configs {
		if !seen[chamberID] {
			result = append(result, config)
		}
	}
	return result
}
