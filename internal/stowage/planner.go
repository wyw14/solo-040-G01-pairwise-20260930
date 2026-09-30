package stowage

import (
	"errors"
	"fmt"
	"math"

	"github.com/wyw14/cry-96/internal/model"
)

const lengthEpsilon = 1e-9

// Planner computes which queue vessels enter a chamber in one lockage.
//
// Vessels are walked strictly in queue order. A safety gap is inserted before
// every vessel except the first. Walking stops at the first vessel whose far
// end would pass the chamber's effective length: that vessel and every vessel
// behind it are reported as deferred. A group is therefore always a prefix of
// the queue, never a partial ("half") selection.
type Planner struct {
	registry *Registry
}

func NewPlanner(registry *Registry) *Planner {
	if registry == nil {
		registry = NewRegistry()
	}
	return &Planner{registry: registry}
}

func (p *Planner) Registry() *Registry { return p.registry }

// Evaluate runs one loading pass against the supplied configuration.
func (p *Planner) Evaluate(config model.StowageConfig, vessels []model.Vessel) (model.StowageReport, error) {
	if err := config.Valid(); err != nil {
		return model.StowageReport{}, err
	}
	report := model.StowageReport{
		ChamberID:        config.ChamberID,
		EffectiveLengthM: config.EffectiveLengthM,
		SafetyGapM:       config.SafetyGapM,
	}
	cursor := 0.0
	for index, vessel := range vessels {
		position := index + 1
		gapBefore := 0.0
		if index > 0 {
			gapBefore = config.SafetyGapM
		}
		start := cursor + gapBefore
		end := start + vessel.LengthM
		if end <= config.EffectiveLengthM+lengthEpsilon {
			report.Loadings = append(report.Loadings, model.VesselLoading{
				VesselID: vessel.ID.String(), Name: vessel.Name, CallSign: vessel.CallSign,
				LengthM: vessel.LengthM, QueuePosition: position,
				StartM: roundMeters(start), EndM: roundMeters(end),
				GapBeforeM: roundMeters(gapBefore), Loaded: true,
			})
			cursor = end
			continue
		}
		report.Deferred = append(report.Deferred, model.DeferredVessel{
			VesselID: vessel.ID.String(), Name: vessel.Name, CallSign: vessel.CallSign,
			LengthM: vessel.LengthM, QueuePosition: position,
			WouldStartM: roundMeters(start), WouldEndM: roundMeters(end),
			OverflowM: roundMeters(end - config.EffectiveLengthM),
			Reason:    "vessel exceeds remaining effective chamber length",
		})
		for behind := index + 1; behind < len(vessels); behind++ {
			queued := vessels[behind]
			report.Deferred = append(report.Deferred, model.DeferredVessel{
				VesselID: queued.ID.String(), Name: queued.Name, CallSign: queued.CallSign,
				LengthM: queued.LengthM, QueuePosition: behind + 1,
				Reason: "queued behind a vessel that does not fit in this lockage",
			})
		}
		break
	}
	report.OccupiedM = roundMeters(cursor)
	report.RemainingM = roundMeters(config.EffectiveLengthM - cursor)
	report.VesselCount = len(report.Loadings)
	report.Exceeded = len(report.Deferred) > 0
	report.CapacityReached = report.VesselCount > 0 && report.RemainingM <= lengthEpsilon
	if report.Exceeded {
		report.Message = exceedanceMessage(config, report)
	}
	return report, nil
}

// EvaluateForChamber resolves the chamber configuration before evaluating.
func (p *Planner) EvaluateForChamber(chamberID string, vessels []model.Vessel) (model.StowageReport, error) {
	config, err := p.registry.Get(chamberID)
	if err != nil {
		return model.StowageReport{}, err
	}
	return p.Evaluate(config, vessels)
}

// Plan is the strict gate used before creating a convoy plan: every submitted
// vessel must fit in a single lockage. On overflow it returns an
// ExceedanceError carrying the detailed report; the caller must not persist a
// plan for only the fitting prefix.
func (p *Planner) Plan(config model.StowageConfig, vessels []model.Vessel) (model.StowageReport, error) {
	if len(vessels) == 0 {
		return model.StowageReport{}, errors.New("stowage requires at least one vessel")
	}
	report, err := p.Evaluate(config, vessels)
	if err != nil {
		return model.StowageReport{}, err
	}
	if report.Exceeded {
		return report, &model.ExceedanceError{Report: report}
	}
	return report, nil
}

// PlanForChamber resolves configuration and applies the strict plan rules.
func (p *Planner) PlanForChamber(chamberID string, vessels []model.Vessel) (model.StowageReport, error) {
	config, err := p.registry.Get(chamberID)
	if err != nil {
		return model.StowageReport{}, err
	}
	return p.Plan(config, vessels)
}

func exceedanceMessage(config model.StowageConfig, report model.StowageReport) string {
	first := report.Deferred[0]
	if report.VesselCount == 0 {
		return fmt.Sprintf(
			"vessel %q (%s, %.2fm) at queue position %d does not fit: it would occupy %.2fm to %.2fm but chamber %s effective length is %.2fm (overflow %.2fm)",
			first.Name, first.CallSign, first.LengthM, first.QueuePosition,
			first.WouldStartM, first.WouldEndM, config.ChamberID,
			config.EffectiveLengthM, first.OverflowM,
		)
	}
	return fmt.Sprintf(
		"vessel %q (%s, %.2fm) at queue position %d does not fit after %d loaded vessel(s): far end %.2fm exceeds chamber %s effective length %.2fm by %.2fm; occupied %.2fm with %.2fm remaining",
		first.Name, first.CallSign, first.LengthM, first.QueuePosition,
		report.VesselCount, first.WouldEndM, config.ChamberID,
		config.EffectiveLengthM, first.OverflowM, report.OccupiedM, report.RemainingM,
	)
}

func roundMeters(value float64) float64 { return math.Round(value*1000) / 1000 }
