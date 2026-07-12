package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

// StateCache is a thread-safe cache for storing the latest state of the robot.
type StateCache struct {
	poseMu     sync.RWMutex
	latestPose Pose
	hasPose    bool

	batteryMu     sync.RWMutex
	latestBattery Battery
	hasBattery    bool

	navigationMu     sync.RWMutex
	latestNavigation Navigation
	hasNavigation    bool
}

func (c *StateCache) Pose() (Pose, bool) {
	c.poseMu.RLock()
	defer c.poseMu.RUnlock()

	return c.latestPose, c.hasPose
}

func (c *StateCache) Battery() (Battery, bool) {
	c.batteryMu.RLock()
	defer c.batteryMu.RUnlock()

	return c.latestBattery, c.hasBattery
}

func (c *StateCache) Navigation() (Navigation, bool) {
	c.navigationMu.RLock()
	defer c.navigationMu.RUnlock()

	return c.latestNavigation, c.hasNavigation
}

func (c *StateCache) DiffBattery(new Battery) bool {
	c.batteryMu.RLock()
	defer c.batteryMu.RUnlock()

	if !c.hasBattery {
		return true
	}

	const deadband = 0.1
	diff := c.latestBattery.Level - new.Level
	return diff >= deadband || diff <= -deadband
}

func (c *StateCache) DiffNavigation(new Navigation) bool {
	c.navigationMu.RLock()
	defer c.navigationMu.RUnlock()

	if !c.hasNavigation {
		return true
	}

	old := c.latestNavigation
	return old.CurrentNode != new.CurrentNode || old.Status != new.Status
}

func (c *StateCache) UpdatePose(pose Pose) {
	c.poseMu.Lock()
	c.latestPose, c.hasPose = pose, true
	c.poseMu.Unlock()
}

func (c *StateCache) UpdateBattery(battery Battery) {
	c.batteryMu.Lock()
	c.latestBattery, c.hasBattery = battery, true
	c.batteryMu.Unlock()
}

func (c *StateCache) UpdateNavigation(navigation Navigation) {
	c.navigationMu.Lock()
	c.latestNavigation, c.hasNavigation = navigation, true
	c.navigationMu.Unlock()
}

// Snapshot returns a snapshot of the current aggregated state of the robot.
func (c *StateCache) Snapshot() RobotState {
	c.poseMu.RLock()
	c.batteryMu.RLock()
	c.navigationMu.RLock()
	defer c.poseMu.RUnlock()
	defer c.batteryMu.RUnlock()
	defer c.navigationMu.RUnlock()

	return RobotState{
		Pose:       c.latestPose,
		Battery:    c.latestBattery,
		Navigation: c.latestNavigation,
		Timestamp:  time.Now(),
	}
}

type SubscribeServer interface {
	Subscribe(topic string, qos byte, handler func(message []byte)) error
}

type StateAggregator struct {
	server  SubscribeServer
	cache   *StateCache
	eventCh chan PublishEvent
}

// NewStateAggregator creates a new instance of StateAggregator.
func NewStateAggregator(server SubscribeServer, cache *StateCache, eventCh chan PublishEvent) *StateAggregator {
	return &StateAggregator{
		server:  server,
		cache:   cache,
		eventCh: eventCh,
	}
}

func (a *StateAggregator) handlePoseMessage(message []byte) {
	var pose Pose
	if err := json.Unmarshal(message, &pose); err != nil {
		slog.Error("failed to unmarshal pose message", "error", err)
		return
	}
	a.cache.UpdatePose(pose)
}

func (a *StateAggregator) handleBatteryMessage(message []byte) {
	var battery Battery
	if err := json.Unmarshal(message, &battery); err != nil {
		slog.Error("failed to unmarshal battery message", "error", err)
		return
	}
	if !a.cache.DiffBattery(battery) {
		return
	}
	a.cache.UpdateBattery(battery)
	snapshot := a.cache.Snapshot()
	a.eventCh <- PublishEvent{
		Header: EventHeader{
			Timestamp: snapshot.Timestamp,
			EventType: EventTypeSnapshot,
		},
		RobotState: snapshot,
	}
}

func (a *StateAggregator) handleNavigationMessage(message []byte) {
	var navigation Navigation
	if err := json.Unmarshal(message, &navigation); err != nil {
		slog.Error("failed to unmarshal navigation message", "error", err)
		return
	}
	if !a.cache.DiffNavigation(navigation) {
		return
	}
	a.cache.UpdateNavigation(navigation)
	snapshot := a.cache.Snapshot()
	a.eventCh <- PublishEvent{
		Header: EventHeader{
			Timestamp: snapshot.Timestamp,
			EventType: EventTypeQueue,
		},
		RobotState: snapshot,
	}
}

func (a *StateAggregator) Start(ctx context.Context) error {
	// navigation is loss-critical (QoS 1); pose and battery tolerate loss (QoS 0).
	a.server.Subscribe("ros/pose", QoS0, a.handlePoseMessage)
	a.server.Subscribe("ros/battery", QoS0, a.handleBatteryMessage)
	a.server.Subscribe("ros/navigation", QoS1, a.handleNavigationMessage)

	return nil
}
