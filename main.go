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

	var s *server.Server

	go func() {
		s = server.New(machineID, "bornhack/2023/wip.bar")
		if err := s.Run(ctx, "test.mosquitto.org:1883"); err != nil {
			log.WithError(err).Errorf("Server failed")
		}
	}()

	// Listen for the interrupt signal
	<-ctx.Done()
	s.Close()
}
