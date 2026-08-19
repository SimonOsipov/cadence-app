package identity

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// ErrNotACursor is returned for a page marker this server did not issue.
var ErrNotACursor = errors.New("the cursor is not one this server issued")

// ErrNotAPageSize is returned for a page of none or fewer, and for one larger than MaxPageSize.
var ErrNotAPageSize = errors.New("a page carries between one row and MaxPageSize of them")

// MaxPageSize is the largest page the roster answers, and the route's schema pins the same number.
//
// Bounded from above because the page's invitation states come from one lookup at the identity
// provider, and that lookup carries at most provisioning.MaxLookupBatch identifiers: a page past it
// is a page whose states all come back unknown. The two are compared by a test in the package that
// holds the other constant — this one may not import it, and the number is the component's.
const MaxPageSize = 100

// ErrNotForPatients is returned for a patient asking for the roster: an empty page would be
// indistinguishable from a breakage, and the account is not one this screen exists for.
var ErrNotForPatients = errors.New("the roster is not a patient's to read")

const (
	detailRosterIsNotForPatients = "Реестр пациентов доступен только сотрудникам клиники."
	detailNotACursor             = "Страница не найдена. Откройте реестр заново."
	detailNotAPageSize           = "Размер страницы вне допустимых значений."
)

// Encoded, not signed: a client can assemble one, and RLS is what keeps a forged start harmless.
const cursorSeparator = "\x00"

// maxCursorLength is the longest makeCursor can emit — 200 four-byte characters, a separator and a
// UUID, base64 — and the number the route's schema pins: a smaller bound refuses this server's own cursor.
const maxCursorLength = 1116

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

	// The last separator and not the first. Not because a stored name can contain one — text may not
	// hold NUL, measured against PostgreSQL 17 — but because this decodes bytes off the wire.
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

// Roster reads the patients the caller is allowed to see, with the state of
// each one's invitation.
type Roster struct {
	pool        *pgxpool.Pool
	provisioner Provisioner
}

// NewRoster builds the service over the request pool; a nil pool yields a nil
// service, which the handler refuses on. A nil provisioner is not refused: the
// page is what the caller came for, and it renders with the states unknown.
func NewRoster(pool *pgxpool.Pool, provisioner Provisioner) *Roster {
	if pool == nil {
		return nil
	}

	return &Roster{pool: pool, provisioner: provisioner}
}

// RosterRow is one patient as the registry draws them in v0. Flags, adherence, sparklines and lastSeen
// are the spec's Non-scope, and M6 extends this same type rather than a second route.
type RosterRow struct {
	UserID   string `json:"user_id" doc:"The patient's id, and the key every later request about them is made on."`
	FullName string `json:"full_name" doc:"The patient's name as the clinic wrote it."`
	Age      *int   `json:"age" doc:"Years, worked out by the server. Absent when the clinic has not entered a date of birth."`

	Invite InviteState `json:"invite_state" enum:"accepted,pending,expired,unknown" doc:"Where the patient is between the invitation and an account they use. Unknown when the identity provider could not be asked."`
}

// RosterPage is a page of the roster and the marker to ask for the next one. Named for its context
// because the OpenAPI document has one namespace for eleven of them, and huma panics on a collision.
type RosterPage struct {
	// nullable false and a slice that is never nil: a doctor with no patients is an ordinary state, and
	// two encodings of «none» on the wire is what the 403 above exists to avoid on the status line.
	Patients []RosterRow `json:"patients" nullable:"false" doc:"The patients this caller may see, ordered by name."`
	Next     string      `json:"next_cursor,omitempty" doc:"Pass as cursor for the following page. Absent on the last one."`
}

// Patients answers the page after the cursor. No predicate on the doctor — profiles_of_my_patients
// selects — and one on the role, because profiles_own_select would otherwise put a doctor in their
// own roster.
func (r *Roster) Patients(ctx context.Context, caller database.Caller, cursor string, limit int) (RosterPage, error) {
	// Refused rather than clamped: the arithmetic below indexes at limit-1, and this method is
	// exported past the schema that pins both of the route's bounds.
	if limit < 1 || limit > MaxPageSize {
		return RosterPage{}, fmt.Errorf("%d: %w", limit, ErrNotAPageSize)
	}

	afterName, afterID, err := readCursor(cursor)
	if err != nil {
		return RosterPage{}, err
	}

	page := RosterPage{Patients: []RosterRow{}}

	err = database.WithCaller(ctx, r.pool, caller, func(ctx context.Context, tx pgx.Tx) error {
		// The parameter is cast to uuid and not the column to text: measured with EXPLAIN, the latter
		// drops the second key out of the index condition. The first page's empty pair becomes the
		// smallest uuid rather than NULL — a NULL second element answers NULL for any row whose name
		// ties with '', and that row would leave the first page with no error to show for it.
		rows, err := tx.Query(ctx, `
			SELECT p.user_id, p.full_name,
			       date_part('year', pg_catalog.age(pp.dob))::int AS age
			FROM app.profiles p
			LEFT JOIN app.patient_profiles pp ON pp.user_id = p.user_id
			WHERE p.role = 'patient' AND (p.full_name, p.user_id) > ($1, coalesce(nullif($2, '')::uuid, '00000000-0000-0000-0000-000000000000'::uuid))
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
				return fmt.Errorf("reading roster row %d after %q: %w", len(page.Patients), cursor, err)
			}

			page.Patients = append(page.Patients, row)
		}

		return rows.Err()
	})
	if err != nil {
		if database.IsUnavailable(err) {
			return RosterPage{}, fmt.Errorf("reading the roster for %s: %w: %w", caller.Subject, ErrDatabaseUnavailable, err)
		}

		return RosterPage{}, fmt.Errorf("reading the roster for %s: %w", caller.Subject, err)
	}

	// One row over the page is how a next page is known without a second count over a set that moves.
	if len(page.Patients) > limit {
		last := page.Patients[limit-1]
		page.Patients = page.Patients[:limit]
		page.Next = makeCursor(last.FullName, last.UserID)
	}

	// After the page is cut, not before: the extra row is not on the screen and
	// looking it up would spend a share of the batch on a patient nobody sees.
	r.fillInviteStates(ctx, page.Patients, time.Now())

	return page, nil
}

// fillInviteStates asks the identity provider about the page and writes what it
// says onto each row.
//
// It returns nothing. A provisioner that is down costs the states and not the
// roster: a doctor reading names with no invitation state can still work, and an
// error here would answer them an empty screen instead.
func (r *Roster) fillInviteStates(ctx context.Context, rows []RosterRow, now time.Time) {
	for i := range rows {
		rows[i].Invite = InviteUnknown
	}

	if r.provisioner == nil || len(rows) == 0 {
		return
	}

	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.UserID)
	}

	accounts, err := r.provisioner.LookupBatch(ctx, ids)
	if err != nil {
		return
	}

	// By identifier and not by position: the component omits an identifier it
	// holds no account for, so the nth account is not the nth row.
	found := make(map[string]Account, len(accounts))
	for _, account := range accounts {
		found[account.ID] = account
	}

	for i := range rows {
		account, ok := found[rows[i].UserID]
		if !ok {
			continue
		}

		rows[i].Invite = inviteStateOf(&account, now)
	}
}
