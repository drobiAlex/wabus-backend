package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"wabus/internal/domain"
	"wabus/internal/hub"
	"wabus/internal/store"
)

type WSHandler struct {
	hub    *hub.Hub
	store  *store.Store
	logger *slog.Logger
}

func NewWSHandler(h *hub.Hub, s *store.Store, logger *slog.Logger) *WSHandler {
	return &WSHandler{hub: h, store: s, logger: logger}
}

type WSMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type SubscribePayload struct {
	TileIDs []string `json:"tileIds"`
}

type UnsubscribePayload struct {
	TileIDs []string `json:"tileIds"`
}

type SnapshotMessage struct {
	Type    string          `json:"type"`
	Payload SnapshotPayload `json:"payload"`
}

type SnapshotPayload struct {
	Vehicles []*domain.Vehicle `json:"vehicles"`
}

type PongMessage struct {
	Type string `json:"type"`
}

func (h *WSHandler) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		h.logger.Error("websocket accept failed", "error", err)
		return
	}

	clientID := uuid.New().String()
	client := hub.NewClient(clientID, 256)

	h.hub.Register(client)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go h.writeLoop(ctx, conn, client)

	h.readLoop(ctx, conn, client)
}

func (h *WSHandler) readLoop(ctx context.Context, conn *websocket.Conn, client *hub.Client) {
	defer func() {
		h.hub.Unregister(client)
		conn.Close(websocket.StatusNormalClosure, "")
	}()

	for {
		msgType, data, err := conn.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
				h.logger.Debug("websocket read error", "client_id", client.ID, "error", err)
			}
			return
		}

		if msgType != websocket.MessageText {
			continue
		}

		var msg WSMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			h.logger.Debug("invalid message format", "client_id", client.ID, "error", err)
			continue
		}

		switch msg.Type {
		case "subscribe":
			var payload SubscribePayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				continue
			}
			if len(payload.TileIDs) > 0 {
				h.hub.Subscribe(client, payload.TileIDs)
				h.sendSnapshot(client, payload.TileIDs)
			}

		case "unsubscribe":
			var payload UnsubscribePayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				continue
			}
			if len(payload.TileIDs) > 0 {
				h.hub.Unsubscribe(client, payload.TileIDs)
			}

		case "ping":
			h.sendPong(client)
		}
	}
}

func (h *WSHandler) writeLoop(ctx context.Context, conn *websocket.Conn, client *hub.Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case msg, ok := <-client.Send:
			if !ok {
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := conn.Write(writeCtx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				return
			}

		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := conn.Ping(pingCtx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func (h *WSHandler) sendSnapshot(client *hub.Client, tileIDs []string) {
	vehicles := h.store.SnapshotForTiles(tileIDs)

	msg := SnapshotMessage{
		Type: "snapshot",
		Payload: SnapshotPayload{
			Vehicles: vehicles,
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	select {
	case client.Send <- data:
	default:
		h.logger.Debug("failed to send snapshot, buffer full", "client_id", client.ID)
	}
}

func (h *WSHandler) sendPong(client *hub.Client) {
	msg := PongMessage{Type: "pong"}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	select {
	case client.Send <- data:
	default:
	}
}
