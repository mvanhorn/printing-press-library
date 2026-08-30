// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/bmw-cardata/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/bmw-cardata/internal/config"

	"github.com/eclipse/paho.golang/paho"
	"github.com/spf13/cobra"
)

// pp:data-source live

const (
	cardataStreamHost = "customer.streaming-cardata.bmwgroup.com"
	cardataStreamPort = "9000"
)

// newStreamCmd subscribes to the BMW CarData MQTT v5 telemetry stream and
// persists incoming snapshots to the local store (and echoes them to stdout).
// Auth uses the GCID (username) and id_token (password) captured by
// `auth login`; the streaming scope (cardata:streaming:read) must be granted
// and events subscribed in the BMW portal.
func newStreamCmd(flags *rootFlags) *cobra.Command {
	var (
		flagDB     string
		flagFor    string
		flagFollow bool
	)
	cmd := &cobra.Command{
		Use:   "stream <vin>",
		Short: "Stream live BMW CarData telematic data over MQTT and persist it locally.",
		Long: `Connect to the BMW CarData MQTT v5 stream for a vehicle and append every
incoming telematic snapshot to the local store (so soc-trends/trips keep updating
in real time). Requires:
  - auth login completed (GCID + id_token stored)
  - the cardata:streaming:read scope granted in the BMW portal
  - the desired events subscribed when the CarData client was created`,
		Example:     "  bmw-cardata-pp-cli stream WBAJB3105JUV12345 --follow",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would connect to the BMW CarData MQTT stream")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a VIN is required"))
			}
			vin := args[0]
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(fmt.Errorf("loading config: %w", err))
			}
			sess, err := loadCardataSession(cfg)
			if err != nil {
				return authErr(fmt.Errorf("no streaming session found; run 'auth login' first (with the streaming scope): %w", err))
			}
			gcid := sess["gcid"]
			idToken := sess["id_token"]
			if gcid == "" || idToken == "" {
				return authErr(fmt.Errorf("streaming session missing GCID/id_token; re-run 'auth login' with the streaming scope"))
			}
			dbPath := resolveDBPath(flagDB)

			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would stream MQTT v5 for %s from %s:%s\n", vin, cardataStreamHost, cardataStreamPort)
				return nil
			}

			topic := gcid + "/" + vin
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			if !flagFollow {
				if d, err := cliutil.ParseDurationLoose(flagFor); err == nil && d > 0 {
					ctx, cancel = context.WithTimeout(ctx, d)
					defer cancel()
				}
			} else {
				// Honor Ctrl-C during --follow.
				sig := make(chan os.Signal, 1)
				signal.Notify(sig, os.Interrupt)
				go func() { <-sig; cancel() }()
			}

			received, err := runCardataStream(ctx, gcid, idToken, vin, topic, dbPath, cmd.OutOrStdout())
			if err != nil {
				return apiErr(fmt.Errorf("MQTT stream: %w", err))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "stream ended after %d message(s)\n", received)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	cmd.Flags().StringVar(&flagFor, "for", "60s", "Duration to stream (e.g. 60s, 10m); ignored with --follow")
	cmd.Flags().BoolVar(&flagFollow, "follow", false, "Stream until interrupted (Ctrl-C)")
	return cmd
}

// runCardataStream dials the BMW MQTT v5 broker, authenticates with
// (GCID, id_token), subscribes to topic, and persists each message until ctx
// is done. Returns the number of messages received.
func runCardataStream(ctx context.Context, gcid, idToken, vin, topic, dbPath string, w io.Writer) (int, error) {
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	conn, err := tls.Dial("tcp", cardataStreamHost+":"+cardataStreamPort, tlsCfg)
	if err != nil {
		return 0, fmt.Errorf("dialing broker: %w", err)
	}
	defer conn.Close()

	clientID := "bmw-cardata-pp-cli-" + randHex(6)
	var received int
	client := paho.NewClient(paho.ClientConfig{
		Conn:     conn,
		ClientID: clientID,
		OnPublishReceived: []func(paho.PublishReceived) (bool, error){
			func(pr paho.PublishReceived) (bool, error) {
				received++
				persistStreamMessage(dbPath, vin, pr.Packet.Payload, w)
				return true, nil
			},
		},
		OnClientError: func(err error) {
			fmt.Fprintf(os.Stderr, "mqtt client error: %v\n", err)
		},
	})

	cp := &paho.Connect{
		KeepAlive:  30,
		ClientID:   clientID,
		CleanStart: true,
		Username:   gcid,
		Password:   []byte(idToken),
	}
	connAck, err := client.Connect(ctx, cp)
	if err != nil {
		return 0, fmt.Errorf("connect (check GCID/id_token + streaming scope): %w", err)
	}
	if connAck == nil || connAck.ReasonCode != 0 {
		rc := byte(0xff)
		if connAck != nil {
			rc = connAck.ReasonCode
		}
		return 0, fmt.Errorf("broker rejected connection (reason %d); verify the cardata:streaming:read scope and subscribed events", rc)
	}
	fmt.Fprintf(w, "connected; subscribed to %s\n", topic)

	if _, err := client.Subscribe(ctx, &paho.Subscribe{
		Subscriptions: []paho.SubscribeOptions{{Topic: topic, QoS: 1}},
	}); err != nil {
		return 0, fmt.Errorf("subscribing to %s: %w", topic, err)
	}

	<-ctx.Done()
	client.Disconnect(&paho.Disconnect{ReasonCode: 0})
	return received, nil
}

// persistStreamMessage normalizes an incoming stream payload into the
// telematic-data store shape and appends it. Stream payloads carry the data
// under either "data" or "telematicData".
func persistStreamMessage(dbPath, vin string, payload []byte, w io.Writer) {
	var msg map[string]json.RawMessage
	if json.Unmarshal(payload, &msg) != nil {
		return
	}
	data, ok := msg["data"]
	if !ok {
		data, ok = msg["telematicData"]
	}
	if !ok {
		return
	}
	wrapped, _ := json.Marshal(map[string]json.RawMessage{"telematicData": data})
	persistCardataTelematicData(dbPath, vin, wrapped)
	fmt.Fprintf(w, "+ %s\n", string(payload))
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// keep time referenced for potential future pacing
var _ = time.Second
