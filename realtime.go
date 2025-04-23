package supabase

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Realtime struct {
	client         *Client
	realtimeClient *RealtimeClient
}

// Initialize sets up the Realtime client with the base URL and API key
func (r *Realtime) Initialize() error {
	if r.client == nil {
		return errors.New("client not initialized")
	}

	// Convert HTTP URL to WebSocket URL
	realtimeURL := r.client.BaseURL + "/realtime/v1"
	realtimeURL = strings.Replace(realtimeURL, "http://", "ws://", 1)
	realtimeURL = strings.Replace(realtimeURL, "https://", "wss://", 1)

	params := map[string]string{
		"apikey": r.client.apiKey,
	}

	r.realtimeClient = NewRealtimeClient(realtimeURL, params, nil)
	return nil
}

// Channel creates a new Realtime channel
func (r *Realtime) Channel(name string) (*Channel, error) {
	if r.realtimeClient == nil {
		if err := r.Initialize(); err != nil {
			return nil, err
		}
	}

	return NewChannel(name, r.realtimeClient, nil), nil
}

// Connect establishes a WebSocket connection
func (r *Realtime) Connect() error {
	if r.realtimeClient == nil {
		if err := r.Initialize(); err != nil {
			return err
		}
	}

	// Create a new transport if needed
	if r.realtimeClient.conn == nil {
		r.realtimeClient.conn = NewGorillaWebSocketTransport()
	}

	// Set up message handler
	r.realtimeClient.conn.OnMessage(func(data []byte) {
		r.realtimeClient.handleMessage(data)
	})

	// Connect using the transport
	return r.realtimeClient.conn.Connect(
		r.realtimeClient.url,
		r.realtimeClient.params,
		r.realtimeClient.headers,
	)
}

// RealtimeClient for realtime communication
type RealtimeClient struct {
	conn        *GorillaWebSocketTransport
	url         string
	params      map[string]string
	headers     map[string]string
	ref         int
	callbacks   map[string]func(Message)
	channels    map[string]*Channel
	status      ConnectionStatus
	mu          sync.Mutex
	callbacksMu sync.RWMutex
	channelsMu  sync.RWMutex
}

func NewRealtimeClient(url string, params map[string]string, headers map[string]string) *RealtimeClient {
	return &RealtimeClient{
		url:      url,
		params:   params,
		headers:  headers,
		channels: make(map[string]*Channel),
	}
}

// makeRef creates and sends a message to the server
func (c *RealtimeClient) makeRef(event string, topic string, payload map[string]interface{}) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ref++
	ref := fmt.Sprintf("%d", c.ref)

	message := Message{
		Event:   event,
		Topic:   topic,
		Payload: payload,
		Ref:     ref,
	}

	data, err := json.Marshal(message)
	if err != nil {
		return "", err
	}

	if c.conn == nil || c.status != StatusOpen {
		return "", errors.New("not connected")
	}

	err = c.conn.Send(data)
	if err != nil {
		return "", err
	}

	return ref, nil
}

// registerCallback registers a callback for a specific reference
func (c *RealtimeClient) registerCallback(ref string, callback func(Message)) {
	c.callbacksMu.Lock()
	defer c.callbacksMu.Unlock()

	if c.callbacks == nil {
		c.callbacks = make(map[string]func(Message))
	}
	c.callbacks[ref] = callback
}

// handleMessage processes incoming messages and routes them to channels
func (c *RealtimeClient) handleMessage(data []byte) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	// Check if we have a callback for this message reference
	c.callbacksMu.RLock()
	callback, ok := c.callbacks[msg.Ref]
	c.callbacksMu.RUnlock()

	if ok && callback != nil {
		callback(msg)
	}

	// Route message to the appropriate channel
	c.channelsMu.RLock()
	channel, ok := c.channels[msg.Topic]
	c.channelsMu.RUnlock()

	if ok && channel != nil {
		channel.on(msg)
	}
}

// ConnectionStatus for WebSocket connection
type ConnectionStatus string

const (
	StatusClosed     ConnectionStatus = "CLOSED"
	StatusConnecting ConnectionStatus = "CONNECTING"
	StatusOpen       ConnectionStatus = "OPEN"
)

// ChannelStatus for realtime channels
type ChannelStatus string

