//go:build integration

package protocol_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// The rungs of the key are walked in course_test.go, where they are a function of their
// arguments: `protocol.Create` writes one transaction per course and lets the database choose
// the id, so a fixture of two rows sharing a created_at with ids against the order of
// insertion is not one this suite arranges — raw INSERTs would build it, but the key is pure
// and measures cheaper where it lives. What is measured here is the read around it: that every
// course of the patient reaches the key, that only theirs does, and that the chosen one comes
// back whole.
func lastPlanFor(t *testing.T, pool *pgxpool.Pool, patient string) (protocol.Plan, bool) {
	t.Helper()

	var (
		plan  protocol.Plan
		found bool
	)
	err := database.WithServiceJob(
		t.Context(), pool, writeJob,
		func(ctx context.Context, tx pgx.Tx) error {
			var err error
			plan, found, err = protocol.LastPlanFor(ctx, tx, civil.UserID(patient))

			return err
		},
	)
	if err != nil {
		t.Fatalf("reading the last course of %s: %v", patient, err)
	}

	return plan, found
}

func courseStarting(patient string, start civil.Date, status protocol.ProtocolStatus) protocol.Draft {
	draft := aCourse(patient)
	draft.StartDate = start
	draft.Status = status

	return draft
}

// The running course, even though a closed one starts two months after it. `start_date` has no
// upper bound, so this is not a contrived fixture: a course written for a patient and then
// abandoned outlives the one they are actually on, by date.
func TestTheRunningCourseWinsOverAClosedOneThatStartsLater(t *testing.T) {
	pool, _ := prescribing(t)

	running, err := protocol.Create(as(t, writeDoctorA, "doctor"), pool,
		courseStarting(writePatientA, civil.NewDate(2026, time.May, 4), protocol.StatusActive))
	if err != nil {
		t.Fatalf("prescribing the running course: %v", err)
	}
	if _, err := protocol.Create(
		as(t, writeDoctorA, "doctor"), pool,
		courseStarting(writePatientA, civil.NewDate(2026, time.July, 1), protocol.StatusCompleted),
	); err != nil {
		t.Fatalf("prescribing the closed course: %v", err)
	}

	// Somebody else's course, later still: the read takes every row of one patient, so
	// the predicate that keeps it to theirs is the whole of the isolation here.
	if _, err := protocol.Create(
		as(t, writeDoctorB, "doctor"), pool,
		courseStarting(writePatientB, civil.NewDate(2026, time.August, 1), protocol.StatusActive),
	); err != nil {
		t.Fatalf("prescribing the other patient's course: %v", err)
	}

	plan, found := lastPlanFor(t, pool, writePatientA)
	if !found {
		t.Fatal("answered no course")
	}
	if plan.Protocol.ID != running.ProtocolID {
		t.Errorf("chose %s, not the running course %s", plan.Protocol.ID, running.ProtocolID)
	}
	if plan.Protocol.PatientID != civil.UserID(writePatientA) {
		t.Errorf("the course belongs to %s", plan.Protocol.PatientID)
	}

	// Whole or not at all, like ActivePlanFor: the overlay of step 5 reads the items and
	// their phases off this same plan, and a course whose items did not load draws an axis
	// with nothing under it.
	if len(plan.Items) != 1 {
		t.Fatalf("read %d items, not 1", len(plan.Items))
	}
	if phases := plan.Phases[plan.Items[0].ID]; len(phases) != 2 {
		t.Errorf("read %d phases, not 2", len(phases))
	}
	if plan.Protocol.CreatedAt.IsZero() {
		t.Error("created_at came back zero, and it is a rung of the key")
	}
}

// The read the day's screen cannot do: a patient whose course ended has no running one, and
// the trends axis is still their last course's geometry.
func TestAPatientBetweenCoursesStillHasALastOne(t *testing.T) {
	pool, _ := prescribing(t)

	closed, err := protocol.Create(as(t, writeDoctorA, "doctor"), pool,
		courseStarting(writePatientA, civil.NewDate(2026, time.February, 2), protocol.StatusCompleted))
	if err != nil {
		t.Fatalf("prescribing: %v", err)
	}

	if _, running, err := planFor(t, pool, writePatientA); err != nil || running {
		t.Fatalf("the running read answered (%v, %v)", running, err)
	}

	plan, found := lastPlanFor(t, pool, writePatientA)
	if !found {
		t.Fatal("answered no course")
	}
	if plan.Protocol.ID != closed.ProtocolID {
		t.Errorf("chose %s, not %s", plan.Protocol.ID, closed.ProtocolID)
	}
	if plan.Protocol.Status != protocol.StatusCompleted {
		t.Errorf("the course came back %s", plan.Protocol.Status)
	}
	if len(plan.Items) != 1 {
		t.Errorf("read %d items, not 1", len(plan.Items))
	}
}

// No course and no error: an empty axis is an answer, and a patient who was never prescribed
// anything is the first of the cycle window's two empty cases.
func TestAPatientWhoWasNeverPrescribedHasNoLastCourse(t *testing.T) {
	pool, _ := prescribing(t)

	if plan, found := lastPlanFor(t, pool, writePatientA); found {
		t.Errorf("answered course %s", plan.Protocol.ID)
	}
}

// The service seam reads every row; the patient's own does not. Not a duplicate of the policy
// suite: this one says that LastPlanFor's SELECT — a whole-table read filtered by a parameter
// rather than by id — is still refused a course that is not the caller's when it runs on the
// request pool.
func TestTheRequestSeamReadsNobodyElsesCourses(t *testing.T) {
	service, requests := prescribingWithRequests(t)

	written, err := protocol.Create(as(t, writeDoctorB, "doctor"), service,
		courseStarting(writePatientB, civil.NewDate(2026, time.May, 4), protocol.StatusActive))
	if err != nil {
		t.Fatalf("prescribing: %v", err)
	}

	asPatient := func(caller string) (protocol.Plan, bool) {
		t.Helper()

		var (
			plan  protocol.Plan
			found bool
		)
		err := database.WithCaller(
			t.Context(), requests,
			database.Caller{Subject: caller, Role: "patient"},
			func(ctx context.Context, tx pgx.Tx) error {
				var err error
				plan, found, err = protocol.LastPlanFor(ctx, tx, civil.UserID(writePatientB))

				return err
			},
		)
		if err != nil {
			t.Fatalf("reading as %s: %v", caller, err)
		}

		return plan, found
	}

	// The owner first, and not as a formality: without it a read that answered nothing to
	// everybody — a missing grant, a column the request roles cannot select — would pass
	// the refusal below while proving nothing about the policy.
	plan, found := asPatient(writePatientB)
	if !found {
		t.Fatal("the patient did not read their own last course")
	}
	if plan.Protocol.ID != written.ProtocolID {
		t.Errorf("read course %s, not %s", plan.Protocol.ID, written.ProtocolID)
	}
	if len(plan.Items) != 1 {
		t.Errorf("read %d items, not 1", len(plan.Items))
	}

	if _, found := asPatient(writePatientA); found {
		t.Error("one patient read another's last course")
	}
}
