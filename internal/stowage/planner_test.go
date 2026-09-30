package stowage

import (
	"errors"
	"testing"

	"github.com/wyw14/cry-96/internal/model"
)

func vessel(t *testing.T, name, callSign string, length float64) model.Vessel {
	t.Helper()
	v, err := model.NewVessel(name, callSign, length)
	if err != nil {
		t.Fatalf("new vessel: %v", err)
	}
	return v
}

func config() model.StowageConfig {
	return model.StowageConfig{ChamberID: "chamber-1", EffectiveLengthM: 100, SafetyGapM: 10}
}

func TestEvaluateFitsExactWithSafetyGaps(t *testing.T) {
	planner := NewPlanner(NewRegistry("chamber-1"))
	// 40 + 10 + 40 = 90, leaving 10; a further 40m vessel would need 50.
	vessels := []model.Vessel{
		vessel(t, "Alpha", "A-1", 40),
		vessel(t, "Bravo", "B-2", 40),
		vessel(t, "Charlie", "C-3", 40),
	}
	report, err := planner.Evaluate(config(), vessels)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if report.VesselCount != 2 {
		t.Fatalf("expected 2 loaded, got %d", report.VesselCount)
	}
	if report.OccupiedM != 90 {
		t.Fatalf("expected 90m occupied, got %v", report.OccupiedM)
	}
	if report.RemainingM != 10 {
		t.Fatalf("expected 10m remaining, got %v", report.RemainingM)
	}
	if !report.Exceeded || len(report.Deferred) != 1 {
		t.Fatalf("expected one deferred vessel, got %+v", report.Deferred)
	}
	deferred := report.Deferred[0]
	if deferred.Name != "Charlie" || deferred.QueuePosition != 3 {
		t.Fatalf("unexpected deferred vessel: %+v", deferred)
	}
	if deferred.WouldStartM != 100 || deferred.WouldEndM != 140 || deferred.OverflowM != 40 {
		t.Fatalf("unexpected span: %+v", deferred)
	}
	second := report.Loadings[1]
	if second.StartM != 50 || second.EndM != 90 || second.GapBeforeM != 10 {
		t.Fatalf("unexpected second vessel placement: %+v", second)
	}
	if report.Message == "" {
		t.Fatal("expected a human-readable exceedance message")
	}
}

func TestEvaluateFillsCapacityExactly(t *testing.T) {
	planner := NewPlanner(NewRegistry("chamber-1"))
	// 45 + 10 + 45 = 100 exactly.
	vessels := []model.Vessel{vessel(t, "Alpha", "A-1", 45), vessel(t, "Bravo", "B-2", 45)}
	report, err := planner.Evaluate(config(), vessels)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if report.Exceeded {
		t.Fatalf("did not expect exceedance: %+v", report.Deferred)
	}
	if !report.CapacityReached || report.RemainingM != 0 {
		t.Fatalf("expected chamber filled exactly, remaining=%v reached=%v", report.RemainingM, report.CapacityReached)
	}
}

func TestPlanRejectsSingleVesselLongerThanChamber(t *testing.T) {
	planner := NewPlanner(NewRegistry("chamber-1"))
	report, err := planner.Plan(config(), []model.Vessel{vessel(t, "Giant", "G-9", 140)})
	if err == nil {
		t.Fatal("expected exceedance error")
	}
	var exceedance *model.ExceedanceError
	if !errors.As(err, &exceedance) {
		t.Fatalf("expected *ExceedanceError, got %T", err)
	}
	if report.VesselCount != 0 || len(report.Deferred) != 1 {
		t.Fatalf("expected no loaded vessels, got %+v", report)
	}
	if report.Deferred[0].OverflowM != 40 {
		t.Fatalf("expected 40m overflow, got %v", report.Deferred[0].OverflowM)
	}
}

func TestPlanAcceptsGroupThatFits(t *testing.T) {
	planner := NewPlanner(NewRegistry("chamber-1"))
	vessels := []model.Vessel{vessel(t, "Alpha", "A-1", 30), vessel(t, "Bravo", "B-2", 50)}
	// 30 + 10 + 50 = 90 <= 100.
	report, err := planner.Plan(config(), vessels)
	if err != nil {
		t.Fatalf("expected fit, got %v", err)
	}
	if report.VesselCount != 2 || report.Exceeded {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestPlanDefersAllVesselsBehindBlockingOne(t *testing.T) {
	planner := NewPlanner(NewRegistry("chamber-1"))
	// 80 fits, next 80 needs 80+10+80=170 and must block the third vessel too.
	vessels := []model.Vessel{
		vessel(t, "Alpha", "A-1", 80),
		vessel(t, "Bravo", "B-2", 80),
		vessel(t, "Charlie", "C-3", 5),
	}
	report, err := planner.Plan(config(), vessels)
	if err == nil {
		t.Fatal("expected exceedance error")
	}
	if len(report.Deferred) != 2 {
		t.Fatalf("expected 2 deferred (no skip ahead), got %d", len(report.Deferred))
	}
	if report.Deferred[1].Reason == "" {
		t.Fatal("expected blocked-behind reason for trailing vessel")
	}
}

func TestPlanRejectsEmptyVesselList(t *testing.T) {
	planner := NewPlanner(NewRegistry("chamber-1"))
	if _, err := planner.Plan(config(), nil); err == nil {
		t.Fatal("expected error for empty vessel list")
	}
}

func TestRegistryDefaultsAndOverrides(t *testing.T) {
	registry := NewRegistry("chamber-1")
	got, err := registry.Get("chamber-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.EffectiveLengthM != DefaultEffectiveLengthM || got.SafetyGapM != DefaultSafetyGapM {
		t.Fatalf("unexpected defaults: %+v", got)
	}
	if err := registry.Set(model.StowageConfig{ChamberID: "chamber-1", EffectiveLengthM: 150, SafetyGapM: 2}); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, _ = registry.Get("chamber-1")
	if got.EffectiveLengthM != 150 || got.SafetyGapM != 2 {
		t.Fatalf("override not stored: %+v", got)
	}
	if err := registry.Set(model.StowageConfig{ChamberID: "chamber-9", EffectiveLengthM: 0}); err == nil {
		t.Fatal("expected invalid config to be rejected")
	}
	if _, err := registry.Get("unknown"); err == nil {
		t.Fatal("expected unknown chamber lookup to fail")
	}
}

func TestEvaluateValidatesConfig(t *testing.T) {
	planner := NewPlanner(NewRegistry())
	if _, err := planner.Evaluate(model.StowageConfig{ChamberID: "x", EffectiveLengthM: 0}, []model.Vessel{vessel(t, "A", "A", 10)}); err == nil {
		t.Fatal("expected invalid config error")
	}
}
