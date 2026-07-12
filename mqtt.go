package main

import (
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MQTT QoS levels.
const (
	QoS0 byte = 0 // at most once  — fire and forget, may be lost
	QoS1 byte = 1 // at least once — acknowledged, may be duplicated
	QoS2 byte = 2 // exactly once
)

// opTimeout bounds how long we wait for a publish/subscribe to complete before
// giving up, so a stalled broker never blocks a caller indefinitely.
const opTimeout = 2 * time.Second

// MQTTClient adapts a paho MQTT client to the SubscribeServer and PublishServer
// interfaces used by the aggregator and publisher.
type MQTTClient struct {
	client mqtt.Client
}

// NewMQTTClient connects to the given broker and returns a ready-to-use client.
// broker is a URL such as "tcp://localhost:1883".
func NewMQTTClient(broker, clientID string) (*MQTTClient, error) {
	opts := mqtt.NewClientOptions().
		AddBroker(broker).
		SetClientID(clientID)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("connect to mqtt broker %q: %w", broker, token.Error())
	}

	return &MQTTClient{client: client}, nil
}

// Publish sends a message to the given topic at the requested QoS.
func (m *MQTTClient) Publish(topic string, qos byte, message []byte) error {
	token := m.client.Publish(topic, qos, true, message)
	if !token.WaitTimeout(opTimeout) {
		return fmt.Errorf("publish to %q timed out after %s", topic, opTimeout)
	}
	return token.Error()
}

// Subscribe registers a handler that is invoked for every message on the topic.
func (m *MQTTClient) Subscribe(topic string, qos byte, handler func(message []byte)) error {
	token := m.client.Subscribe(topic, qos, func(_ mqtt.Client, msg mqtt.Message) {
		handler(msg.Payload())
	})
	if !token.WaitTimeout(opTimeout) {
		return fmt.Errorf("subscribe to %q timed out after %s", topic, opTimeout)
	}
	return token.Error()
}

// Disconnect closes the connection to the broker.
func (m *MQTTClient) Disconnect() {
	const gracePeriodMs = 250
	m.client.Disconnect(gracePeriodMs)
}
