package filenextra

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

type WebsocketConnection struct {
	url           string
	conn          *websocket.Conn
	closedChan    chan struct{}
	isClosing     bool
	messageChan   chan []byte
	pingInterval  time.Duration
	requestHeader http.Header
}

func NewWebsocketConnection(u string, requestHeader http.Header) *WebsocketConnection {
	return &WebsocketConnection{
		url:           u,
		closedChan:    make(chan struct{}),
		messageChan:   make(chan []byte, 16),
		pingInterval:  15 * time.Second,
		requestHeader: requestHeader,
	}
}

func (w *WebsocketConnection) SetPingInterval(d time.Duration) {
	w.pingInterval = d
}

func (w *WebsocketConnection) Start() {
	w.connect()
	go w.startMessageLoop()
	go w.startPingLoop()
}

func (w *WebsocketConnection) SendString(message string) error {
	return w.conn.WriteMessage(websocket.TextMessage, []byte(message))
}

func (w *WebsocketConnection) NextMessage() ([]byte, bool) {
	message, ok := <-w.messageChan
	return message, ok
}

func (w *WebsocketConnection) Close() error {
	w.isClosing = true
	err := w.conn.Close()
	<-w.closedChan
	return err
}

func (w *WebsocketConnection) startMessageLoop() {
	defer w.conn.Close()
	for {
		_, message, err := w.conn.ReadMessage()
		if err != nil {
			if isCloseError(err) && w.isClosing {
				defer close(w.closedChan)
				return
			} else if isCloseError(err) {
				w.connect()
				continue
			} else {
				log.Warn().Err(err).Msg("WebSocket read error")
				continue
			}
		}

		w.handleMessage(message)
	}
}

func isCloseError(err error) bool {
	if _, ok := err.(*websocket.CloseError); ok {
		return true
	}
	return false
}

func (w *WebsocketConnection) handleMessage(message []byte) {
	w.messageChan <- message
}

func (w *WebsocketConnection) connect() {
	for {
		con, _, err := websocket.DefaultDialer.Dial(w.url, w.requestHeader)
		if err != nil {
			log.Error().Err(err).Msg("WebSocket connection failed, retrying...")
			time.Sleep(time.Second * 5)
			continue
		}
		w.conn = con
		break
	}
}

func (w *WebsocketConnection) startPingLoop() {
	ticker := time.NewTicker(w.pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err := w.conn.WriteMessage(websocket.PingMessage, []byte{})
			if err != nil {
				log.Warn().Err(err).Msg("WebSocket ping failed")
				continue
			}
			ticker.Reset(w.pingInterval)
		case <-w.closedChan:
			return
		}
	}
}
