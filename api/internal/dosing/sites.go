package dosing

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

// Sites is every zone, and the order is load-bearing rather than cosmetic: it is the
// rotation's tie-break, so it decides what a patient with no history is offered and what
// they are offered when two zones are equally stale.
//
// It is the KMP enum's order — the same 000019's CHECK lists — and deliberately not the
// prototype's drawing order, which would move the opening suggestion from the left abdomen
// to the right deltoid.
func Sites() []Site {
	return []Site{
		SiteLeftAbdomen, SiteRightAbdomen,
		SiteLeftDeltoid, SiteRightDeltoid,
		SiteLeftGlute, SiteRightGlute,
		SiteLeftThigh, SiteRightThigh,
		SiteLeftLowBack, SiteRightLowBack,
	}
}
