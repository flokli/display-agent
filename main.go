package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/flokli/display-agent/server"
	log "github.com/sirupsen/logrus"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	log.SetLevel(log.DebugLevel)

	// get machine id
	machineID, err := GetMachineID()
	if err != nil {
		log.WithError(err).Error("Unable to get machine id")
		os.Exit(1)
	}

	// MQTT_SERVER_URL
	mqttServerUrl := os.Getenv("MQTT_SERVER_URL")
	if mqttServerUrl == "" {
		panic("MQTT_SERVER_URL must be set")
	}

	// MQTT_TOPIC_PREFIX
	mqttTopicPrefix := os.Getenv("MQTT_TOPIC_PREFIX")
	if mqttTopicPrefix == "" {
		panic("MQTT_TOPIC_PREFIX must be set")
	}

	s := server.New(machineID, mqttTopicPrefix)
	if err := s.Run(ctx, mqttServerUrl); err != nil {
		log.WithError(err).Errorf("Server failed")
		os.Exit(1)
	}

	// Listen for the interrupt signal
	<-ctx.Done()
	s.Close()
}
