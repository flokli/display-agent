package mqtt

import (
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
)

const (
	timeout = 10 * time.Second
)

func Connect(serverURL string) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions().AddBroker(serverURL)
	client := mqtt.NewClient(opts)

	token := client.Connect()
	completed := token.WaitTimeout(timeout)
	if !completed {
		return nil, fmt.Errorf("timeout connecting to mqtt")
	} else {
		return client, token.Error()
	}
}

// Publishes a given value to the the broker at the given topic.
// Non-strings are converted to their string representations.
func Publish(mqttClient mqtt.Client, topic string, qos byte, retained bool, value interface{}) error {
	payload := fmt.Sprintf("%v", value)

	l := log.WithFields(log.Fields{
		"topic":    topic,
		"qos":      qos,
		"retained": retained,
		"value":    value,
		"payload":  payload,
	})

	token := mqttClient.Publish(topic, qos, retained, payload)
	completed := token.WaitTimeout(timeout)

	if !completed {
		return fmt.Errorf("timeout publishing to mqtt")
	} else {
		if token.Error() == nil {
			l.Trace("published message")
		}
		return token.Error()
	}
}

func Subscribe(mqttClient mqtt.Client, topic string, qos byte, cb mqtt.MessageHandler) error {
	l := log.WithFields(log.Fields{
		"topic": topic,
		"qos":   qos,
	})

	token := mqttClient.Subscribe(topic, qos, cb)
	completed := token.WaitTimeout(timeout)
	if !completed {
		return fmt.Errorf("timeout subscribing to mqtt")
	} else {
		if token.Error() == nil {
			l.Debug("subscribed")
		}
		return token.Error()
	}
}

func Unsubscribe(mqttClient mqtt.Client, topics []string) error {
	l := log.WithFields(log.Fields{
		"topics": topics,
	})

	token := mqttClient.Unsubscribe(topics...)
	completed := token.WaitTimeout(timeout)
	if !completed {
		return fmt.Errorf("timeout unsubscribing from mqtt")
	} else {
		if token.Error() == nil {
			l.Debug("unsubscribed")
		}
		return token.Error()
	}
}
