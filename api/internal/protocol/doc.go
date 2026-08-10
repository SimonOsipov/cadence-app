// Package protocol owns the prescription: what is taken, when, how much, and up to
// which dose the patient is titrated.
//
// One of the eleven bounded contexts of the API: it calls its neighbours
// through their service functions and never reads their tables.
package protocol
