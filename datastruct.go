package main

// Pose represents a geometeric position of robot.
type Pose struct {
	x float64
	y float64
}

// Battery represents a rest of battery level of robot.
type Battery struct {
	level float64
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
	cNode  string
	status NavigationStatus
}

// RobotState represents a snapshot of a robot's overall data.
type RobotState struct {
	pose       Pose
	battery    Battery
	navigation Navigation
}
