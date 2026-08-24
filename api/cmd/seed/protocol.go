package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// courseWeeks and demoWeek: the course runs twelve weeks and the seeded day sits
// in the fourth, which is the week every screen was drawn against and every KMP
// fixture winds to.
const (
	courseWeeks = 12
	demoWeek    = 4
)

// courseStart answers the day the seeded course began, counted back from today
// rather than written down.
//
// A literal start date is a fixture with an expiry, and this project has already
// paid for one: MockSeed's course ran from 10 May 2026 and ended on 1 August, and
// every screen went blank on the second of August with the suite still green —
// ActivePlanFor reads the active course, and there was none. Counted back, the
// seeded patient is in week four whenever the seed is run.
//
// A Sunday, because the weekly injection is prescribed on one: the course opening
// mid-week would put its first occurrence six days in.
func courseStart(today civil.Date) civil.Date {
	sunday := today.AddDays(-int(today.Weekday()))

	return sunday.AddDays(-(demoWeek - 1) * 7)
}

// theCourse is the whole protocol as data: twelve weeks of titrated semaglutide,
// BPC-157 twice a day, an evening supplement and a Sunday weigh-in.
//
// The first three are MockSeed's, field for field, because those are what the
// screens were drawn against. The weigh-in is not in MockSeed at all — WEIGH_IN
// is declared in the KMP enum and used nowhere — and it is here so that all three
// kinds of occurrence are on a real stand rather than only injections.
func theCourse(patient civil.UserID, today civil.Date) protocol.Draft {
	return protocol.Draft{
		PatientID: patient,
		StartDate: courseStart(today),
		Weeks:     courseWeeks,
		Status:    protocol.StatusActive,
		Items: []protocol.DraftItem{
			{
				Kind: protocol.KindInjection,
				Compound: protocol.CompoundRef{New: &protocol.NewCompound{
					NameRU:      "Семаглутид",
					DefaultUnit: protocol.MG,
					Route:       "п/к",
					Icon:        "beaker",
				}},
				Cadence:    protocol.CadenceWeekly,
				DaysOfWeek: []time.Weekday{time.Sunday},
				Times:      []civil.Slot{{Hour: 7}},
				Loggable:   true,
				// The titration the screens show: 0,25 → 0,5 → 1,0, four weeks each.
				Phases: []protocol.ProtocolPhase{
					{FromWeek: 1, ToWeek: 4, Dose: protocol.Dose{Value: 0.25, Unit: protocol.MG}},
					{FromWeek: 5, ToWeek: 8, Dose: protocol.Dose{Value: 0.5, Unit: protocol.MG}},
					{FromWeek: 9, ToWeek: 12, Dose: protocol.Dose{Value: 1.0, Unit: protocol.MG}},
				},
			},
			{
				// Two slots on one item, which is what makes an occurrence keyed by
				// (item, date, time) rather than by (item, date).
				Kind: protocol.KindInjection,
				Compound: protocol.CompoundRef{New: &protocol.NewCompound{
					NameRU:      "BPC-157",
					DefaultUnit: protocol.MCG,
					Route:       "п/к",
					Icon:        "beaker",
				}},
				Cadence:  protocol.CadenceDaily,
				Times:    []civil.Slot{{Hour: 8}, {Hour: 20}},
				Loggable: true,
				Phases: []protocol.ProtocolPhase{
					{FromWeek: 1, ToWeek: 12, Dose: protocol.Dose{Value: 250, Unit: protocol.MCG}},
				},
			},
			{
				// No phases, so no dose on the row — which is what makes the strip's
				// dose column meaningfully optional rather than always filled. And
				// not loggable: the clinic tracks it without asking.
				Kind: protocol.KindSupplement,
				Compound: protocol.CompoundRef{New: &protocol.NewCompound{
					NameRU:      "Глицин + магний",
					DefaultUnit: protocol.MG,
					Route:       "внутрь",
					Icon:        "moon",
				}},
				Cadence:  protocol.CadenceDaily,
				Times:    []civil.Slot{{Hour: 21, Minute: 30}},
				Loggable: false,
			},
			{
				// Not loggable, and that is a limit rather than a choice: nothing
				// records a weight yet — measurements is an unbuilt context — so a
				// row offering «записать» would be a screen promising what no
				// endpoint accepts.
				Kind:       protocol.KindWeighIn,
				Cadence:    protocol.CadenceWeekly,
				DaysOfWeek: []time.Weekday{time.Sunday},
				Times:      []civil.Slot{{Hour: 7, Minute: 30}},
				Loggable:   false,
			},
		},
	}
}

// prescribe puts the course on the patient, once.
//
// Re-running the seed is ordinary, so a patient who already holds a course is
// left with the one they have: prescribing a second would give them two active
// protocols, and every read answers the first it finds.
func prescribe(
	ctx context.Context, writes *pgxpool.Pool, patient civil.UserID, today civil.Date,
) (bool, error) {
	held, err := holdsACourse(ctx, writes, patient)
	if err != nil {
		return false, err
	}
	if held {
		return false, nil
	}

	if _, err := protocol.Create(ctx, writes, theCourse(patient, today)); err != nil {
		return false, fmt.Errorf("prescribing: %w", err)
	}

	return true, nil
}

func holdsACourse(ctx context.Context, writes *pgxpool.Pool, patient civil.UserID) (bool, error) {
	var held bool

	err := database.WithServiceJob(ctx, writes, seedJob, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM app.protocols WHERE patient_id = $1 AND status = 'active'
			)
		`, string(patient)).Scan(&held)
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("looking for a course already prescribed: %w", err)
	}

	return held, nil
}
