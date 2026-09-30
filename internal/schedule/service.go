package schedule

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/wyw14/cry-96/internal/model"
	"github.com/wyw14/cry-96/internal/signal"
)

// Service exposes planning and signal-publication workflows to the API.
type Service struct {
	planner   *Planner
	publisher *signal.Publisher
}

func NewService(planner *Planner, publisher *signal.Publisher) *Service {
	return &Service{planner: planner, publisher: publisher}
}

func (s *Service) Create(ctx context.Context, chamberID string, direction model.Direction, vessels []model.Vessel) (model.ConvoyPlan, error) {
	return s.planner.Create(ctx, chamberID, direction, vessels)
}

// Preview reports the loading outcome without creating a plan or touching
// the admission queue.
func (s *Service) Preview(chamberID string, vessels []model.Vessel) (model.StowageReport, error) {
	return s.planner.Preview(chamberID, vessels)
}

// StowageConfig returns the configured loading geometry for a chamber.
func (s *Service) StowageConfig(chamberID string) (model.StowageConfig, error) {
	return s.planner.Stowage().Registry().Get(chamberID)
}

// SetStowageConfig validates and updates a chamber's loading geometry.
func (s *Service) SetStowageConfig(config model.StowageConfig) error {
	return s.planner.Stowage().Registry().Set(config)
}

// StowageConfigs lists chamber loading geometry in runtime chamber order.
func (s *Service) StowageConfigs(orderedIDs []string) []model.StowageConfig {
	return s.planner.Stowage().Registry().List(orderedIDs)
}

func (s *Service) Publish(ctx context.Context, planID uuid.UUID, cycle model.TransitCycle) (signal.Publication, error) {
	plan, ok := s.planner.Get(planID)
	if !ok || plan.Cancelled {
		return signal.Publication{}, errors.New("active convoy plan not found")
	}
	if plan.ChamberID != cycle.ChamberID || plan.ID != cycle.PlanID {
		return signal.Publication{}, errors.New("plan and transit do not match")
	}
	return s.publisher.Publish(ctx, cycle, plan.Vessels, model.SignalProceed)
}

func (s *Service) Cancel(ctx context.Context, planID uuid.UUID, note string, publication signal.Publication) (Cancellation, error) {
	return s.planner.Cancel(ctx, planID, note, s.publisher, publication)
}

func (s *Service) Active(chamberID string) []model.ConvoyPlan {
	return s.planner.Active(chamberID)
}
