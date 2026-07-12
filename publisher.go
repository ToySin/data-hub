package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

type outbox struct {
	mu            sync.Mutex
	lastPublished time.Time

	stateQueue    []PublishEvent
	stateSnapshot PublishEvent
	hasSnapshot   bool
}

type PublishServer interface {
	Publish(topic string, qos byte, message []byte) error
}

type MessagePublisher struct {
	server  PublishServer
	outbox  *outbox
	eventCh chan PublishEvent

	cache *StateCache
}

// NewMessagePublisher creates a new instance of MessagePublisher.
func NewMessagePublisher(server PublishServer, eventCh chan PublishEvent, cache *StateCache) *MessagePublisher {
	return &MessagePublisher{
		server:  server,
		eventCh: eventCh,
		outbox:  &outbox{},
		cache:   cache,
	}
}

func (p *MessagePublisher) Start(ctx context.Context) error {
	// Receiving events from the event channel and updating the outbox accordingly.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-p.eventCh:
				if !ok {
					slog.Error("event channel closed")
					return
				}
				p.outbox.mu.Lock()
				switch event.Header.EventType {
				case EventTypeSnapshot:
					p.outbox.stateSnapshot = event
					p.outbox.hasSnapshot = true
				case EventTypeQueue:
					p.outbox.stateQueue = append(p.outbox.stateQueue, event)
				default:
					slog.Warn("unknown event type", "type", event.Header.EventType)
				}
				p.outbox.mu.Unlock()
			}
		}
	}()

	robotStateTicker := time.NewTicker(100 * time.Millisecond) // 10Hz
	go func() {
		<-ctx.Done()
		robotStateTicker.Stop()
	}()

	// RobotState publishing loop.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-robotStateTicker.C: // 10Hz
				var toPublish PublishEvent
				var has bool
				p.outbox.mu.Lock()
				if len(p.outbox.stateQueue) > 0 {
					toPublish = p.outbox.stateQueue[0]
					p.outbox.stateQueue = p.outbox.stateQueue[1:]
					p.outbox.lastPublished = toPublish.Header.Timestamp
					has = true
				} else if p.outbox.hasSnapshot && p.outbox.lastPublished.Before(p.outbox.stateSnapshot.Header.Timestamp) {
					toPublish = p.outbox.stateSnapshot
					p.outbox.lastPublished = toPublish.Header.Timestamp
					has = true
				}
				p.outbox.mu.Unlock()

				if has {
					jsonState, err := json.Marshal(toPublish)
					if err != nil {
						slog.Error("failed to marshal robot state", "error", err)
						continue
					}
					if err := p.server.Publish("api/robot_state", QoS1, jsonState); err != nil {
						slog.Error("failed to publish robot state", "error", err)
					}
				}
			}
		}
	}()

	poseTicker := time.NewTicker(100 * time.Millisecond) // 10Hz
	go func() {
		<-ctx.Done()
		poseTicker.Stop()
	}()

	// Pose publishing loop.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-poseTicker.C: // 10Hz
				pose, ok := p.cache.Pose()
				if !ok {
					continue
				}
				jsonPose, err := json.Marshal(pose)
				if err != nil {
					slog.Error("failed to marshal pose", "error", err)
					continue
				}
				if err := p.server.Publish("api/pose", QoS0, jsonPose); err != nil {
					slog.Error("failed to publish pose", "error", err)
				}
			}
		}
	}()
	return nil
}
