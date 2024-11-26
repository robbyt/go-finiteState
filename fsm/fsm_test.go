package fsm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFSM(t *testing.T) {
	t.Parallel()

	t.Run("NewFSM with invalid initial status", func(t *testing.T) {
		fsm, err := New(nil, "bla", TypicalTransitions)
		assert.Nil(t, fsm)
		require.Error(t, err)
	})

	t.Run("NewFSM with nil allowedTransitions", func(t *testing.T) {
		fsm, err := New(nil, StatusNew, nil)
		assert.Nil(t, fsm)
		require.Error(t, err)
	})

	t.Run("GetState and SetState", func(t *testing.T) {
		fsm, err := New(nil, StatusNew, TypicalTransitions)
		require.NoError(t, err)

		assert.Equal(t, StatusNew, fsm.GetState())

		err = fsm.SetState(StatusRunning)
		require.NoError(t, err)
		assert.Equal(t, StatusRunning, fsm.GetState())

		// Test setting an invalid state
		err = fsm.SetState("invalid_state")
		assert.ErrorIs(t, err, ErrInvalidState)
		assert.Equal(t, StatusRunning, fsm.GetState())
	})

	t.Run("Transition", func(t *testing.T) {
		testCases := []struct {
			name          string
			initialState  string
			toState       string
			expectedErr   error
			expectedState string
		}{
			{
				name:          "Valid transition from StatusNew to StatusBooting",
				initialState:  StatusNew,
				toState:       StatusBooting,
				expectedErr:   nil,
				expectedState: StatusBooting,
			},
			{
				name:          "Invalid transition from StatusNew to StatusRunning",
				initialState:  StatusNew,
				toState:       StatusRunning,
				expectedErr:   ErrInvalidStateTransition,
				expectedState: StatusNew,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				fsm, err := New(nil, tc.initialState, TypicalTransitions)
				require.NoError(t, err)

				err = fsm.Transition(tc.toState)

				if tc.expectedErr != nil {
					assert.ErrorIs(t, err, tc.expectedErr)
				} else {
					assert.NoError(t, err)
				}

				assert.Equal(t, tc.expectedState, fsm.GetState())
			})
		}
	})

	t.Run("TransitionIfCurrentState", func(t *testing.T) {
		testCases := []struct {
			name          string
			initialState  string
			fromState     string
			toState       string
			expectedErr   error
			expectedState string
		}{
			{
				name:          "Valid transition with matching current state",
				initialState:  StatusNew,
				fromState:     StatusNew,
				toState:       StatusBooting,
				expectedErr:   nil,
				expectedState: StatusBooting,
			},
			{
				name:          "Invalid transition due to mismatched current state",
				initialState:  StatusBooting,
				fromState:     StatusNew,
				toState:       StatusRunning,
				expectedErr:   ErrCurrentStateIncorrect,
				expectedState: StatusBooting,
			},
			{
				name:          "Invalid transition due to invalid state transition",
				initialState:  StatusNew,
				fromState:     StatusNew,
				toState:       StatusRunning,
				expectedErr:   ErrInvalidStateTransition,
				expectedState: StatusNew,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				fsm, err := New(nil, tc.initialState, TypicalTransitions)
				require.NoError(t, err)

				err = fsm.TransitionIfCurrentState(tc.fromState, tc.toState)

				if tc.expectedErr != nil {
					assert.ErrorIs(t, err, tc.expectedErr)
				} else {
					assert.NoError(t, err)
				}

				assert.Equal(t, tc.expectedState, fsm.GetState())
			})
		}
	})
}

func TestFSM_Transition_DisallowedStateChange(t *testing.T) {
	t.Parallel()

	fsm, err := New(nil, StatusNew, TypicalTransitions)
	require.NoError(t, err)

	err = fsm.Transition("InvalidState")

	assert.ErrorIs(t, err, ErrInvalidStateTransition)
	assert.Equal(t, StatusNew, fsm.GetState())
}

func TestFSM_Transition_ModifiedAllowedTransitions(t *testing.T) {
	t.Parallel()

	fsm, err := New(nil, StatusNew, TypicalTransitions)
	require.NoError(t, err)

	// Remove all allowed transitions for the current state
	fsm.mutex.Lock()
	delete(fsm.allowedTransitions, StatusNew)
	fsm.mutex.Unlock()

	t.Run("Transition with removed allowedTransitions", func(t *testing.T) {
		err := fsm.Transition(StatusBooting)
		assert.ErrorIs(t, err, ErrInvalidState)
		assert.Equal(t, StatusNew, fsm.GetState())
	})
}

func TestFSM_NoAllowedTransitions(t *testing.T) {
	smallestTransitions := TransitionsConfig{
		StatusNew:   {StatusError},
		StatusError: {},
	}
	fsm, err := New(nil, StatusNew, smallestTransitions)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	listener := fsm.GetStateChan(ctx)

	err = fsm.Transition(StatusError)
	require.NoError(t, err)
	assert.Equal(t, StatusError, fsm.GetState())
	assert.Equal(t, StatusNew, <-listener, "Channel was created before the state, so the first is the initial state")
	assert.Equal(t, StatusError, <-listener, "The second state is the one we transitioned to")

	// since the valid states for StatusError are empty, unable to transition to any other state
	err = fsm.Transition(StatusNew)
	require.ErrorIs(t, err, ErrInvalidStateTransition)

	select {
	case <-listener:
		t.Error("No state transition expected")
	default:
		// All good, channel is empty!
	}
}
