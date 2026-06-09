package client

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

// WSConn is a live WebSocket connection. Events surfaces server pushes (posts,
// edits, etc.); it is closed when the connection drops.
type WSConn struct {
	client *model.WebSocketClient
	Events chan *model.WebSocketEvent
}

// ConnectWS opens a WebSocket to the server and starts forwarding events. The
// forwarder also drains the ping/response channels so the reader never blocks.
func (mm *MM) ConnectWS() (*WSConn, error) {
	// https://host -> wss://host, http://host -> ws://host
	wsURL := strings.Replace(mm.URL, "http", "ws", 1)
	ws, err := model.NewWebSocketClient4(wsURL, mm.Client.AuthToken)
	if err != nil {
		return nil, fmt.Errorf("websocket connect failed: %w", err)
	}
	ws.Listen()

	out := make(chan *model.WebSocketEvent, 100)
	go func() {
		for {
			select {
			case ev, ok := <-ws.EventChannel:
				if !ok {
					close(out) // connection closed
					return
				}
				out <- ev
			case <-ws.PingTimeoutChannel:
			case <-ws.ResponseChannel:
			}
		}
	}()

	return &WSConn{client: ws, Events: out}, nil
}

// Close terminates the connection.
func (c *WSConn) Close() {
	if c != nil && c.client != nil {
		c.client.Close()
	}
}
