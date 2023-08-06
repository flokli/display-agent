# display-agent

Daemon useful to manage orientation, resolution and payload of one or multiple
screens over MQTT.

## MQTT Topics

For each connected output, the server (periodically) publishes to the following
topics:

 - `$topicPrefix/$outputName@$machineID/state`
 - `$topicPrefix/$outputName@$machineID/info`

State contains the current configuration (display layout, running scenario).
Info contains more general information about the connected display (Make, Model,
Serial, Supported Modes).

Check outputs/type.go for an exhaustive list of the fields. They're published
JSON-encoding.

The server listens on the following topics:

 - `$topicPrefix/$outputName@$machineID/set`

If a message is published to that topic that contains a subset of the fields
from `State`, these settings are applied (and the published state is updated
subsequently).

## Backends

The server currently only supports Sway as a backend, by invoking `swaymsg`.

PRs for other backends welcome! In case you're stuck with X, adding support
for i3 should probably be easiest (as the JSON `i3-msg` can emit should be
fairly similar).