// Command simulator mimics a robot's raw sensor feeds, publishing pose, battery
// and navigation state to MQTT at 20Hz. It plays out a short scenario: the robot
// drives toward a goal, hits an obstacle and gets stuck, then resumes and
// arrives — all while the battery slowly drains.
//
// Every topic is published on every tick, so duplicate/unchanged samples are
// expected and intentional; the data-hub service is responsible for debouncing.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Wire types — these must marshal to the same JSON shape the data-hub service
// unmarshals. They are intentionally a separate copy: the robot side and the
// service side are independent programs that only share a message contract.

type Pose struct {
	X float64
	Y float64
}

type Battery struct {
	Level float64
}

type NavigationStatus int

const (
	NavStUnknown NavigationStatus = iota
	NavStNavigating
	NavStStuck
	NavStArrived
)

type Navigation struct {
	CurrentNode string
	Status      NavigationStatus
}

const (
	topicPose       = "ros/pose"
	topicBattery    = "ros/battery"
	topicNavigation = "ros/navigation"

	publishRate      = 50 * time.Millisecond // 20Hz
	batteryDrainRate = 1.0                   // percent per second
)

// scenario returns the robot's pose and navigation state at the given elapsed
// time (seconds). Timeline:
//
//	0 – 5s : navigating from n1 toward n2 (x: 0 → 5)
//	5 – 8s : stuck at an obstacle (x frozen at 5)
//	8 – 13s: navigating from n2 toward n3 (x: 5 → 10)
//	13s +  : arrived at n3
func scenario(elapsed float64) (Pose, Navigation) {
	switch {
	case elapsed < 5:
		return Pose{X: elapsed, Y: 0}, Navigation{CurrentNode: "n2", Status: NavStNavigating}
	case elapsed < 8:
		return Pose{X: 5, Y: 0}, Navigation{CurrentNode: "n2", Status: NavStStuck}
	case elapsed < 13:
		return Pose{X: 5 + (elapsed - 8), Y: 0}, Navigation{CurrentNode: "n3", Status: NavStNavigating}
	default:
		return Pose{X: 10, Y: 0}, Navigation{CurrentNode: "n3", Status: NavStArrived}
	}
}

func main() {
	broker := flag.String("broker", "tcp://localhost:1883", "MQTT broker URL")
	clientID := flag.String("client-id", "ros-simulator", "MQTT client ID")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	opts := mqtt.NewClientOptions().AddBroker(*broker).SetClientID(*clientID)
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		slog.Error("failed to connect to mqtt broker", "error", token.Error())
		os.Exit(1)
	}
	defer client.Disconnect(250)

	publish := func(topic string, v any) {
		payload, err := json.Marshal(v)
		if err != nil {
			slog.Error("failed to marshal", "topic", topic, "error", err)
			return
		}
		// QoS 0: raw feeds tolerate loss, and we republish at 20Hz anyway.
		token := client.Publish(topic, 0, false, payload)
		token.Wait()
		if token.Error() != nil {
			slog.Error("failed to publish", "topic", topic, "error", token.Error())
		}
	}

	slog.Info("simulator started", "broker", *broker, "rate", "20Hz")

	ticker := time.NewTicker(publishRate)
	defer ticker.Stop()

	start := time.Now()
	lastStatus := NavStUnknown
	for {
		select {
		case <-ctx.Done():
			slog.Info("simulator stopped")
			return
		case <-ticker.C:
			elapsed := time.Since(start).Seconds()

			pose, nav := scenario(elapsed)
			level := math.Max(0, 100-elapsed*batteryDrainRate)
			battery := Battery{Level: level}

			if nav.Status != lastStatus {
				slog.Info("navigation status changed",
					"status", nav.Status, "node", nav.CurrentNode,
					"x", pose.X, "battery", math.Round(level*10)/10)
				lastStatus = nav.Status
			}

			publish(topicPose, pose)
			publish(topicBattery, battery)
			publish(topicNavigation, nav)
		}
	}
}
