package model

import (
	"errors"
)

// StowageConfig describes the usable length of a chamber and the safety gap
// required between consecutive vessels. Effective length already excludes the
// structural end clearances that must never be occupied.
type StowageConfig struct {
	ChamberID        string  `json:"chamber_id"`
	EffectiveLengthM float64 `json:"effective_length_m"`
	SafetyGapM       float64 `json:"safety_gap_m"`
}

func (c StowageConfig) Valid() error {
	if c.ChamberID == "" {
		return errors.New("stowage config requires a chamber id")
	}
	if c.EffectiveLengthM <= 0 {
		return errors.New("stowage effective length must be positive")
	}
	if c.SafetyGapM < 0 {
		return errors.New("stowage safety gap cannot be negative")
	}
	return nil
}

// VesselLoading reports one vessel's position inside the loading calculation.
type VesselLoading struct {
	VesselID      string  `json:"vessel_id"`
	Name          string  `json:"name"`
	CallSign      string  `json:"call_sign"`
	LengthM       float64 `json:"length_m"`
	QueuePosition int     `json:"queue_position"`
	StartM        float64 `json:"start_m"`
	EndM          float64 `json:"end_m"`
	GapBeforeM    float64 `json:"gap_before_m"`
	Loaded        bool    `json:"loaded"`
}

// DeferredVessel names a queue vessel that cannot enter in this group and why.
type DeferredVessel struct {
	VesselID      string  `json:"vessel_id"`
	Name          string  `json:"name"`
	CallSign      string  `json:"call_sign"`
	LengthM       float64 `json:"length_m"`
	QueuePosition int     `json:"queue_position"`
	WouldStartM   float64 `json:"would_start_m"`
	WouldEndM     float64 `json:"would_end_m,omitempty"`
	OverflowM     float64 `json:"overflow_m,omitempty"`
	Reason        string  `json:"reason"`
}

// StowageReport is the deterministic result of one chamber loading pass.
type StowageReport struct {
	ChamberID        string           `json:"chamber_id"`
	EffectiveLengthM float64          `json:"effective_length_m"`
	SafetyGapM       float64          `json:"safety_gap_m"`
	Loadings         []VesselLoading  `json:"loadings"`
	OccupiedM        float64          `json:"occupied_m"`
	RemainingM       float64          `json:"remaining_m"`
	VesselCount      int              `json:"vessel_count"`
	CapacityReached  bool             `json:"capacity_reached"`
	Deferred         []DeferredVessel `json:"deferred,omitempty"`
	Exceeded         bool             `json:"exceeded"`
	Message          string           `json:"message,omitempty"`
}

// ExceedanceError carries the full report so callers can show the operator
// exactly which vessel overflows and how much chamber length is occupied.
type ExceedanceError struct {
	Report StowageReport
}

func (e *ExceedanceError) Error() string { return e.Report.Message }
