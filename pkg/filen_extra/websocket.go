package filenextra

import (
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

type WebsocketConnection struct {
	url            string
	connMu         sync.RWMutex
	conn           *websocket.Conn
	stopChan       chan struct{}
	doneChan       chan struct{}
	closeOnce      sync.Once
	finishOnce     sync.Once
	started        atomic.Bool
	writeMu        sync.Mutex
	messageChan    chan []byte
	pingIntervalMu sync.RWMutex
	pingInterval   time.Duration
	requestHeader  http.Header
}

func NewWebsocketConnection(u string, requestHeader http.Header) *WebsocketConnection {
	return &WebsocketConnection{
		url:           u,
		stopChan:      make(chan struct{}),
		doneChan:      make(chan struct{}),
		messageChan:   make(chan []byte, 16),
		pingInterval:  15 * time.Second,
		requestHeader: requestHeader,
	}
}

func (w *WebsocketConnection) SetPingInterval(d time.Duration) {
	if d <= 0 {
		d = 15 * time.Second
	}
	w.pingIntervalMu.Lock()
	w.pingInterval = d
	w.pingIntervalMu.Unlock()
}

func (w *WebsocketConnection) Start() {
	if !w.started.CompareAndSwap(false, true) {
		return
	}
	if !w.connect() {
		w.finish()
		return
	}
	go w.startMessageLoop()
	go w.startPingLoop()
}

func (w *WebsocketConnection) SendString(message string) error {
	return w.writeMessage(websocket.TextMessage, []byte(message))
}

func (w *WebsocketConnection) NextMessage() ([]byte, bool) {
	message, ok := <-w.messageChan
	return message, ok
}

func (w *WebsocketConnection) Close() error {
	w.closeOnce.Do(func() { close(w.stopChan) })
	conn := w.currentConn()
	var err error
	if conn != nil {
		err = conn.Close()
	}
	if !w.started.Load() {
		w.finish()
	} else {
		<-w.doneChan
	}
	return err
}

func (w *WebsocketConnection) startMessageLoop() {
	defer func() {
		if conn := w.currentConn(); conn != nil {
			_ = conn.Close()
		}
		w.finish()
	}()
	for {
		conn := w.currentConn()
		if conn == nil {
			return
		}
		_, message, err := conn.ReadMessage()
		if err != nil {
			select {
			case <-w.stopChan:
				return
			default:
			}
			if isCloseError(err) {
				log.Warn().Err(err).Msg("WebSocket closed, reconnecting")
			} else {
				log.Warn().Err(err).Msg("WebSocket read error, reconnecting")
			}
			w.connMu.Lock()
			if w.conn == conn {
				w.conn = nil
			}
			w.connMu.Unlock()
			_ = conn.Close()
			if !w.connect() {
				return
			}
			continue
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
	select {
	case w.messageChan <- message:
	case <-w.stopChan:
	}
}

func (w *WebsocketConnection) connect() bool {
	for {
		select {
		case <-w.stopChan:
			return false
		default:
		}

		con, _, err := websocket.DefaultDialer.Dial(w.url, w.requestHeader)
		if err != nil {
			log.Error().Err(err).Msg("WebSocket connection failed, retrying...")
			timer := time.NewTimer(5 * time.Second)
			select {
			case <-timer.C:
			case <-w.stopChan:
				if !timer.Stop() {
					<-timer.C
				}
				return false
			}
			continue
		}

		w.connMu.Lock()
		select {
		case <-w.stopChan:
			w.connMu.Unlock()
			_ = con.Close()
			return false
		default:
			w.conn = con
			w.connMu.Unlock()
			return true
		}
	}
}

func (w *WebsocketConnection) currentConn() *websocket.Conn {
	w.connMu.RLock()
	defer w.connMu.RUnlock()
	return w.conn
}

func (w *WebsocketConnection) writeMessage(messageType int, message []byte) error {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()

	conn := w.currentConn()
	if conn == nil {
		return errors.New("WebSocket is not connected")
	}
	return conn.WriteMessage(messageType, message)
}

func (w *WebsocketConnection) pingIntervalValue() time.Duration {
	w.pingIntervalMu.RLock()
	defer w.pingIntervalMu.RUnlock()
	return w.pingInterval
}

func (w *WebsocketConnection) finish() {
	w.finishOnce.Do(func() {
		close(w.messageChan)
		close(w.doneChan)
	})
}

func (w *WebsocketConnection) startPingLoop() {
	ticker := time.NewTicker(w.pingIntervalValue())
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err := w.writeMessage(websocket.PingMessage, []byte{})
			if err != nil {
				log.Warn().Err(err).Msg("WebSocket ping failed")
				continue
			}
			ticker.Reset(w.pingIntervalValue())
		case <-w.stopChan:
			return
		}
	}
}
