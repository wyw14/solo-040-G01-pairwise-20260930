package monitor

import (
	"time"

	"github.com/wyw14/cry-96/internal/admission"
	"github.com/wyw14/cry-96/internal/cycle"
	"github.com/wyw14/cry-96/internal/gate"
	"github.com/wyw14/cry-96/internal/model"
	"github.com/wyw14/cry-96/internal/safety"
	"github.com/wyw14/cry-96/internal/signal"
	"github.com/wyw14/cry-96/internal/stowage"
)

type ChamberView struct {
	ChamberID   string                                 `json:"chamber_id"`
	Cycle       *model.TransitCycle                    `json:"cycle,omitempty"`
	Gates       map[model.Direction]model.GatePosition `json:"gates"`
	Queue       []admission.QueueEntry                 `json:"queue"`
	Signals     []model.SignalState                    `json:"signals"`
	Emergency   model.EmergencyRecord                  `json:"emergency"`
	Stowage     model.StowageConfig                    `json:"stowage"`
	NextLoading *model.StowageReport                   `json:"next_loading,omitempty"`
	UpdatedAt   time.Time                              `json:"updated_at"`
}

// Snapshot aggregates current responsibilities without taking ownership.
type Snapshot struct {
	cycles  *cycle.Coordinator
	gates   *gate.State
	queue   *admission.Service
	signals *signal.Service
	safety  *safety.Service
	stowage *stowage.Planner
	now     func() time.Time
}

func NewSnapshot(cycles *cycle.Coordinator, gates *gate.State, queue *admission.Service, signals *signal.Service, safetyService *safety.Service, stowagePlanner *stowage.Planner, now func() time.Time) *Snapshot {
	if now == nil {
		now = time.Now
	}
	return &Snapshot{cycles: cycles, gates: gates, queue: queue, signals: signals, safety: safetyService, stowage: stowagePlanner, now: now}
}

func (s *Snapshot) Chamber(chamberID string) ChamberView {
	view := ChamberView{
		ChamberID: chamberID, Gates: s.gates.Snapshot(chamberID),
		Queue: s.queue.List(chamberID), Signals: s.signals.States(),
		Emergency: s.safety.Status(chamberID), UpdatedAt: s.now().UTC(),
	}
	if s.stowage != nil {
		if config, err := s.stowage.Registry().Get(chamberID); err == nil {
			view.Stowage = config
			queued := view.Queue
			vessels := make([]model.Vessel, 0, len(queued))
			for _, entry := range queued {
				if !entry.Released {
					vessels = append(vessels, entry.Vessel)
				}
			}
			if len(vessels) > 0 {
				if report, err := s.stowage.Evaluate(config, vessels); err == nil {
					view.NextLoading = &report
				}
			}
		}
	}
	if cycleValue, ok := s.cycles.Current(chamberID); ok {
		view.Cycle = &cycleValue
	}
	return view
}

func (s *Snapshot) Chambers(ids []string) []ChamberView {
	result := make([]ChamberView, 0, len(ids))
	for _, chamberID := range ids {
		result = append(result, s.Chamber(chamberID))
	}
	return result
}
