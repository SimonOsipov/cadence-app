// Package audit owns the append-only log of clinical changes.
//
// One of the eleven bounded contexts of the API: it calls its neighbours
// through their service functions and never reads their tables.
package audit
