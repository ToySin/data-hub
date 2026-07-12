package main

import "time"

// Pose represents a geometeric position of robot.
type Pose struct {
	X float64
	Y float64
}

// Battery represents a rest of battery level of robot.
type Battery struct {
	Level float64
}

// NavigationStatus represents a current navigation situation.
type NavigationStatus int

const (
	NavStUnknown NavigationStatus = iota
	NavStNavigating
	NavStStuck
	NavStArrived
)

// Navigation represents the waypoints within a robot's path and its navigation situation.
type Navigation struct {
	CurrentNode string
	Status      NavigationStatus
}

// RobotState represents a snapshot of a robot's overall data.
type RobotState struct {
	Pose       Pose
	Battery    Battery
	Navigation Navigation
	Timestamp  time.Time
}

// EventType represents the type of event that can be published.
type EventType int

const (
	// EventTypeUnknown represents an unknown event type.
	EventTypeUnknown EventType = iota

	// EventTypeSnapshot represents a snapshot of the robot's state.
	// It is used to publish a loss-tolerant state of robot.
	EventTypeSnapshot

	// EventTypeQueue has higher priority than EventTypeSnapshot.
	// It is used to publish a loss-critical state of robot.
	EventTypeQueue
)

type EventHeader struct {
	Timestamp time.Time
	EventType EventType
}

type PublishEvent struct {
	Header     EventHeader
	RobotState RobotState
}
