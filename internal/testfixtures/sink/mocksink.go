//go:build test

package sink

import (
	"context"
	"sync"

	"github.com/withobsrvr/flowctl/internal/processor"
)

// MockSink implements Sink interface for testing
type MockSink struct {
	messages      [][]*processor.Message
	connectCalled bool
	writeCalled   bool
	closeCalled   bool
	mutex         sync.Mutex
}

// NewMockSink creates a new mock sink that stores messages in memory
func NewMockSink() *MockSink {
	return &MockSink{
		messages: make([][]*processor.Message, 0),
	}
}

func (s *MockSink) Connect(cfg map[string]any) error {
	s.connectCalled = true
	return nil
}

func (s *MockSink) Write(ctx context.Context, msgs []*processor.Message) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.writeCalled = true
	s.messages = append(s.messages, msgs)
	return nil
}

func (s *MockSink) Close() error {
	s.closeCalled = true
	return nil
}

func (s *MockSink) Healthy() error {
	return nil
}

// Messages returns all the messages written to this sink for testing
func (s *MockSink) Messages() [][]*processor.Message {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	result := make([][]*processor.Message, len(s.messages))
	copy(result, s.messages)
	return result
}

// Stats returns information about mock sink calls for testing
func (s *MockSink) Stats() map[string]interface{} {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	return map[string]interface{}{
		"connect_called": s.connectCalled,
		"write_called":   s.writeCalled,
		"close_called":   s.closeCalled,
		"message_count":  len(s.messages),
	}
}