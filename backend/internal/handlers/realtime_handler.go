package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/srams/backend/internal/middleware"
)

// SSE Event Types
const (
	EventConfigUpdate    = "CONFIG_UPDATE"
	EventUserDeleted     = "USER_DELETED"
	EventUserDeactivated = "USER_DEACTIVATED"
	EventSessionRevoked  = "SESSION_REVOKED"
	EventForceLogout     = "FORCE_LOGOUT"
)

// SSEEvent represents a real-time event
type SSEEvent struct {
	Type      string                 `json:"type"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// SSEClient represents a connected client
type SSEClient struct {
	ID      uuid.UUID
	UserID  uuid.UUID
	Channel chan SSEEvent
	Done    chan struct{}
}

// SSEHub manages all connected clients
type SSEHub struct {
	clients    map[uuid.UUID]*SSEClient
	mutex      sync.RWMutex
	register   chan *SSEClient
	unregister chan *SSEClient
	broadcast  chan SSEEvent
}

// Global hub instance
var Hub *SSEHub

// InitSSEHub initializes the global SSE hub
func InitSSEHub() {
	Hub = &SSEHub{
		clients:    make(map[uuid.UUID]*SSEClient),
		register:   make(chan *SSEClient),
		unregister: make(chan *SSEClient),
		broadcast:  make(chan SSEEvent, 100),
	}
	go Hub.run()
}

func (h *SSEHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client.ID] = client
			h.mutex.Unlock()

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				close(client.Channel)
			}
			h.mutex.Unlock()

		case event := <-h.broadcast:
			h.mutex.RLock()
			for _, client := range h.clients {
				select {
				case client.Channel <- event:
				default:
					// Client not receiving, skip
				}
			}
			h.mutex.RUnlock()
		}
	}
}

// Broadcast sends an event to all connected clients
func (h *SSEHub) Broadcast(event SSEEvent) {
	event.Timestamp = time.Now()
	h.broadcast <- event
}

// BroadcastToUser sends an event to a specific user
func (h *SSEHub) BroadcastToUser(userID uuid.UUID, event SSEEvent) {
	event.Timestamp = time.Now()
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	for _, client := range h.clients {
		if client.UserID == userID {
			select {
			case client.Channel <- event:
			default:
			}
		}
	}
}

// RealtimeHandler handles SSE connections
type RealtimeHandler struct{}

// NewRealtimeHandler creates a new realtime handler
func NewRealtimeHandler() *RealtimeHandler {
	return &RealtimeHandler{}
}

// ServeSSE handles SSE subscription
func (h *RealtimeHandler) ServeSSE(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Set headers for SSE
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	// Create client
	client := &SSEClient{
		ID:      uuid.New(),
		UserID:  user.ID,
		Channel: make(chan SSEEvent, 10),
		Done:    make(chan struct{}),
	}

	// Register client
	Hub.register <- client

	// Cleanup on disconnect
	defer func() {
		Hub.unregister <- client
	}()

	// Send initial ping
	fmt.Fprintf(c.Writer, "event: connected\ndata: {\"client_id\":\"%s\"}\n\n", client.ID)
	c.Writer.Flush()

	// Keep-alive ticker
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Listen for events
	clientGone := c.Request.Context().Done()
	for {
		select {
		case <-clientGone:
			return

		case event := <-client.Channel:
			data, _ := json.Marshal(event)
			fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event.Type, data)
			c.Writer.Flush()

		case <-ticker.C:
			// Keep-alive ping
			fmt.Fprintf(c.Writer, "event: ping\ndata: {}\n\n")
			c.Writer.Flush()
		}
	}
}

// Helper functions for broadcasting events

// BroadcastConfigUpdate broadcasts a config change to all clients
func BroadcastConfigUpdate(key string, value interface{}) {
	if Hub == nil {
		return
	}
	Hub.Broadcast(SSEEvent{
		Type: EventConfigUpdate,
		Payload: map[string]interface{}{
			"key":   key,
			"value": value,
		},
	})
}

// BroadcastUserDeleted notifies a user they have been deleted (force logout)
func BroadcastUserDeleted(userID uuid.UUID) {
	if Hub == nil {
		return
	}
	Hub.BroadcastToUser(userID, SSEEvent{
		Type: EventForceLogout,
		Payload: map[string]interface{}{
			"reason": "Your account has been deleted",
		},
	})
}

// BroadcastUserDeactivated notifies a user they have been deactivated
func BroadcastUserDeactivated(userID uuid.UUID) {
	if Hub == nil {
		return
	}
	Hub.BroadcastToUser(userID, SSEEvent{
		Type: EventForceLogout,
		Payload: map[string]interface{}{
			"reason": "Your account has been deactivated",
		},
	})
}

// BroadcastSessionRevoked notifies all sessions of a user to logout
func BroadcastSessionRevoked(userID uuid.UUID) {
	if Hub == nil {
		return
	}
	Hub.BroadcastToUser(userID, SSEEvent{
		Type: EventSessionRevoked,
		Payload: map[string]interface{}{
			"reason": "Your session has been revoked by administrator",
		},
	})
}
