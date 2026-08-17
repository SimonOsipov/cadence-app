package identity

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// ErrNotACursor is returned for a page marker this server did not issue.
var ErrNotACursor = errors.New("the cursor is not one this server issued")

// ErrNotForPatients is returned for a patient asking for the roster: an empty page would be
// indistinguishable from a breakage, and the account is not one this screen exists for.
var ErrNotForPatients = errors.New("the roster is not a patient's to read")

const (
	detailRosterIsNotForPatients = "Реестр пациентов доступен только сотрудникам клиники."
	detailNotACursor             = "Страница не найдена. Откройте реестр заново."
)

// The pair the ordering is keyed on travels in one opaque token. It is base64 rather than two query
// parameters because it is the server's to shape: a client that could assemble one would be choosing
// where a page starts, and the next version of the ordering would break every bookmark.
const cursorSeparator = "\x00"

func makeCursor(fullName, userID string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fullName + cursorSeparator + userID))
}

// readCursor returns the pair to continue after. The empty cursor is the first page rather than a
// malformed one — it is what a client sends before it has been given anything to continue from.
func readCursor(cursor string) (fullName, userID string, err error) {
	if cursor == "" {
		return "", "", nil
	}

	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", "", fmt.Errorf("%q: %w", cursor, ErrNotACursor)
	}

	// The last separator and not the first: a name may contain one and a UUID may not, so everything
	// before the last is the name.
	at := strings.LastIndex(string(decoded), cursorSeparator)
	if at < 0 {
		return "", "", fmt.Errorf("%q: %w", cursor, ErrNotACursor)
	}

	fullName, userID = string(decoded[:at]), string(decoded[at+len(cursorSeparator):])

	if !database.IsUUIDShaped(userID) {
		return "", "", fmt.Errorf("%q: %w", cursor, ErrNotACursor)
	}

	return fullName, userID, nil
}

// Roster reads the patients the caller is allowed to see.
type Roster struct {
	pool *pgxpool.Pool
}

func NewRoster(pool *pgxpool.Pool) *Roster {
	if pool == nil {
		return nil
	}

	return &Roster{pool: pool}
}

// RosterRow is one patient as the Overview's registry draws them in v0. Flags, status, adherence and
// the sparkline are M6 and extend this same route rather than a second one.
type RosterRow struct {
	UserID   string `json:"user_id" doc:"The patient's id, and the key every later request about them is made on."`
	FullName string `json:"full_name" doc:"The patient's name as the clinic wrote it."`
	Age      *int   `json:"age" doc:"Years, worked out by the server. Absent when the clinic has not entered a date of birth."`
}

// Page is a page of the roster and the marker to ask for the next one.
type Page struct {
	Patients []RosterRow `json:"patients" doc:"The patients this caller may see, ordered by name."`
	Next     string      `json:"next_cursor,omitempty" doc:"Pass as cursor for the following page. Absent on the last one."`
}

// Patients answers the page after the cursor.
//
// No predicate on the doctor: profiles_of_my_patients selects, and a condition here would be a second
// source of truth beside it. The one predicate that is not that: role = 'patient', because a doctor
// reads their own row through profiles_own_select and would otherwise appear in their own roster.
func (r *Roster) Patients(ctx context.Context, caller database.Caller, cursor string, limit int) (Page, error) {
	afterName, afterID, err := readCursor(cursor)
	if err != nil {
		return Page{}, err
	}

	var page Page

	err = database.WithCaller(ctx, r.pool, caller, func(ctx context.Context, tx pgx.Tx) error {
		// Row-value comparison rather than two conditions: it is the ordering written once, and it is
		// what the index on (full_name, user_id) is read by. The first page passes the empty pair,
		// which every row is greater than.
		rows, err := tx.Query(ctx, `
			SELECT p.user_id, p.full_name,
			       date_part('year', pg_catalog.age(pp.dob))::int AS age
			FROM app.profiles p
			LEFT JOIN app.patient_profiles pp ON pp.user_id = p.user_id
			WHERE p.role = 'patient' AND (p.full_name, p.user_id::text) > ($1, $2)
			ORDER BY p.full_name, p.user_id
			LIMIT $3
		`, afterName, afterID, limit+1)
		if err != nil {
			return fmt.Errorf("reading the roster: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var row RosterRow
			if err := rows.Scan(&row.UserID, &row.FullName, &row.Age); err != nil {
				return fmt.Errorf("reading a roster row: %w", err)
			}

			page.Patients = append(page.Patients, row)
		}

		return rows.Err()
	})
	if err != nil {
		if database.IsUnavailable(err) {
			return Page{}, fmt.Errorf("reading the roster for %s: %w: %w", caller.Subject, ErrDatabaseUnavailable, err)
		}

		return Page{}, fmt.Errorf("reading the roster for %s: %w", caller.Subject, err)
	}

	// One row more than the page was asked for is how «there is a next page» is known without a second
	// count: a count over a policy-filtered set is a second query answering about a set that may have
	// changed between the two.
	if len(page.Patients) > limit {
		last := page.Patients[limit-1]
		page.Patients = page.Patients[:limit]
		page.Next = makeCursor(last.FullName, last.UserID)
	}

	return page, nil
}
