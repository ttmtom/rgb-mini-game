package mission

import (
	"rgb-game/config"
	"rgb-game/internal/core/interfaces"
)

// MissionModule wires together the MissionService dependencies.
type MissionModule struct {
	service *MissionService
}

// NewMissionModule creates a MissionModule backed by the given repository and config.
func NewMissionModule(repo interfaces.MissionRepository, cfg *config.GameConfig) *MissionModule {
	return &MissionModule{
		service: newMissionService(repo, cfg),
	}
}

// Service exposes the underlying MissionService for injection into other modules.
func (m *MissionModule) Service() *MissionService {
	return m.service
}
