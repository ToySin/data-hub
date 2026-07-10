# data-hub

### Overview

This is an example service that aggregates various types of data into a single object and publishes them using an event-driven architecture.

The data handled by this service is broadly categorized into three types:

**1. High-Frequency Data** (e.g., Robot Pose)

* Processing every single state change in a high-frequency stream is highly inefficient.
* Instead of triggering an event for every change, this data relies on an internal **time window** to periodically publish the latest snapshot.

**2. Noisy / Fluctuating Data** (e.g., Battery Status)

* Sensor readings like battery levels are prone to noise and micro-fluctuations (jitter).
* To prevent unnecessary updates from these tiny jitters, a publish event is triggered *only* when the change in value exceeds a predefined **delta threshold** (deadband).

**3. Mission-Critical / Loss-Sensitive Data** (e.g., Navigation Messages)

* While dropping some telemetry data might be acceptable, losing state changes in critical data can severely disrupt higher-level logic.
* The system is specifically designed to guarantee that these crucial navigation-related messages are processed and published without any data loss.

### Objective & Output

The primary goal is to process each message type via an event-driven approach—applying the specific optimizations mentioned above (like time windows for high-frequency data)—while also publishing a single, consolidated payload containing the aggregated state.

As a result, this example program publishes the following **four distinct topics**:

* `/pose`
* `/battery`
* `/navigation`
* `/robot_state` *(The aggregated object combining all of the above)*

### Data Structure

```go
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
```
