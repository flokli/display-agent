package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/flokli/display-agent/mqtt"
	"github.com/flokli/display-agent/outputs"
	"github.com/flokli/display-agent/outputs/sway"
	log "github.com/sirupsen/logrus"
)

type Server struct {
	MachineID   string
	TopicPrefix string
	mqttClient  pahomqtt.Client
	swayConn    *sway.Sway
}

func New(machineID string, topicPrefix string) *Server {
	return &Server{
		MachineID:   machineID,
		TopicPrefix: topicPrefix,
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
		s.publishOutputData(output)

		outputName := *output.GetInfo().Name

		// subscribe to the MQTT set topic
		topic := s.getTopicPrefixForOutputName(outputName) + "/set"
		l := log.WithField("topic", topic)
		l.Debug("subscribing")

		mqtt.Subscribe(s.mqttClient, topic, 0, func(c pahomqtt.Client, m pahomqtt.Message) {
			l := l.WithFields(log.Fields{
				"message_id": m.MessageID(),
				"payload":    m.Payload(),
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
	})

	swayConn.RegisterOutputUpdate(func(output outputs.Output) {
		s.publishOutputData(output)
	})

	// what to do if the output is removed
	swayConn.RegisterOutputRemove(func(output outputs.Output) {
		outputName := *output.GetInfo().Name

		// unsubscribe from the MQTT set topic
		mqtt.Unsubscribe(s.mqttClient, []string{s.getTopicPrefixForOutputName(outputName) + "/set"})

		// publish an empty string to the topics state and info
		mqtt.Publish(s.mqttClient, s.getTopicPrefixForOutputName(outputName)+"/state", 0, false, []byte("{}"))
		mqtt.Publish(s.mqttClient, s.getTopicPrefixForOutputName(outputName)+"/info", 0, false, []byte("{}"))
	})

	<-ctx.Done()
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

	output.SetState(setState)

	return nil
}

func (s *Server) getTopicPrefixForOutputName(name string) string {
	return s.TopicPrefix + "/" + name + "@" + s.MachineID
}
