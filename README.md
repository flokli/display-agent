# display-agent

Daemon useful to manage orientation, resolution and payload of one or multiple
screens over MQTT.

## MQTT Topics

For each connected output, the server (periodically) publishes to the following
topics:

 - `$topicPrefix/$outputName@$machineID/state`
    contains the current configuration (display layout, currently active
    scenario).
 - `$topicPrefix/$outputName@$machineID/info`
    Info contains more general information about the connected display (make,
    model, serial, supported modes).


Check `outputs/type.go` for an exhaustive list of the fields.

They're published JSON-encoded.

The server listens on the following topics:

 - `$topicPrefix/$outputName@$machineID/set`

If a message is published to that topic, it is parsed as a (sparse) `state`,
containing all fields that should be updated in the current state.

## Backends

The server currently only supports Sway as a backend, by invoking `swaymsg`.

PRs for other backends welcome! In case you're stuck with X, adding support
for i3 should probably be easiest (as the JSON `i3-msg` can emit should be
fairly similar).