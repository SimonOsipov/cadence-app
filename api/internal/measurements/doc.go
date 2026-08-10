// Package measurements owns biomarkers, body measurements and weight — one table for
// every kind of measured value.
//
// One of the eleven bounded contexts of the API: it calls its neighbours
// through their service functions and never reads their tables.
package measurements
