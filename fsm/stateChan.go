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

import "context"

// stateChanBufferSize is the
const defaultStateChanBufferSize = 5

// SetChanBufferSize sets the buffer size for the state change channel.
func (fsm *Machine) SetChanBufferSize(size int) {
	if size < 1 {
		fsm.Logger.Warn("Invalid channel buffer size; must be at least 1", "size", size)
		return
	}

	fsm.mutex.RLock()
	currentBuffer := fsm.channelBufferSize
	fsm.mutex.RUnlock()

	if size == currentBuffer {
		fsm.Logger.Debug("Channel buffer size is already set to the requested size", "size", size)
		return
	}

	fsm.mutex.Lock()
	fsm.channelBufferSize = size
	fsm.mutex.Unlock()
}

// GetStateChan returns a channel that emits the FSM's state whenever it changes. The channel is
// closed when the provided context is canceled. If the channel is not being read, the status
// changes will be dropped, and a warning will be logged.
func (fsm *Machine) GetStateChan(ctx context.Context) <-chan string {
	if ctx == nil {
		fsm.Logger.Warn("Context is nil; this will cause a goroutine leak")
		ctx = context.Background()
	}

	// Ensure thread-safe access to channelBufferSize
	fsm.mutex.RLock()
	bufferSize := fsm.channelBufferSize
	fsm.mutex.RUnlock()

	// Send the current state immediately
	ch := make(chan string, bufferSize)
	ch <- fsm.GetState()

	// Add the channel to the subscribers list
	fsm.subscriberMutex.Lock()
	fsm.subscribers[ch] = struct{}{}
	fsm.subscriberMutex.Unlock()

	// Start a goroutine to monitor the context
	go func() {
		// Wait for context cancellation
		<-ctx.Done()
		// Clean up: remove the subscriber and close the channel
		fsm.unsubscribe(ch)
	}()

	return ch
}

func (fsm *Machine) unsubscribe(ch chan string) {
	fsm.subscriberMutex.Lock()
	defer fsm.subscriberMutex.Unlock()
	delete(fsm.subscribers, ch)
	close(ch)
}

func (fsm *Machine) broadcast(newState string) {
	logger := fsm.Logger.With("state", newState)
	fsm.subscriberMutex.Lock()
	defer fsm.subscriberMutex.Unlock()

	// Now iterate over the copied slice without holding the lock
	for ch := range fsm.subscribers {
		select {
		case ch <- newState:
			logger.Debug("Broadcasted state to channel", "channel", ch)
		default:
			logger.Warn("Channel is full; skipping broadcast", "channel", ch)
		}
	}
}
