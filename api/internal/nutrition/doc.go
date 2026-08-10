// Package nutrition owns meals, recipes, macro goals, and the free-text meal
// parsing that turns a Russian sentence into editable items.
//
// One of the eleven bounded contexts of the API: it calls its neighbours
// through their service functions and never reads their tables.
package nutrition
