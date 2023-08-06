package outputs

import ()

// State describes the current state of an output.
// it can be also used to set (some) options, in a /set request.
type State struct {
	Enabled   *bool     `json:"enabled"`
	Mode      *Mode     `json:"mode"`
	Power     *bool     `json:"power"`
	Scale     *float64  `json:"scale"`
	Transform *string   `json:"transform"`
	Scenario  *Scenario `json:"scenario"`
}

// Info describes some (fairly static) info about an output, such as the
// make/ model and available modes.
type Info struct {
	Make   *string  `json:"make"`
	Model  *string  `json:"model"`
	Modes  *[]*Mode `json:"modes"`
	Name   *string  `json:"name"`
	Serial *string  `json:"serial"`
}

type Output interface {
	// Getters
	GetInfo() *Info
	GetState() *State

	// Accepts a (partially populated) state object, and updates the underlying output.
	SetState(*State) (*State, error)
}

type Scenario struct {
	Name string   `json:"name"`
	Args []string `json:"args"`
}
