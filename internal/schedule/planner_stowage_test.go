package schedule

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wyw14/cry-96/internal/admission"
	"github.com/wyw14/cry-96/internal/gate"
	"github.com/wyw14/cry-96/internal/journal"
	"github.com/wyw14/cry-96/internal/model"
	"github.com/wyw14/cry-96/internal/signal"
	"github.com/wyw14/cry-96/internal/stowage"
)

func newTestPlanner(t *testing.T, effectiveLength float64) (*Planner, *journal.MemoryStore) {
	t.Helper()
	now := func() time.Time { return time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC) }
	store := journal.NewMemoryStore()
	recorder := journal.NewRecorder(store, now)
	registry := stowage.NewRegistry("chamber-1")
	if err := registry.Set(model.StowageConfig{
		ChamberID: "chamber-1", EffectiveLengthM: effectiveLength, SafetyGapM: 10,
	}); err != nil {
		t.Fatalf("config: %v", err)
	}
	queue := admission.NewQueue()
	controller := admission.NewController(queue, gate.NewState("chamber-1"), signal.NewStore(), now)
	service := admission.NewService(controller, recorder, now)
	return NewPlanner(stowage.NewPlanner(registry), service, recorder, now), store
}

func mkVessel(t *testing.T, name string, length float64) model.Vessel {
	t.Helper()
	v, err := model.NewVessel(name, name+"-CS", length)
	if err != nil {
		t.Fatalf("vessel: %v", err)
	}
	return v
}

func TestCreateRejectsOverflowWithoutSideEffects(t *testing.T) {
	planner, store := newTestPlanner(t, 100)
	ctx := context.Background()
	// 60 + 10 + 60 = 130 > 100.
	_, err := planner.Create(ctx, "chamber-1", model.DirectionUpstream,
		[]model.Vessel{mkVessel(t, "Alpha", 60), mkVessel(t, "Bravo", 60)})

	var exceedance *model.ExceedanceError
	if !errors.As(err, &exceedance) {
		t.Fatalf("expected ExceedanceError, got %v", err)
	}
	if exceedance.Report.VesselCount != 1 || len(exceedance.Report.Deferred) != 1 {
		t.Fatalf("unexpected report: %+v", exceedance.Report)
	}
	if exceedance.Report.Deferred[0].Name != "Bravo" {
		t.Fatalf("expected Bravo to be named, got %q", exceedance.Report.Deferred[0].Name)
	}
	if len(planner.Active("chamber-1")) != 0 {
		t.Fatal("no plan must exist after rejection")
	}
	if queued := planner.admission.List("chamber-1"); len(queued) != 0 {
		t.Fatalf("queue must stay empty, got %d entries", len(queued))
	}
	events, err := store.Events(ctx, "chamber-1", 0)
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("no events may be recorded for a half plan, got %d", len(events))
	}
}

func TestCreateStoresStowageReportForFittingPlan(t *testing.T) {
	planner, store := newTestPlanner(t, 100)
	ctx := context.Background()
	// 40 + 10 + 40 = 90 <= 100.
	plan, err := planner.Create(ctx, "chamber-1", model.DirectionUpstream,
		[]model.Vessel{mkVessel(t, "Alpha", 40), mkVessel(t, "Bravo", 40)})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if plan.Stowage.VesselCount != 2 || plan.Stowage.OccupiedM != 90 {
		t.Fatalf("unexpected stowage on plan: %+v", plan.Stowage)
	}
	if got := planner.Active("chamber-1"); len(got) != 1 || got[0].Stowage.SafetyGapM != 10 {
		t.Fatalf("stored plan missing stowage: %+v", got)
	}
	events, err := store.Events(ctx, "chamber-1", 0)
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected plan events for a created plan")
	}
}

func TestPreviewDoesNotCreatePlan(t *testing.T) {
	planner, _ := newTestPlanner(t, 100)
	report, err := planner.Preview("chamber-1",
		[]model.Vessel{mkVessel(t, "Alpha", 90), mkVessel(t, "Bravo", 90)})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if !report.Exceeded || report.VesselCount != 1 {
		t.Fatalf("unexpected preview: %+v", report)
	}
	if len(planner.Active("chamber-1")) != 0 {
		t.Fatal("preview must not create a plan")
	}
}
