package common

import (
	"fmt"
	"sync"
	"time"
)

// Spinner is a simple, cursor-safe terminal spinner.
// It never hides the cursor, so it needs no Ctrl+C cleanup.
type Spinner struct {
	frames  []string
	message string
	delay   time.Duration

	mu      sync.Mutex
	running bool
	done    chan struct{}
	wg      sync.WaitGroup
}

// New creates a spinner with a default frame set and message.
func NewSpinner(message string) *Spinner {
	return &Spinner{
		frames:  []string{"|", "/", "-", "\\"},
		message: message,
		delay:   100 * time.Millisecond,
	}
}

// Start begins spinning in the background. Safe to call once per instance.
func (s *Spinner) Start() *Spinner {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return s
	}
	s.running = true
	s.done = make(chan struct{})
	s.wg.Add(1)

	go func() {
		defer s.wg.Done()
		i := 0
		for {
			select {
			case <-s.done:
				return
			default:
				s.mu.Lock()
				msg := s.message
				s.mu.Unlock()
				fmt.Printf("\r\x1b[K%s %s ", msg, s.frames[i%len(s.frames)])
				i++
				time.Sleep(s.delay)
			}
		}
	}()

	return s
}

// Stop halts the spinner and clears the line. Safe to call multiple times
// or even if Start was never called.
func (s *Spinner) Stop(finalMsg string) {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.done)
	s.mu.Unlock()

	s.wg.Wait() // ensure the goroutine has stopped writing before we write the final line

	fmt.Print("\r\x1b[K") // clear whatever the spinner line currently has, no padding math needed
	if finalMsg != "" {
		fmt.Println(finalMsg)
	}
}

// UpdateMessage lets you change the label while it's spinning (optional).
func (s *Spinner) UpdateMessage(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = msg
}
