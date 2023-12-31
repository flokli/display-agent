package sway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"sync"

	// "sync"
	"time"

	"github.com/flokli/display-agent/outputs"
	log "github.com/sirupsen/logrus"
)

// Sway contains all sway-wide state
type Sway struct {
	outputs   map[string]*Output
	outputsMu sync.Mutex

	refreshTicker *time.Ticker

	// Called when the output appeared
	onAddFns []func(outputs.Output)
	// Called when the output was updated
	onUpdateFns []func(outputs.Output)
	// Called when the output was removed
	onRemoveFns []func(outputs.Output)
}

func New(ctx context.Context, refreshInterval time.Duration) *Sway {
	s := &Sway{
		outputs:       make(map[string]*Output),
		refreshTicker: time.NewTicker(refreshInterval),
	}

	go func() {
		for {
			select {
			case <-s.refreshTicker.C:
				if err := s.refreshOutputs(); err != nil {
					log.WithError(err).Error("Failed to refresh outputs")
				}
			case <-ctx.Done():
				for outputName, output := range s.outputs {
					log.WithField("outputName", outputName).Debug("calling cleanup handlers")
					for _, removeFn := range s.onRemoveFns {
						removeFn(output)
					}
				}
			}

		}
	}()

	return s
}

func (s *Sway) Close() {
	log.Debug("stopping refresh ticker")
	s.refreshTicker.Stop()
}

// Register a new handler for when an output was added
func (o *Sway) RegisterOutputAdd(fn func(outputs.Output)) {
	o.onAddFns = append(o.onAddFns, fn)
}

// Register a new handler for when an output was updated
func (o *Sway) RegisterOutputUpdate(fn func(outputs.Output)) {
	o.onUpdateFns = append(o.onUpdateFns, fn)
}

// Register a new handler for when an output was removed
func (s *Sway) RegisterOutputRemove(fn func(outputs.Output)) {
	s.onRemoveFns = append(s.onRemoveFns, fn)
}

// Invoke `swaymsg -t get_outputs` and sync the state observed from there with
// the internal state in all outputs. Afterwards, return all (updated) outputs.
func (s *Sway) refreshOutputs() error {
	l := log.WithField("f", "refreshOutputs")
	l.Debug("refreshing outputs")
	s.outputsMu.Lock()
	l.Debug("acquired lock")
	defer s.outputsMu.Unlock()

	out, err := exec.Command("swaymsg", "-t", "get_outputs").Output()
	if err != nil {
		return fmt.Errorf("Failed to invoke swaymsg: %w", err)
	}

	var newOutputs []*Output

	if err := json.Unmarshal(out, &newOutputs); err != nil {
		return fmt.Errorf("Failed to invoke swaymsg: %w", err)
	}

	// loop over all outputs returned
	seenOutputNames := make(map[string]interface{}, len(newOutputs))

	for _, newOutput := range newOutputs {
		outputName := newOutput.Name
		l := log.WithField("outputName", outputName)

		seenOutputNames[outputName] = nil

		// the output already exists…
		if oldOutput, old := s.outputs[outputName]; old {
			// update attributes with the new values.
			oldOutput.Active = newOutput.Active
			oldOutput.CurrentMode = newOutput.CurrentMode
			oldOutput.Make = newOutput.Make
			oldOutput.Model = newOutput.Model
			oldOutput.Modes = newOutput.Modes
			oldOutput.Name = newOutput.Name
			oldOutput.Power = newOutput.Power
			oldOutput.Scale = newOutput.Scale
			oldOutput.Serial = newOutput.Serial
			oldOutput.Transform = newOutput.Transform
			// keep Scenario, this can't be modified from sway

			// TODO: only notify if it's a new output
			l.Debug("calling update fns")
			for _, updateFn := range s.onUpdateFns {
				updateFn(&*oldOutput)
			}

		} else {
			// If the output didn't exist, insert into s.outputs.
			// add the pointer back to here, so the implementation can use it to acquire
			// a lock.
			newOutput.sway = s
			newOutput.Scenario = &outputs.Scenario{
				Name: "blank",
				Args: []string{},
			}
			s.outputs[outputName] = newOutput

			l.Debug("calling add fns")
			for _, addFn := range s.onAddFns {
				addFn(&*newOutput)
			}
		}
	}
	log.Debug("done looping over all outputs")

	// loop over all outputs in our global state, remove these that we didn't see.
	for prevOutputName, prevOutput := range s.outputs {
		if _, found := seenOutputNames[prevOutputName]; !found {
			delete(s.outputs, prevOutputName)

			log.Debug("calling delete fns")
			for _, removeFn := range s.onRemoveFns {
				removeFn(&*prevOutput)
			}
		}
	}

	return nil
}

