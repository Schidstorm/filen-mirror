package filenextra

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestMalformedPacketsDoNotPanic(t *testing.T) {
	listener := &FilenEventListener{}
	listener.handleMessagePayload(nil)
	listener.handleHandshake([]byte("{"))
}

func TestWebsocketCloseStopsReadLoop(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_, _, _ = conn.ReadMessage()
	}))
	defer server.Close()

	connection := NewWebsocketConnection(strings.Replace(server.URL, "http", "ws", 1), nil)
	connection.Start()

	closed := make(chan error, 1)
	go func() { closed <- connection.Close() }()

	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not stop the WebSocket read loop")
	}
}
