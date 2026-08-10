// Package inventory owns vials, the medicine cabinet, and the remaining volume
// derived from the doses logged against each vial.
//
// One of the eleven bounded contexts of the API: it calls its neighbours
// through their service functions and never reads their tables.
package inventory
