package server

import "sync"

// Broker distributes PNG frames to all connected HTTP clients.
type Broker struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

func NewBroker() *Broker {
	return &Broker{
		clients: make(map[chan []byte]struct{}),
	}
}

// Subscribe returns a channel that receives each published frame.
func (b *Broker) Subscribe() chan []byte {
	ch := make(chan []byte, 1)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a client channel.
func (b *Broker) Unsubscribe(ch chan []byte) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
}

// Publish sends a frame to all subscribers, dropping slow clients.
func (b *Broker) Publish(frame []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.clients {
		select {
		case ch <- frame:
		default: // drop frame for slow/unresponsive clients
		}
	}
}
