package bambu

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Client struct {
	serial  string
	monitor *Monitor
	mqtt    mqtt.Client
	reports chan []byte
	mu      sync.Mutex
}

func Connect(ctx context.Context, host, serial, accessCode string) (*Client, error) {
	if host == "" {
		return nil, fmt.Errorf("printer host is required")
	}
	if !serialPattern.MatchString(serial) {
		return nil, fmt.Errorf("BAMBU_SERIAL must contain 12-20 letters or digits")
	}
	if accessCode == "" {
		return nil, fmt.Errorf("BAMBU_ACCESS_CODE is required")
	}
	tlsConfig, err := TLSConfig(serial)
	if err != nil {
		return nil, err
	}
	var clientNonce [4]byte
	if _, err := cryptorand.Read(clientNonce[:]); err != nil {
		return nil, fmt.Errorf("generate MQTT client identifier: %w", err)
	}
	c := &Client{serial: serial, monitor: NewMonitor(serial), reports: make(chan []byte, 32)}
	options := mqtt.NewClientOptions().
		AddBroker("ssl://" + host + ":8883").
		SetClientID(fmt.Sprintf("bambu-pp-%x", clientNonce)).
		SetUsername("bblp").
		SetPassword(accessCode).
		SetTLSConfig(tlsConfig).
		SetCleanSession(true).
		SetAutoReconnect(false).
		SetConnectTimeout(8 * time.Second).
		SetWriteTimeout(8 * time.Second)
	options.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		select {
		case c.reports <- []byte(`{"_connection_error":true}`):
		default:
		}
	})
	c.mqtt = mqtt.NewClient(options)
	if err := waitToken(ctx, c.mqtt.Connect()); err != nil {
		return nil, fmt.Errorf("connect MQTT: %w", err)
	}
	reportTopic := fmt.Sprintf("device/%s/report", serial)
	token := c.mqtt.Subscribe(reportTopic, 0, func(_ mqtt.Client, message mqtt.Message) {
		copyPayload := append([]byte(nil), message.Payload()...)
		select {
		case c.reports <- copyPayload:
		default:
		}
	})
	if err := waitToken(ctx, token); err != nil {
		c.Close()
		return nil, fmt.Errorf("subscribe MQTT report: %w", err)
	}
	return c, nil
}

func (c *Client) Close() {
	if c != nil && c.mqtt != nil && c.mqtt.IsConnected() {
		c.mqtt.Disconnect(250)
	}
}

func (c *Client) RequestPushAll(ctx context.Context) error {
	payload, _ := json.Marshal(map[string]any{"pushing": map[string]string{"sequence_id": "0", "command": "pushall"}})
	token := c.mqtt.Publish(fmt.Sprintf("device/%s/request", c.serial), 0, false, payload)
	if err := waitToken(ctx, token); err != nil {
		return fmt.Errorf("publish MQTT pushall: %w", err)
	}
	return nil
}

func (c *Client) Next(ctx context.Context) (Snapshot, []Event, error) {
	select {
	case <-ctx.Done():
		return Snapshot{}, nil, ctx.Err()
	case payload := <-c.reports:
		var marker map[string]any
		if json.Unmarshal(payload, &marker) == nil {
			if failed, _ := marker["_connection_error"].(bool); failed {
				return Snapshot{}, nil, fmt.Errorf("MQTT connection lost")
			}
		}
		c.mu.Lock()
		events, err := c.monitor.Ingest(payload)
		snapshot := c.monitor.Snapshot()
		c.mu.Unlock()
		return snapshot, events, err
	}
}

func (c *Client) Status(ctx context.Context) (Snapshot, error) {
	if err := c.RequestPushAll(ctx); err != nil {
		return Snapshot{}, err
	}
	for {
		snapshot, _, err := c.Next(ctx)
		if err != nil {
			return Snapshot{}, err
		}
		if snapshot.ObservedAt.IsZero() || snapshot.State == "UNKNOWN" {
			continue
		}
		return snapshot, nil
	}
}

func waitToken(ctx context.Context, token mqtt.Token) error {
	done := make(chan struct{})
	go func() {
		token.Wait()
		close(done)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return token.Error()
	}
}
