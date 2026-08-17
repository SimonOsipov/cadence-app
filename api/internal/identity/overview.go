package identity

import (
	"context"
	"errors"

	"github.com/danielgtaylor/huma/v2"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// OverviewInput is the query of GET /v1/dashboard/overview.
type OverviewInput struct {
	Cursor string `query:"cursor" maxLength:"512" doc:"Where to continue from, as the previous page's next_cursor. Absent asks for the first page."`
	Limit  int    `query:"limit" minimum:"1" maximum:"100" default:"8" doc:"How many patients the page carries."`
}

// OverviewOutput is the roster and nothing else yet. The other five sections of the screen — the stats
// strip, the triage queue, today's schedule, the patient card and the side menu — stay on the
// dashboard's fixtures until M6, which extends this same route rather than adding a second one.
type OverviewOutput struct {
	Body Page
}

func (s *Service) overview(ctx context.Context, input *OverviewInput) (*OverviewOutput, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("no verified principal on the request context")
	}

	// Refused here rather than left to the policies. A patient's own policies select nothing on this
	// query, so the database would answer 200 with an empty page — and an empty roster is what a
	// doctor with no patients also sees, so the one answer would mean two things.
	if principal.Role == patientRole {
		return nil, refusalForRoster(ErrNotForPatients)
	}

	if s.roster == nil {
		return nil, huma.Error500InternalServerError("this API was assembled without a roster service")
	}

	caller := database.Caller{Subject: principal.Subject, Role: principal.Role}

	page, err := s.roster.Patients(ctx, caller, input.Cursor, input.Limit)
	if err != nil {
		return nil, refusalForRoster(err)
	}

	return &OverviewOutput{Body: page}, nil
}

func refusalForRoster(err error) error {
	switch {
	case errors.Is(err, ErrNotForPatients):
		return huma.Error403Forbidden(detailRosterIsNotForPatients)

	case errors.Is(err, ErrNotACursor):
		return huma.Error400BadRequest(detailNotACursor)

	case errors.Is(err, ErrDatabaseUnavailable):
		return huma.Error503ServiceUnavailable("the database could not serve the request", err)

	default:
		return huma.Error500InternalServerError("reading the roster", err)
	}
}
