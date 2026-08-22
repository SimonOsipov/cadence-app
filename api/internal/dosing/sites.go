package dosing

import "github.com/SimonOsipov/cadence-app/api/internal/journal"

// Site is the body zone an injection went into, and the code is what
// dose_events.site_code stores.
//
// §03 names three of them and writes «(10 zones)»; the rest are the body map the
// frozen prototype draws (mobile/src/features/log-dose/data.ts, ZONES_FRONT and
// ZONES_BACK — six in front, four behind). Read from there rather than invented:
// this project has twice shipped an enum whose values were guessed and whose test
// counted entries instead of naming them.
type Site string

const (
	SiteLeftAbdomen  Site = "l-abdomen"
	SiteRightAbdomen Site = "r-abdomen"
	SiteLeftDeltoid  Site = "l-delt"
	SiteRightDeltoid Site = "r-delt"
	SiteLeftGlute    Site = "l-glute"
	SiteRightGlute   Site = "r-glute"
	SiteLeftThigh    Site = "l-thigh"
	SiteRightThigh   Site = "r-thigh"
	SiteLeftLowBack  Site = "l-lback"
	SiteRightLowBack Site = "r-lback"
)

// Sites is every zone the body map can draw, and the order is the prototype's:
// the front six as it lists them, then the back four.
func Sites() []Site {
	return []Site{
		SiteRightDeltoid, SiteLeftDeltoid,
		SiteRightAbdomen, SiteLeftAbdomen,
		SiteRightThigh, SiteLeftThigh,
		SiteLeftLowBack, SiteRightLowBack,
		SiteLeftGlute, SiteRightGlute,
	}
}

// SideEffect is journal's tag under the name §03 gives it here. One set, not two:
// the same seven reach the diary and the dose event from one patient action, and a
// second declaration is a second thing to forget to change.
type SideEffect = journal.Tag
