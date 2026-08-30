package protocol

import (
	"slices"
	"strings"
)

// latestCourse is the patient's last course: the running one if there is one, else the one
// that started latest, and on a tie the one written latest, and on a tie the greater id.
//
// The running course comes first on purpose rather than by accident of sorting. `start_date`
// has no upper bound — a doctor may write a course that begins next month — so «latest by
// start» alone would hand back a future or a closed course while the patient is on another
// one. `protocols_one_active_per_patient` is what makes «the running one» a single row.
//
// Status is asked here, and only here: the callers that build a window off the answer ask
// about its geometry instead, so a cancelled course still has an axis.
func latestCourse(courses []Protocol) (Protocol, bool) {
	if len(courses) == 0 {
		return Protocol{}, false
	}

	return slices.MaxFunc(courses, compareCourses), true
}

// The key written out, rung by rung. Total, because the last rung is the primary key: two
// courses can share a start date and — inserted in one transaction, where now() is the
// transaction's — the same created_at, and «whichever the database handed back first» is not
// an answer a screen can be built on.
func compareCourses(a, b Protocol) int {
	if running := boolOrder(a.Status == StatusActive) - boolOrder(b.Status == StatusActive); running != 0 {
		return running
	}
	// Both directions through Before, and never `!=`: a Date the calendar does not have
	// compares unequal by field while Before calls it the same day, and a comparator that
	// answers 1 both ways round stops being an order at all.
	if a.StartDate.Before(b.StartDate) {
		return -1
	}
	if b.StartDate.Before(a.StartDate) {
		return 1
	}
	if a.CreatedAt.Before(b.CreatedAt) {
		return -1
	}
	if b.CreatedAt.Before(a.CreatedAt) {
		return 1
	}

	return strings.Compare(string(a.ID), string(b.ID))
}

func boolOrder(b bool) int {
	if b {
		return 1
	}

	return 0
}
