package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// StaffMember is one of the clinic's own, as a colleague needs them: an identifier to put on a care
// team and the words a patient will read beside the name.
type StaffMember struct {
	UserID   string  `json:"user_id" doc:"The identifier a care-team assignment names."`
	FullName string  `json:"full_name" doc:"The name as the clinic wrote it."`
	Role     string  `json:"role" doc:"doctor or admin."`
	TitleRU  *string `json:"title_ru" doc:"What a patient sees beside the name. Absent when the clinic stated none."`
}

// Directory answers who works at the clinic.
//
// Through the service seam, and that is the whole design decision here: no policy lets a doctor read a
// colleague's profile — profiles_own_select is their own row and profiles_of_my_patients is their
// patients — so the authorization for this read is in Go, above, exactly as it is for creating a
// person. What it authorizes is narrow: staff only, and the answer carries no address, no timezone and
// nobody who is not staff.
type Directory struct {
	writes *pgxpool.Pool
}

// NewDirectory builds the service over the service pool; a nil pool yields a nil service, which the
// handler refuses on.
func NewDirectory(writes *pgxpool.Pool) *Directory {
	if writes == nil {
		return nil
	}

	return &Directory{writes: writes}
}

// Staff answers the clinic's doctors and administrators, by name.
func (d *Directory) Staff(ctx context.Context) ([]StaffMember, error) {
	staff := []StaffMember{}

	err := database.WithService(ctx, d.writes, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT p.user_id, p.full_name, p.role, pp.title_ru
			FROM app.profiles p
			LEFT JOIN app.provider_profiles pp ON pp.user_id = p.user_id
			WHERE p.role IN ('doctor', 'admin')
			ORDER BY p.full_name, p.user_id
		`)
		if err != nil {
			return fmt.Errorf("reading the staff: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var member StaffMember
			if err := rows.Scan(&member.UserID, &member.FullName, &member.Role, &member.TitleRU); err != nil {
				return fmt.Errorf("reading staff row %d: %w", len(staff), err)
			}

			staff = append(staff, member)
		}

		return rows.Err()
	})
	if err != nil {
		if database.IsUnavailable(err) {
			return nil, fmt.Errorf("reading the staff: %w: %w", ErrDatabaseUnavailable, err)
		}

		return nil, fmt.Errorf("reading the staff: %w", err)
	}

	return staff, nil
}

// StaffOutput is the answer of GET /v1/providers.
type StaffOutput struct {
	Body StaffPage
}

// StaffPage is not paginated and says so by its shape: a clinic's staff is tens of people, and a
// cursor would be a page marker nobody ever sends.
type StaffPage struct {
	Staff []StaffMember `json:"staff" nullable:"false" doc:"Everyone who may be put on a care team, ordered by name."`
}

func (s *Service) staff(ctx context.Context, _ *struct{}) (*StaffOutput, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("no verified principal on the request context")
	}

	// The closed set, and the patient arm is the reason this route exists at all: a patient's care
	// team is answered to them elsewhere, by the context that owns the screen it is drawn on.
	switch principal.Role {
	case providerRole, adminRole:
	case patientRole:
		return nil, huma.Error403Forbidden(detailStaffIsNotForPatients)
	default:
		return nil, huma.Error403Forbidden(detailNoRole)
	}

	if s.directory == nil {
		return nil, huma.Error500InternalServerError("this API was assembled without a directory service")
	}

	staff, err := s.directory.Staff(ctx)
	if err != nil {
		if errors.Is(err, ErrDatabaseUnavailable) {
			return nil, huma.Error503ServiceUnavailable("the database could not serve the request", err)
		}

		return nil, huma.Error500InternalServerError("reading the staff", err)
	}

	return &StaffOutput{Body: StaffPage{Staff: staff}}, nil
}

const detailStaffIsNotForPatients = "Список сотрудников доступен только сотрудникам клиники."
