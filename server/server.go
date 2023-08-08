package server

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/flokli/display-agent/mqtt"
	"github.com/flokli/display-agent/outputs"
	"github.com/flokli/display-agent/outputs/sway"
	log "github.com/sirupsen/logrus"

	"github.com/coreos/go-systemd/daemon"
)

type Server struct {
	MachineID   string
	TopicPrefix string
	mqttClient  pahomqtt.Client
	swayConn    *sway.Sway

	muNumOutputs sync.Mutex
	numOutputs   uint
}

func New(machineID string, topicPrefix string) *Server {
	return &Server{
		MachineID:   machineID,
		TopicPrefix: topicPrefix,
		numOutputs:  0,
	}
}

func (s *Server) Close() {
	log.Debug("closing swayConn")
	s.swayConn.Close()
}

func (s *Server) Run(ctx context.Context, mqttServerURL string) error {
	// setup mqtt
	mqttClient, err := mqtt.Connect(mqttServerURL)
	if err != nil {
		log.Error("unable to connect to MQTT")
		return fmt.Errorf("unable to connect to mqtt: %w", err)
	}
	s.mqttClient = mqttClient

	log.WithFields(log.Fields{
		"machineID":   s.MachineID,
		"topicPrefix": s.TopicPrefix,
	}).Info("Server started")

	swayConn := sway.New(ctx, 1*time.Second)
	s.swayConn = swayConn

	// what to do if there's a new output.
	swayConn.RegisterOutputAdd(func(output outputs.Output) {
		firstNewOutput := false
		s.muNumOutputs.Lock()
		// If we previously had no outputs and now have one, mark as ready.
		if s.numOutputs == 0 {
			firstNewOutput = true
		}
		s.numOutputs = s.numOutputs + 1
		s.muNumOutputs.Unlock()

		outputName := *output.GetInfo().Name
		l := log.WithField("outputName", outputName)

		// subscribe to the MQTT set topic
		topic := s.getTopicPrefixForOutputName(outputName) + "/set"
		err := mqtt.Subscribe(s.mqttClient, topic, 0, func(c pahomqtt.Client, m pahomqtt.Message) {
			l := l.WithFields(log.Fields{
				"message_id": m.MessageID(),
				"payload":    m.Payload(),
				"topic":      topic,
			})
			l.Debug("received message")

			if m.Topic() != topic {
				// This should only happen if the broker sends us unsolicited messages,
				// and/or the client doesn't properly route them to the right callbacks.
				log.Warn("discarded unrelated message")
				return
			}

			if err := handleSetCmd(m.Payload(), output); err != nil {
				log.WithError(err).Error("unable to handle setCmd")
			}

		})
		if err != nil {
			l.WithField("topic", topic).WithError(err).Error("unable to subscribe to set topic")
		}

		// mark as ready if this was the first output for which we published state, info
		// and subscribed to the set topic.
		if firstNewOutput {
			daemon.SdNotify(false, daemon.SdNotifyReady)
		}

		if err := s.publishOutputData(output); err != nil {
			log.WithError(err).Warn("unable to publish output data")
		} else {
			daemon.SdNotify(false, "WATCHDOG=1")
		}
	})

	swayConn.RegisterOutputUpdate(func(output outputs.Output) {
		if err := s.publishOutputData(output); err != nil {
			log.WithError(err).Warn("unable to publish output data")
		} else {
			daemon.SdNotify(false, "WATCHDOG=1")
		}
	})

	// what to do if the output is removed
	swayConn.RegisterOutputRemove(func(output outputs.Output) {
		s.muNumOutputs.Lock()
		s.numOutputs = s.numOutputs - 1
		s.muNumOutputs.Unlock()

		outputName := *output.GetInfo().Name
		l := log.WithField("outputName", outputName)
		// unsubscribe from the MQTT set topic
		err := mqtt.Unsubscribe(s.mqttClient, []string{s.getTopicPrefixForOutputName(outputName) + "/set"})
		if err != nil {
			l.WithError(err).Warn("unable to unsubscribe")
		}

		// publish an empty string to the topics state and info
		if err := mqtt.Publish(s.mqttClient, s.getTopicPrefixForOutputName(outputName)+"/state", 0, false, []byte("{}")); err != nil {
			l.WithError(err).Warn("unable to publish empty string for state")
		}
		if err := mqtt.Publish(s.mqttClient, s.getTopicPrefixForOutputName(outputName)+"/info", 0, false, []byte("{}")); err != nil {
			l.WithError(err).Warn("unable to publish empty string for info")
		}
	})

	log.Info("server.Run() finished")

	return nil
}

// publishOutputData publishes all info about a given output to the mqtt broker.
func (s *Server) publishOutputData(output outputs.Output) error {
	state := output.GetState()
	info := output.GetInfo()

	topicPrefix := s.getTopicPrefixForOutputName(*info.Name)

	stateJSON, err := json.Marshal(&state)
	if err != nil {
		return fmt.Errorf("unable to marshal state json: %w", err)
	}

	infoJSON, err := json.Marshal(&info)
	if err != nil {
		return fmt.Errorf("unable to marshal info json: %w", err)
	}

	if err := mqtt.Publish(s.mqttClient, topicPrefix+"/state", 0, false, string(stateJSON)); err != nil {
		return fmt.Errorf("unable to publish state: %w", err)
	}
	if err := mqtt.Publish(s.mqttClient, topicPrefix+"/info", 0, false, string(infoJSON)); err != nil {
		return fmt.Errorf("unable to publish info: %w", err)
	}

	return nil
}

// decode the mqtt set command and update the output.
func handleSetCmd(payload []byte, output outputs.Output) error {
	// Parse payload into (sparse) state
	var setState *outputs.State
	if err := json.Unmarshal(payload, &setState); err != nil {
		return fmt.Errorf("failed to parse set payload: %w", err)
	}

	// Dedup settings that are already set the way they should be.
	currentState := output.GetState()

	if setState.Enabled != nil && *setState.Enabled == *currentState.Enabled {
		setState.Enabled = nil
	}
	if setState.Mode != nil && *setState.Mode == *currentState.Mode {
		setState.Mode = nil
	}
	if setState.Power != nil && *setState.Power == *currentState.Power {
		setState.Power = nil
	}
	if setState.Scale != nil && *setState.Scale == *currentState.Scale {
		setState.Scale = nil
	}
	if setState.Transform != nil && *setState.Transform == *currentState.Transform {
		setState.Scale = nil
	}
	if setState.Scenario != nil {
		if setState.Scenario.Name == currentState.Scenario.Name && reflect.DeepEqual(setState.Scenario.Args, currentState.Scenario.Args) {
			setState.Scenario = nil
		}
	}

	output.SetState(setState)

	return nil
}

func (s *Server) getTopicPrefixForOutputName(name string) string {
	return s.TopicPrefix + "/" + name + "@" + s.MachineID
}
