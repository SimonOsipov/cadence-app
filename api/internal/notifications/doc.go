// Package notifications owns devices, push delivery, the reminder sweep and the log of
// what was sent.
//
// One of the eleven bounded contexts of the API: it calls its neighbours
// through their service functions and never reads their tables.
package notifications
