package filenextra

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/FilenCloudDienste/filen-sdk-go/filen"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
	"github.com/rs/zerolog/log"
)

var ErrInvalidWebSocketURL = errors.New("invalid WebSocket URL")

const packetTypeConnect = '0'
const messageTypeEvent = '2'
const packetTypePong = '3'
const packetTypeMessage = '4'

type FilenEventListener struct {
	filen     *filen.Filen
	conn      *WebsocketConnection
	eventChan chan TypedEvent
}

func NewFilenEvents(u string, client *filen.Filen, requestHeader http.Header) (*FilenEventListener, error) {
	wsUrl, err := buildWebSocketURL(u)
	if err != nil {
		return nil, err
	}

	return &FilenEventListener{
		filen: client,
		conn:  NewWebsocketConnection(wsUrl, requestHeader),
	}, nil
}

func (e *FilenEventListener) Start() {
	e.eventChan = make(chan TypedEvent, 100)
	e.conn.Start()

	go func() {
		for {
			msg, ok := e.conn.NextMessage()
			if !ok {
				break
			}
			e.handleMessage(msg)
		}
	}()
}

func (e *FilenEventListener) NextEvent() (TypedEvent, bool) {
	event, ok := <-e.eventChan
	return event, ok
}

func (e *FilenEventListener) Close() error {
	defer close(e.eventChan)
	return e.conn.Close()
}

func (e *FilenEventListener) handleMessage(message []byte) {
	if len(message) == 0 {
		log.Warn().Msg("Received empty message")
		return
	}

	packetType := message[0]
	payload := message[1:]

	switch packetType {
	case packetTypeConnect:
		e.handleHandshake(payload)
	case packetTypePong:
		log.Debug().Msg("Received pong")
	case packetTypeMessage:
		e.handleMessagePayload(payload)
	default:
		log.Warn().Msgf("Unknown packet type: %c", packetType)
	}
}

func (e *FilenEventListener) handleMessagePayload(payload []byte) {
	messageType := payload[0]
	messageData := payload[1:]

	if messageType != messageTypeEvent {
		return
	}

	var eventData []any
	err := json.Unmarshal(messageData, &eventData)
	if err != nil {
		log.Error().Err(err).Str("payload", string(messageData)).Msg("Failed to unmarshal event payload")
		return
	}

	if len(eventData) == 0 {
		log.Warn().Msg("Empty event data")
		return
	}

	eventName, ok := eventData[0].(string)
	if !ok {
		log.Warn().Msg("Invalid event name type")
		return
	}

	switch eventName {
	case "authFailed":
		e.Close()
		log.Fatal().Msg("Authentication failed")
	case "authSuccess":
		log.Info().Msg("Authentication successful")
	case "authed":
		if len(eventData) < 2 || eventData[1] == false {
			err := e.sendEvent("auth", map[string]string{
				"apiKey": e.filen.Client.APIKey,
			})
			if err != nil {
				log.Error().Err(err).Msg("Failed to send auth event")
			}
		}
	default:
		filenEventData := make(map[string]any)
		if len(eventData) >= 2 {
			if dataMap, ok := eventData[1].(map[string]any); ok {
				filenEventData = e.decryptFields(dataMap)
				unmarshalKey(filenEventData, "metadata")
				unmarshalKey(filenEventData, "name")
			}
		}

		filenEvent, err := InterpretEvent(eventName, filenEventData)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to parse event: %s", eventName)
			return
		}
		e.eventChan <- filenEvent
	}
}

func unmarshalKey(m map[string]any, key string) {
	if raw, ok := m[key]; ok {
		if str, ok := raw.(string); ok {
			var result map[string]any
			err := json.Unmarshal([]byte(str), &result)
			if err == nil {
				m[key] = result
			}
		}
	}
}

func (e *FilenEventListener) decryptFields(data map[string]any) map[string]any {
	decryptedData := make(map[string]any)
	for key, value := range data {
		if strValue, ok := value.(string); ok {
			if strings.HasPrefix(strValue, "U2FsdGVk") || strings.HasPrefix(strValue, "002") || strings.HasPrefix(strValue, "003") {
				decrypted, err := e.filen.DecryptMeta(crypto.EncryptedString(strValue))
				if err != nil {
					decryptedData[key] = value
				} else {
					decryptedData[key] = decrypted
				}
			} else {
				decryptedData[key] = value
			}
		} else if nestedMap, ok := value.(map[string]any); ok {
			decryptedData[key] = e.decryptFields(nestedMap)
		} else {
			decryptedData[key] = value
		}
	}

	return decryptedData
}

type handshakePayload struct {
	PingInterval int `json:"pingInterval"`
}

func (e *FilenEventListener) handleHandshake(payload []byte) {
	hp, err := parseHandshakePayload(payload)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to parse handshake payload")
	}
	e.conn.SetPingInterval(time.Duration(hp.PingInterval) * time.Millisecond)

	err = e.conn.SendString(string(packetTypeMessage) + string(packetTypeConnect))
	if err != nil {
		log.Error().Err(err).Msg("Failed to send connect acknowledgment")
		return
	}

	err = e.sendEvent("authed", time.Now().UnixMilli())
	if err != nil {
		log.Error().Err(err).Msg("Failed to send authed event")
		return
	}
}

func parseHandshakePayload(payload []byte) (*handshakePayload, error) {
	var hp handshakePayload
	err := json.Unmarshal(payload, &hp)
	if err != nil {
		return nil, err
	}

	if hp.PingInterval == 0 {
		hp.PingInterval = 15000
	}

	return &hp, nil
}

func (e *FilenEventListener) sendEvent(event string, data any) error {
	var message []any
	message = append(message, event)
	if data != nil {
		message = append(message, data)
	}
	payloadBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	fullMessage := string(packetTypeMessage) + string(messageTypeEvent) + string(payloadBytes)
	return e.conn.SendString(fullMessage)
}

func buildWebSocketURL(baseURL string) (string, error) {
	params := map[string]string{
		"EIO":       "3",
		"transport": "websocket",
		"t":         strconv.FormatInt(time.Now().UnixMilli(), 10),
	}

	wsUrl, err := url.Parse(baseURL + "/socket.io/")
	if err != nil {
		return "", err
	}
	query := wsUrl.Query()
	for k, v := range params {
		query.Set(k, v)
	}
	wsUrl.RawQuery = query.Encode()
	return wsUrl.String(), nil
}