const (
	ChannelStatusClosed  ChannelStatus = "CLOSED"
	ChannelStatusJoining ChannelStatus = "JOINING"
	ChannelStatusJoined  ChannelStatus = "JOINED"
	ChannelStatusLeaving ChannelStatus = "LEAVING"
	ChannelStatusError   ChannelStatus = "ERROR"
)

// ClientOptions for Realtime client configuration
type ClientOptions struct {
	ConnectionTimeout  time.Duration
	HeartbeatInterval  time.Duration
	Headers            map[string]string
	Params             map[string]string
	AccessToken        GetAccessToken
	Transport          WebSocketTransport
	Logger             Logger
	ReconnectAfterMs   func(tries int) time.Duration
	HeartbeatTimeoutMs int
}

// ChannelOptions for channel configuration
type ChannelOptions struct {
	Config               map[string]interface{}
	Params               map[string]string
	RetryAfterMs         func(tries int) time.Duration
	RetryJoinUntil       time.Duration
	BroadcastEndpointURL string
	PresenceEndpointURL  string
}

// Message from a channel
type Message struct {
	Event   string                 `json:"event"`
	Topic   string                 `json:"topic"`
	Payload map[string]interface{} `json:"payload"`
	Ref     string                 `json:"ref,omitempty"`
	JoinRef string                 `json:"join_ref,omitempty"`
}

// PresenceState map
type PresenceState map[string]map[string]interface{}

// PresenceDiff for presence changes
type PresenceDiff struct {
	Joins  map[string]map[string]interface{} `json:"joins"`
	Leaves map[string]map[string]interface{} `json:"leaves"`
}

// BroadcastParams for message broadcasting
type BroadcastParams struct {
	Type    string      `json:"type"`
	Event   string      `json:"event"`
	Payload interface{} `json:"payload"`
}

// SubscribeParams for topic subscription
type SubscribeParams struct {
	PostgresChanges []PostgresChange `json:"postgres_changes,omitempty"`
	Broadcast       BroadcastConfig  `json:"broadcast,omitempty"`
	Presence        PresenceConfig   `json:"presence,omitempty"`
}

// PostgresChange subscription
type PostgresChange struct {
	Event   string   `json:"event"`
	Schema  string   `json:"schema"`
	Table   string   `json:"table"`
	Filter  string   `json:"filter,omitempty"`
	Columns []string `json:"columns,omitempty"`
}

// BroadcastConfig settings
type BroadcastConfig struct {
	Self   bool     `json:"self"`
	Ack    bool     `json:"ack"`
	Events []string `json:"events"`
}

// PresenceConfig settings
type PresenceConfig struct {
	Key string `json:"key"`
}

// Push message for WebSocket
type Push struct {
	Event   string
	Topic   string
	Payload map[string]interface{}
	Ref     string
	JoinRef string
}

// GetAccessToken function type
type GetAccessToken func() (string, error)

// EventHandler function type
type EventHandler func(message Message)

// WebSocketTransport interface
type WebSocketTransport interface {
	Connect(url string, params map[string]string, headers map[string]string) error
	Disconnect(code int, reason string) error
	Send(data []byte) error
	OnOpen(callback func()) error
	OnClose(callback func(code int, reason string)) error
	OnError(callback func(err error)) error
	OnMessage(callback func(data []byte)) error
}

// Logger interface
type Logger interface {
	Debug(args ...interface{})
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
}

// PusherEvent for Pusher compatibility
type PusherEvent struct {
	Channel string      `json:"channel"`
	Event   string      `json:"event"`
	Data    interface{} `json:"data"`
}

// RealtimePostgresChangesPayload for PostgreSQL changes
type RealtimePostgresChangesPayload struct {
	Commit  string                 `json:"commit_timestamp"`
	Errors  []string               `json:"errors"`
	Schema  string                 `json:"schema"`
	Table   string                 `json:"table"`
	Type    string                 `json:"type"`
	Old     map[string]interface{} `json:"old,omitempty"`
	New     map[string]interface{} `json:"new,omitempty"`
	Columns []ColumnData           `json:"columns,omitempty"`
}

// ColumnData for PostgreSQL column information
type ColumnData struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	DataType string `json:"data_type"`
}

// Channel represents a Realtime channel.
type Channel struct {
	name            string
	client          *RealtimeClient
	options         *ChannelOptions
	status          ChannelStatus
	handlers        map[string][]EventHandler
	presenceState   PresenceState
	joinPush        *Push
	rejoinTimer     *time.Timer
	pushBuffer      []*Push
	subscribeParams *SubscribeParams
	joinRef         string
	handlersMu      sync.RWMutex
	presenceStateMu sync.RWMutex
	pushBufferMu    sync.Mutex
}

