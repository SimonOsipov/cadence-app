package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/danielgtaylor/huma/v2"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// OverviewInput is the query of GET /v1/dashboard/overview.
type OverviewInput struct {
	Cursor string `query:"cursor" maxLength:"1116" doc:"Where to continue from, as the previous page's next_cursor. Absent asks for the first page."`
	Limit  int    `query:"limit" minimum:"1" maximum:"100" default:"8" doc:"How many patients the page carries."`
}

// OverviewOutput is the roster and nothing else yet. The other five sections of the screen — the stats
// strip, the triage queue, today's schedule, the patient card and the side menu — stay on the
// dashboard's fixtures until M6, which extends this same route rather than adding a second one.
type OverviewOutput struct {
	Body RosterPage
}

func (s *Service) overview(ctx context.Context, input *OverviewInput) (*OverviewOutput, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("no verified principal on the request context")
	}

	// A closed set, and the patient arm is not decoration: their own row passes both the policy and
	// role = 'patient', so without this a patient is answered a registry consisting of themselves.
	switch principal.Role {
	case providerRole, adminRole:
	case patientRole:
		return nil, refusalForRoster(ErrNotForPatients)
	default:
		// Invited and not provisioned, which the issuance hook and the verifier both call ordinary.
		return nil, refusalForRoster(fmt.Errorf("%q: %w", principal.Role, ErrNoRole))
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

	case errors.Is(err, ErrNoRole):
		return huma.Error403Forbidden(detailNoRole)

	case errors.Is(err, ErrNotACursor):
		return huma.Error400BadRequest(detailNotACursor)

	// Unreachable through the route, whose schema pins a minimum of one, and its own sentence anyway:
	// a caller past that schema is not a person who lost their place in a list.
	case errors.Is(err, ErrNotAPageSize):
		return huma.Error400BadRequest(detailNotAPageSize)

	case errors.Is(err, ErrDatabaseUnavailable):
		return huma.Error503ServiceUnavailable("the database could not serve the request", err)

	default:
		return huma.Error500InternalServerError("reading the roster", err)
	}
}
