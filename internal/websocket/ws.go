package websocket

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/nrynss/opsec-control/internal/contracts"
)

// client wraps a conn with a mutex to serialize writes (gorilla/websocket requires
// at most one concurrent writer per conn).
type client struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (c *client) write(msg []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteMessage(websocket.TextMessage, msg)
}

// Server handles the WS /stream for state ripples and token-by-token outputs.
type Server struct {
	bus contracts.EventBus

	upgrader websocket.Upgrader
	mu       sync.Mutex
	clients  []*client
}

// New creates a WS server.
func New(bus contracts.EventBus) *Server {
	return &Server{
		bus: bus,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // for demo/MVD
			},
		},
	}
}

// Handler returns the http.Handler for the WS endpoint.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.serveWS)
}

func (s *Server) serveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "upgrade failed", http.StatusInternalServerError)
		return
	}

	c := &client{conn: conn}
	s.mu.Lock()
	s.clients = append(s.clients, c)
	s.mu.Unlock()

	defer func() {
		conn.Close()
		s.removeClient(c)
	}()

	// Subscribe to bus and forward events to this conn.
	ch, cancel := s.bus.Subscribe()
	defer cancel()

	// Read pump to detect client disconnects/closes promptly (gorilla best practice).
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for ev := range ch {
		data, _ := json.Marshal(ev)
		if err := c.write(data); err != nil {
			return
		}
	}
}

// Broadcast can be used to push arbitrary messages (e.g. COP or token streams).
// Writes are serialized per client.
func (s *Server) Broadcast(msg any) {
	data, _ := json.Marshal(msg)
	s.mu.Lock()
	clients := append([]*client(nil), s.clients...) // copy
	s.mu.Unlock()

	for _, c := range clients {
		if err := c.write(data); err != nil {
			s.removeClient(c)
		}
	}
}

func (s *Server) removeClient(c *client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.clients {
		if s.clients[i] == c {
			s.clients = append(s.clients[:i], s.clients[i+1:]...)
			return
		}
	}
}