// GorillaWebSocketTransport implements WebSocketTransport using gorilla/websocket
type GorillaWebSocketTransport struct {
	conn        *websocket.Conn
	onOpen      func()
	onClose     func(code int, reason string)
	onError     func(err error)
	onMessage   func(data []byte)
	mu          sync.Mutex
	initialized bool
	done        chan struct{}
}

// NewChannel creates a new channel.
func NewChannel(name string, client *RealtimeClient, options *ChannelOptions) *Channel {
	if options == nil {
		options = &ChannelOptions{
			Config: make(map[string]interface{}),
		}
	}

	channel := &Channel{
		name:            name,
		client:          client,
		options:         options,
		status:          ChannelStatusClosed,
		handlers:        make(map[string][]EventHandler),
		presenceState:   make(PresenceState),
		pushBuffer:      make([]*Push, 0),
		subscribeParams: &SubscribeParams{},
	}

	// Register this channel with the client
	client.channelsMu.Lock()
	client.channels[name] = channel
	client.channelsMu.Unlock()

	return channel
}

// Subscribe subscribes to all events on the channel.
func (c *Channel) Subscribe(callback EventHandler) error {
	return c.SubscribeToEvent("*", callback)
}

// SubscribeToEvent subscribes to a specific event on the channel.
func (c *Channel) SubscribeToEvent(event string, callback EventHandler) error {
	// First, ensure the channel is joined
	if c.status != ChannelStatusJoined {
		if err := c.join(); err != nil {
			return err
		}
	}

	// Register the event handler
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()

	if c.handlers == nil {
		c.handlers = make(map[string][]EventHandler)
	}

	c.handlers[event] = append(c.handlers[event], callback)
	return nil
}

// Unsubscribe unsubscribes from the channel.
func (c *Channel) Unsubscribe() error {
	if c.status != ChannelStatusJoined {
		return nil
	}

	// Set status to leaving
	c.status = ChannelStatusLeaving

	// Use the client's makeRef method to send the leave message
	_, err := c.client.makeRef("phx_leave", c.name, map[string]interface{}{})
	if err != nil {
		return err
	}

	// Clear handlers
	c.handlersMu.Lock()
	c.handlers = make(map[string][]EventHandler)
	c.handlersMu.Unlock()

	// Clear presence state
	c.presenceStateMu.Lock()
	c.presenceState = make(PresenceState)
	c.presenceStateMu.Unlock()

	// Stop rejoin timer if it's running
	if c.rejoinTimer != nil {
		c.rejoinTimer.Stop()
		c.rejoinTimer = nil
	}

	c.status = ChannelStatusClosed
	return nil
}

// Join joins the channel.
func (c *Channel) Join() error {
	return c.join()
}

// join joins the channel (internal implementation).
func (c *Channel) join() error {
	if c.status == ChannelStatusJoined || c.status == ChannelStatusJoining {
		return nil
	}

	// Set status to joining
	c.status = ChannelStatusJoining

	// Create join push
	joinPayload := map[string]interface{}{
		"config": c.options.Config,
	}

	// Add custom params if provided
	if c.options.Params != nil {
		for k, v := range c.options.Params {
			joinPayload[k] = v
		}
	}

	// Include subscribe params if we have any
	if len(c.subscribeParams.PostgresChanges) > 0 ||
		len(c.subscribeParams.Broadcast.Events) > 0 ||
		c.subscribeParams.Presence.Key != "" {
		jsonBytes, err := json.Marshal(c.subscribeParams)
		if err == nil {
			var asMap map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &asMap); err == nil {
				for k, v := range asMap {
					joinPayload[k] = v
				}
			}
		}
	}

	c.joinPush = &Push{
		Event:   "phx_join",
		Topic:   c.name,
		Payload: joinPayload,
	}

	// Register a callback for the join response
	ref, err := c.client.makeRef("phx_join", c.name, joinPayload)
	if err != nil {
		c.status = ChannelStatusError
		return err
	}

	c.joinRef = ref

	// Register callback for join response
	c.client.registerCallback(ref, func(message Message) {
		if status, ok := message.Payload["status"].(string); ok {
			if status == "ok" {
				c.status = ChannelStatusJoined

				// Send any buffered messages
				c.pushBufferMu.Lock()
				for _, push := range c.pushBuffer {
					c.pushMessage(push)
				}
				c.pushBuffer = make([]*Push, 0)
				c.pushBufferMu.Unlock()
			} else {
				c.status = ChannelStatusError
			}
		}
	})

	return nil
}

