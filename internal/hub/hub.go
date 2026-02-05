package hub

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"wabus/internal/domain"
)

type Client struct {
	ID    string
	Send  chan []byte
	tiles map[string]struct{}
	mu    sync.RWMutex
}

func NewClient(id string, bufferSize int) *Client {
	return &Client{
		ID:    id,
		Send:  make(chan []byte, bufferSize),
		tiles: make(map[string]struct{}),
	}
}

func (c *Client) HasTile(tileID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.tiles[tileID]
	return ok
}

func (c *Client) AddTiles(tileIDs []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, id := range tileIDs {
		c.tiles[id] = struct{}{}
	}
}

func (c *Client) RemoveTiles(tileIDs []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, id := range tileIDs {
		delete(c.tiles, id)
	}
}

func (c *Client) GetTiles() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	tiles := make([]string, 0, len(c.tiles))
	for id := range c.tiles {
		tiles = append(tiles, id)
	}
	return tiles
}

type Hub struct {
	mu          sync.RWMutex
	clients     map[*Client]struct{}
	tileClients map[string]map[*Client]struct{}

	register   chan *Client
	unregister chan *Client
	broadcast  chan []domain.VehicleDelta

	logger *slog.Logger
}

func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients:     make(map[*Client]struct{}),
		tileClients: make(map[string]map[*Client]struct{}),
		register:    make(chan *Client, 16),
		unregister:  make(chan *Client, 16),
		broadcast:   make(chan []domain.VehicleDelta, 256),
		logger:      logger,
	}
}

func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.closeAllClients()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = struct{}{}
			h.mu.Unlock()
			h.logger.Debug("client registered", "client_id", client.ID, "total", len(h.clients))

		case client := <-h.unregister:
			h.removeClient(client)

		case deltas := <-h.broadcast:
			h.fanoutDeltas(deltas)
		}
	}
}

func (h *Hub) Subscribe(client *Client, tileIDs []string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	client.AddTiles(tileIDs)

	for _, tileID := range tileIDs {
		if h.tileClients[tileID] == nil {
			h.tileClients[tileID] = make(map[*Client]struct{})
		}
		h.tileClients[tileID][client] = struct{}{}
	}
}

func (h *Hub) Unsubscribe(client *Client, tileIDs []string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	client.RemoveTiles(tileIDs)

	for _, tileID := range tileIDs {
		if h.tileClients[tileID] != nil {
			delete(h.tileClients[tileID], client)
			if len(h.tileClients[tileID]) == 0 {
				delete(h.tileClients, tileID)
			}
		}
	}
}

func (h *Hub) Broadcast(deltas []domain.VehicleDelta) {
	if len(deltas) == 0 {
		return
	}
	select {
	case h.broadcast <- deltas:
	default:
		h.logger.Warn("broadcast channel full, dropping deltas", "count", len(deltas))
	}
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

type DeltaMessage struct {
	Type    string                 `json:"type"`
	Payload DeltaPayload           `json:"payload"`
}

type DeltaPayload struct {
	Updates []*domain.Vehicle `json:"updates,omitempty"`
	Removes []string          `json:"removes,omitempty"`
}

func (h *Hub) fanoutDeltas(deltas []domain.VehicleDelta) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clientDeltas := make(map[*Client][]domain.VehicleDelta)

	for _, d := range deltas {
		if clients, ok := h.tileClients[d.TileID]; ok {
			for client := range clients {
				clientDeltas[client] = append(clientDeltas[client], d)
			}
		}
	}

	for client, ds := range clientDeltas {
		msg := buildDeltaMessage(ds)
		data, err := json.Marshal(msg)
		if err != nil {
			continue
		}

		select {
		case client.Send <- data:
		default:
			h.logger.Debug("client send buffer full", "client_id", client.ID)
		}
	}
}

func buildDeltaMessage(deltas []domain.VehicleDelta) DeltaMessage {
	var updates []*domain.Vehicle
	var removes []string

	for _, d := range deltas {
		switch d.Type {
		case domain.DeltaUpdate:
			updates = append(updates, d.Vehicle)
		case domain.DeltaRemove:
			removes = append(removes, d.Key)
		}
	}

	return DeltaMessage{
		Type: "delta",
		Payload: DeltaPayload{
			Updates: updates,
			Removes: removes,
		},
	}
}

func (h *Hub) removeClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client]; !ok {
		return
	}

	for _, tileID := range client.GetTiles() {
		if h.tileClients[tileID] != nil {
			delete(h.tileClients[tileID], client)
			if len(h.tileClients[tileID]) == 0 {
				delete(h.tileClients, tileID)
			}
		}
	}

	delete(h.clients, client)
	close(client.Send)
	h.logger.Debug("client unregistered", "client_id", client.ID, "total", len(h.clients))
}

func (h *Hub) closeAllClients() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for client := range h.clients {
		close(client.Send)
	}
	h.clients = make(map[*Client]struct{})
	h.tileClients = make(map[string]map[*Client]struct{})
}
