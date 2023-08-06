package sway

import (
	"fmt"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

// helper command, invokes swaycmd
func swaycmd(args ...string) ([]byte, error) {
	out, err := exec.Command("swaymsg", args...).Output()
	log := log.WithFields(log.Fields{
		"out":  string(out),
		"name": "swaymsg",
		"args": args,
	})
	if err != nil {
		// keep this log statement, so we see the output somewhere.
		// callsites usually discard it without logging output.
		log.Debug("failed running swaymsg")
		return out, fmt.Errorf("failed running swaymsg: %w", err)
	}
	log.Debug("ran swaymsg")
	return out, nil
}

// helper command, invokes `swaycmd output $output ...args`
func (o *Output) configure(args ...string) error {
	fullArgs := []string{"output", o.Name}
	fullArgs = append(fullArgs, args...)
	_, err := swaycmd(fullArgs...)
	return err
}

func (o *Output) focusWorkspace() error {
	// select workspace, and make sure it's created.
	if _, err := swaycmd("workspace", o.Name); err != nil {
		return fmt.Errorf("Unable to select workspace: %w", err)
	}
	// pin workspace to output
	if _, err := swaycmd("workspace", o.Name, "output", o.Name); err != nil {
		return fmt.Errorf("Unable to pin workspace to output: %w", err)
	}
	return nil
}

func (o *Output) empty() error {
	swaycmd(fmt.Sprintf("[workspace=\"%v\"]", o.Name), "kill")
	// TODO: this seems to exit 2 if it couldn't find anything?
	return nil
}

// Run the given command on the screen.
func (o *Output) runCommand(cmd string) error {
	if err := o.focusWorkspace(); err != nil {
		return fmt.Errorf("unable to focus workspace: %w", err)
	}

	if len(cmd) != 0 {
		// start the process on the workspace
		if _, err := swaycmd(fmt.Sprintf("exec %s", cmd)); err != nil {
			return fmt.Errorf("unable to execute command: %w", err)
		}
	}

	// TODO: wait for a window to have appeared

	return nil
}