// on handles incoming messages from the server.
func (c *Channel) on(message Message) {
	// Special handling for presence events
	if message.Event == "presence_state" {
		var state PresenceState
		if jsonBytes, err := json.Marshal(message.Payload); err == nil {
			if err := json.Unmarshal(jsonBytes, &state); err == nil {
				c.presenceStateMu.Lock()
				c.presenceState = state
				c.presenceStateMu.Unlock()
			}
		}
	} else if message.Event == "presence_diff" {
		var diff PresenceDiff
		if jsonBytes, err := json.Marshal(message.Payload); err == nil {
			if err := json.Unmarshal(jsonBytes, &diff); err == nil {
				c.presenceStateMu.Lock()
				// Apply joins
				for key, presence := range diff.Joins {
					c.presenceState[key] = presence
				}
				// Apply leaves
				for key := range diff.Leaves {
					delete(c.presenceState, key)
				}
				c.presenceStateMu.Unlock()
			}
		}
	}

	c.handlersMu.RLock()
	defer c.handlersMu.RUnlock()

	// Call handlers for the specific event
	if handlers, ok := c.handlers[message.Event]; ok {
		for _, handler := range handlers {
			handler(message)
		}
	}

	// Call wildcard handlers
	if handlers, ok := c.handlers["*"]; ok {
		for _, handler := range handlers {
			handler(message)
		}
	}
}

// Broadcast sends a broadcast message on the channel.
func (c *Channel) Broadcast(event string, payload interface{}) error {
	if c.status != ChannelStatusJoined {
		return errors.New("channel not joined")
	}

	// Create broadcast push
	payloadMap := make(map[string]interface{})
	if jsonBytes, err := json.Marshal(payload); err == nil {
		if err := json.Unmarshal(jsonBytes, &payloadMap); err != nil {
			// If it's not a map, use the raw payload
			payloadMap = map[string]interface{}{
				"data": payload,
			}
		}
	} else {
		// If marshaling fails, use the raw payload
		payloadMap = map[string]interface{}{
			"data": payload,
		}
	}

	// Use the client's makeRef method directly
	_, err := c.client.makeRef(event, c.name, payloadMap)
	return err
}

// Track sends a presence tracking message on the channel.
func (c *Channel) Track(payload interface{}) error {
	if c.status != ChannelStatusJoined {
		return errors.New("channel not joined")
	}

	// Create presence payload
	payloadMap := make(map[string]interface{})
	if jsonBytes, err := json.Marshal(payload); err == nil {
		if err := json.Unmarshal(jsonBytes, &payloadMap); err != nil {
			// If it's not a map, use the raw payload
			payloadMap = map[string]interface{}{
				"data": payload,
			}
		}
	} else {
		// If marshaling fails, use the raw payload
		payloadMap = map[string]interface{}{
			"data": payload,
		}
	}

	// Use the client's makeRef method directly
	_, err := c.client.makeRef("presence_track", c.name, payloadMap)
	return err
}

// Untrack sends a presence untracking message on the channel.
func (c *Channel) Untrack() error {
	if c.status != ChannelStatusJoined {
		return errors.New("channel not joined")
	}

	// Use the client's makeRef method directly
	_, err := c.client.makeRef("presence_untrack", c.name, map[string]interface{}{})
	return err
}

// GetPresenceState returns the current presence state.
func (c *Channel) GetPresenceState() PresenceState {
	c.presenceStateMu.RLock()
	defer c.presenceStateMu.RUnlock()

	// Create a copy to avoid concurrent access issues
	stateCopy := make(PresenceState)
	for k, v := range c.presenceState {
		stateCopy[k] = v
	}

	return stateCopy
}

// SubscribeToPostgresChanges subscribes to PostgreSQL changes.
func (c *Channel) SubscribeToPostgresChanges(changes []PostgresChange) *Channel {
	c.subscribeParams.PostgresChanges = append(c.subscribeParams.PostgresChanges, changes...)
	return c
}

