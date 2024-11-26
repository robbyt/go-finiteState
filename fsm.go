/*
Copyright 2024 Robert Terhaar <robbyt@robbyt.net>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package fsm

import (
	"fmt"
	"log/slog"
	"sync"
)

// Machine represents a finite state machine that tracks its current state
// and manages state transitions.
type Machine struct {
	mutex              sync.RWMutex
	state              string
	allowedTransitions transitionConfigWithIndex
	subscriberMutex    sync.Mutex
	subscribers        map[chan string]struct{}
	channelBufferSize  int // ChannelBufferSize for the status channel, must always be at least 1
	Logger             *slog.Logger
}

// NewMachine initializes a new finite state machine with the specified initial state and
// allowed state transitions.
//
// Example of allowedTransitions:
//
//	allowedTransitions := TransitionsConfig{
//		StatusNew:       {StatusBooting, StatusError},
//		StatusBooting:   {StatusRunning, StatusError},
//		StatusRunning:   {StatusReloading, StatusExited, StatusError},
//		StatusReloading: {StatusRunning, StatusError},
//		StatusError:     {StatusNew, StatusExited},
//		StatusExited:    {StatusNew},
//	}
func NewMachine(logger *slog.Logger, initialState string, allowedTransitions TransitionsConfig) (*Machine, error) {
	if logger == nil {
		logger = slog.Default().WithGroup("fsm")
		logger.Warn("Logger is nil, using the default logger configuration.")
	}

	if allowedTransitions == nil || len(allowedTransitions) == 0 {
		return nil, fmt.Errorf("%w: allowedTransitions is empty or nil", ErrAvailableStateData)
	}

	if _, ok := allowedTransitions[initialState]; !ok {
		return nil, fmt.Errorf("%w: initial state '%s' is not defined", ErrInvalidState, initialState)
	}

	trnsIndex := newTransitionWithIndex(allowedTransitions)

	return &Machine{
		state:              initialState,
		allowedTransitions: trnsIndex,
		subscribers:        make(map[chan string]struct{}),
		channelBufferSize:  defaultStateChanBufferSize,
		Logger:             logger,
	}, nil
}

// GetState returns the current state of the finite state machine.
func (fsm *Machine) GetState() string {
	fsm.mutex.RLock()
	defer fsm.mutex.RUnlock()
	return fsm.state
}

// setState updates the FSM's state and notifies all subscribers about the state change.
// It assumes that the caller has already acquired the necessary locks.
func (fsm *Machine) setState(state string) {
	fsm.state = state
	fsm.broadcast(state)
}

// SetState updates the FSM's state to the provided state, bypassing the usual transition rules.
// It only succeeds if the requested state is defined in the allowedTransitions configuration.
func (fsm *Machine) SetState(state string) error {
	fsm.mutex.Lock()
	defer fsm.mutex.Unlock()

	if _, ok := fsm.allowedTransitions[state]; !ok {
		return fmt.Errorf("%w: state '%s' is not defined", ErrInvalidState, state)
	}

	fsm.setState(state)
	return nil
}

// transition attempts to change the FSM's state to toState. It returns an error if the transition
// is invalid or if the target state is not allowed from the current state.
func (fsm *Machine) transition(toState string) error {
	currentState := fsm.state
	allowedTransitions, ok := fsm.allowedTransitions[currentState]
	if !ok {
		err := fmt.Errorf("%w: current state is '%s'", ErrInvalidState, currentState)
		fsm.Logger.Error("Invalid state transition table", "error", err)
		return err
	}

	if _, exists := allowedTransitions[toState]; exists {
		fsm.setState(toState)
		fsm.Logger.Debug("Transition successful", "from", currentState, "to", toState)
		return nil
	}

	return fmt.Errorf("%w: from '%s' to '%s'", ErrInvalidStateTransition, currentState, toState)
}

// Transition changes the FSM's state to toState. It ensures that the transition adheres to the
// allowed transitions. Returns ErrInvalidState if the target state is undefined, or
// ErrInvalidStateTransition if the transition is not allowed.
func (fsm *Machine) Transition(toState string) error {
	fsm.mutex.Lock()
	defer fsm.mutex.Unlock()
	return fsm.transition(toState)
}

// TransitionIfCurrentState changes the FSM's state to toState only if the current state matches
// fromState. This returns an error if the current state does not match or if the transition is
// not allowed.
func (fsm *Machine) TransitionIfCurrentState(fromState, toState string) error {
	fsm.mutex.Lock()
	defer fsm.mutex.Unlock()

	if fsm.state != fromState {
		return fmt.Errorf("%w: current state is '%s', expected '%s'", ErrCurrentStateIncorrect, fsm.state, fromState)
	}

	return fsm.transition(toState)
}
