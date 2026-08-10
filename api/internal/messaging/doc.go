// Package messaging owns threads, messages, structured cards, and the WebSocket
// registry that delivers them.
//
// One of the eleven bounded contexts of the API: it calls its neighbours
// through their service functions and never reads their tables.
package messaging