type Output struct {
	// A handle to the global sway object
	sway *Sway

	Active      bool            `json:"active"`
	CurrentMode outputs.Mode    `json:"current_mode"`
	Make        string          `json:"make"`
	Model       string          `json:"model"`
	Modes       []*outputs.Mode `json:"modes"`
	Name        string          `json:"name"`
	Power       bool            `json:"power"`
	Scale       float64         `json:"scale"`
	Serial      string          `json:"serial"`
	Transform   string          `json:"transform"`

	Scenario *outputs.Scenario
}

// GetInfo implements Output.
func (o *Output) GetInfo() *outputs.Info {
	return &outputs.Info{
		Make:   &o.Make,
		Model:  &o.Model,
		Modes:  &o.Modes,
		Name:   &o.Name,
		Serial: &o.Serial,
	}
}

// GetState implements Output.
func (o *Output) GetState() *outputs.State {
	return &outputs.State{
		Enabled:   &o.Active,
		Mode:      &o.CurrentMode,
		Power:     &o.Power,
		Scale:     &o.Scale,
		Transform: &o.Transform,
		Scenario:  o.Scenario,
	}
}

func (o *Output) SetState(newState *outputs.State) (*outputs.State, error) {
	o.sway.outputsMu.Lock()
	defer o.sway.outputsMu.Unlock()

	log.WithFields(log.Fields{
		"newState.Enabled":   fmt.Sprintf("%v", newState.Enabled),
		"newState.Mode":      fmt.Sprintf("%v", newState.Mode),
		"newState.Power":     fmt.Sprintf("%v", newState.Power),
		"newState.Scale":     fmt.Sprintf("%v", newState.Scale),
		"newState.Transform": fmt.Sprintf("%v", newState.Transform),
		"newState.Scenario":  *newState.Scenario,
	}).Debug("SetState()")

	if newState.Enabled != nil {
		arg := ""
		if *newState.Enabled {
			arg = "enable"
		} else {
			arg = "disable"
		}
		if err := o.configure(arg); err != nil {
			return o.GetState(), fmt.Errorf("failed to set active: %w", err)
		}
	}
	if newState.Mode != nil {
		if err := o.configure("mode", newState.Mode.String()); err != nil {
			return o.GetState(), fmt.Errorf("failed to set mode: %w", err)
		}
	}
	if newState.Power != nil {
		arg := ""
		if *newState.Power {
			arg = "on"
		} else {
			arg = "off"
		}
		if err := o.configure("power", arg); err != nil {
			return o.GetState(), fmt.Errorf("failed to set power: %w", err)
		}
	}
	if newState.Scale != nil {
		if err := o.configure("scale", fmt.Sprintf("%v", *newState.Scale)); err != nil {
			return o.GetState(), fmt.Errorf("failed to set scale: %w", err)
		}
	}
	if newState.Transform != nil {
		if err := o.configure("transform", fmt.Sprintf("%v", *newState.Transform)); err != nil {
			return o.GetState(), fmt.Errorf("failed to set transform: %w", err)
		}
	}
	if newState.Scenario != nil {
		if err := o.setScenario(newState.Scenario.Name, newState.Scenario.Args); err != nil {
			return o.GetState(), fmt.Errorf("failed to set scenario: %w", err)
		}
	}

	return o.GetState(), nil
}

// SetScenario implements Output.
func (o *Output) setScenario(name string, args []string) error {
	log.WithFields(log.Fields{
		"scenario": name,
		"args":     args,
	}).Debug("SetScenario", name, args)

	// focus the workspace
	if err := o.focusWorkspace(); err != nil {
		return fmt.Errorf("unable to focus workspace: %w", err)
	}

	// kill all previous windows there, if present
	if err := o.empty(); err != nil {
		return fmt.Errorf("unable to empty workspace: %w", err)
	}

	if name == "url" {
		if len(args) != 1 {
			return fmt.Errorf("need to specify exactly 1 arg")
		}
		urlStr := args[0]

		// try to parse the URL
		u, err := url.Parse(urlStr)
		if err != nil {
			return fmt.Errorf("unable to parse URL")
		}

		o.runCommand("chromium --ozone-platform-hint=auto --app=" + u.String())
	} else if name == "blank" {
		// We killed all windows before, nothing to execute.
	} else if name == "video" {
		if len(args) != 1 {
			return fmt.Errorf("need to specify exactly 1 arg")
		}
		urlStr := args[0]

		// try to parse the URL
		u, err := url.Parse(urlStr)
		if err != nil {
			return fmt.Errorf("unable to parse URL")
		}

		o.runCommand("mpv --loop " + u.String())
	} else {
		return fmt.Errorf("scenario %v unimplemented", name)
	}

	// update the internal state
	o.Scenario = &outputs.Scenario{
		Name: name,
		Args: args,
	}

	return nil
}