// SubscribeToBroadcast subscribes to broadcast events.
func (c *Channel) SubscribeToBroadcast(events []string, opts BroadcastConfig) *Channel {
	c.subscribeParams.Broadcast.Events = append(c.subscribeParams.Broadcast.Events, events...)
	c.subscribeParams.Broadcast.Self = opts.Self
	c.subscribeParams.Broadcast.Ack = opts.Ack
	return c
}

// SubscribeToPresence subscribes to presence events.
func (c *Channel) SubscribeToPresence(key string) *Channel {
	c.subscribeParams.Presence.Key = key
	return c
}

// pushMessage sends a message over the WebSocket.
// If the channel is not joined, it buffers the message.
func (c *Channel) pushMessage(push *Push) error {
	if c.status != ChannelStatusJoined && push.Event != "phx_join" {
		// Buffer the message to send when we're joined
		c.pushBufferMu.Lock()
		c.pushBuffer = append(c.pushBuffer, push)
		c.pushBufferMu.Unlock()
		return nil
	}

	// Use the client's makeRef method to send the message and get a reference
	ref, err := c.client.makeRef(push.Event, push.Topic, push.Payload)
	if err != nil {
		return err
	}

	// If this is a join message, store the join reference
	if push.Event == "phx_join" {
		c.joinRef = ref
	}

	return nil
}

// Status returns the current status of the channel.
func (c *Channel) Status() ChannelStatus {
	return c.status
}

// Name returns the name of the channel.
func (c *Channel) Name() string {
	return c.name
}

func NewGorillaWebSocketTransport() *GorillaWebSocketTransport {
	return &GorillaWebSocketTransport{
		done: make(chan struct{}),
	}
}

// Connect establishes a WebSocket connection
func (t *GorillaWebSocketTransport) Connect(wsURL string, params map[string]string, headers map[string]string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.conn != nil {
		return errors.New("connection already established")
	}

	// Parse the URL
	u, err := url.Parse(wsURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Add query parameters
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	// Create HTTP header
	header := http.Header{}
	for k, v := range headers {
		header.Set(k, v)
	}

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), header)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	t.conn = conn
	t.initialized = true
	t.done = make(chan struct{})

	// Start reading messages in a goroutine
	go t.readPump()

	// Call onOpen callback if set
	if t.onOpen != nil {
		t.onOpen()
	}

	return nil
}

// Disconnect closes the WebSocket connection
func (t *GorillaWebSocketTransport) Disconnect(code int, reason string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.conn == nil {
		return errors.New("not connected")
	}

	// Signal the read pump to stop
	close(t.done)

	// Send close message
	err := t.conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(code, reason),
		time.Now().Add(time.Second),
	)
	if err != nil {
		// Try to close the connection anyway
		t.conn.Close()
		t.conn = nil
		t.initialized = false
		return fmt.Errorf("failed to send close message: %w", err)
	}

	// Close the connection
	err = t.conn.Close()
	if err != nil {
		return fmt.Errorf("failed to close connection: %w", err)
	}

	t.conn = nil
	t.initialized = false
	return nil
}

// Send sends a message over the WebSocket connection
func (t *GorillaWebSocketTransport) Send(data []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.conn == nil {
		return errors.New("not connected")
	}

	return t.conn.WriteMessage(websocket.TextMessage, data)
}

// readPump continuously reads messages from the WebSocket
func (t *GorillaWebSocketTransport) readPump() {
	defer func() {
		if t.onClose != nil {
			t.onClose(websocket.CloseNormalClosure, "connection closed")
		}
	}()

	for {
		select {
		case <-t.done:
			return
		default:
			_, message, err := t.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(
					err,
					websocket.CloseNormalClosure,
					websocket.CloseGoingAway,
					websocket.CloseAbnormalClosure,
				) && t.onError != nil {
					t.onError(err)
				}
				return
			}

			if t.onMessage != nil {
				t.onMessage(message)
			}
		}
	}
}

// Callbacks
func (t *GorillaWebSocketTransport) OnOpen(callback func()) error {
	t.onOpen = callback
	if t.initialized && t.conn != nil && callback != nil {
		callback()
	}
	return nil
}

func (t *GorillaWebSocketTransport) OnClose(callback func(code int, reason string)) error {
	t.onClose = callback
	return nil
}

func (t *GorillaWebSocketTransport) OnError(callback func(err error)) error {
	t.onError = callback
	return nil
}

func (t *GorillaWebSocketTransport) OnMessage(callback func(data []byte)) error {
	t.onMessage = callback
	return nil
}
