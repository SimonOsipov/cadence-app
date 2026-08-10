// Package dosing owns the stream of clinical facts: doses the patient logged.
//
// One of the eleven bounded contexts of the API: it calls its neighbours
// through their service functions and never reads their tables.
package dosing
