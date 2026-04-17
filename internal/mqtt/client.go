package mqtt

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"zimaos-monitor/internal/config"
)

// Client wraps a paho MQTT client with auto-reconnect.
type Client struct {
	inner paho.Client
	cfg   *config.Config
}

func NewClient(cfg *config.Config) (*Client, error) {
	broker := cfg.MQTT.Broker
	clientID := cfg.MQTT.ClientID
	hasAuth := cfg.MQTT.Username != ""

	log.Printf("mqtt: connecting to %s (client_id=%q auth=%v)", broker, clientID, hasAuth)

	opts := paho.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	if hasAuth {
		opts.SetUsername(cfg.MQTT.Username)
		opts.SetPassword(cfg.MQTT.Password)
	}
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetKeepAlive(30 * time.Second)
	opts.SetOnConnectHandler(func(_ paho.Client) {
		log.Printf("mqtt: connected to %s", broker)
	})
	opts.SetConnectionLostHandler(func(_ paho.Client, err error) {
		log.Printf("mqtt: connection lost (%s): %v — will retry every 5s", broker, err)
	})
	opts.SetReconnectingHandler(func(_ paho.Client, _ *paho.ClientOptions) {
		log.Printf("mqtt: reconnecting to %s...", broker)
	})

	inner := paho.NewClient(opts)
	token := inner.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		// Distinguish common error types to make journalctl output actionable.
		errStr := err.Error()
		switch {
		case containsAny(errStr, "connection refused", "no route to host", "i/o timeout", "no such host"):
			log.Printf("mqtt: cannot reach broker at %s — check the address and port", broker)
		case containsAny(errStr, "not authorised", "bad user name or password", "CONNACK"):
			log.Printf("mqtt: broker rejected credentials for user %q", cfg.MQTT.Username)
		}
		return nil, fmt.Errorf("mqtt connect to %s: %w", broker, err)
	}

	return &Client{inner: inner, cfg: cfg}, nil
}

func containsAny(s string, subs ...string) bool {
	sl := strings.ToLower(s)
	for _, sub := range subs {
		if strings.Contains(sl, sub) {
			return true
		}
	}
	return false
}

// Publish sends a payload to the given topic. retained=true for discovery configs.
// A nil payload with retained=true clears a previously retained message.
func (c *Client) Publish(topic string, payload []byte, retained bool) error {
	token := c.inner.Publish(topic, 0, retained, payload)
	token.Wait()
	return token.Error()
}

// CollectRetained subscribes to topicFilter and collects retained messages from the broker.
// Returns after `idleGrace` with no new messages, or `timeout` total — whichever comes first.
func (c *Client) CollectRetained(topicFilter string, timeout time.Duration) (map[string][]byte, error) {
	const idleGrace = 300 * time.Millisecond

	collected := make(map[string][]byte)
	var mu sync.Mutex

	// Timer starts at full timeout (no-messages case); each incoming message resets it
	// to idleGrace so we stop shortly after the retained burst finishes.
	timer := time.NewTimer(timeout)

	handler := func(_ paho.Client, msg paho.Message) {
		mu.Lock()
		collected[msg.Topic()] = msg.Payload()
		mu.Unlock()
		timer.Reset(idleGrace)
	}

	token := c.inner.Subscribe(topicFilter, 0, handler)
	token.Wait()
	if err := token.Error(); err != nil {
		timer.Stop()
		return nil, fmt.Errorf("subscribe %s: %w", topicFilter, err)
	}

	<-timer.C
	c.inner.Unsubscribe(topicFilter).Wait()

	mu.Lock()
	defer mu.Unlock()
	out := make(map[string][]byte, len(collected))
	for k, v := range collected {
		out[k] = v
	}
	return out, nil
}

func (c *Client) Disconnect() {
	c.inner.Disconnect(250)
}
