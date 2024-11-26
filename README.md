# Finite State Machine (FSM) Package for Go
A flexible and thread-safe finite state machine (FSM) implementation for Go. This package allows you to define custom states and transitions, manage state changes, and subscribe to state updates using channels.

## Features

- Define custom states and allowed transitions
- Thread-safe state management
- Subscribe to state changes via channels
- Logging support using the standard library's `log/slog`

## Installation

```bash
go get github.com/robbyt/go-finiteState
```

## Usage

### Defining States and Transitions
First, define your custom states and the allowed transitions between them:

```go
import (
    "github.com/robbyt/go-finiteState/fsm"
)

// Define custom states
const (
    StatusOnline  = "StatusOnline"
    StatusOffline = "StatusOffline"
    StatusUnknown = "StatusUnknown"
)

// Define allowed transitions
var allowed = fsm.TransitionsConfig{
    StatusOnline:   {StatusOffline, StatusUnknown},
    StatusOffline:  {StatusOnline, StatusUnknown},
    StatusUnknown:  {},
}
```