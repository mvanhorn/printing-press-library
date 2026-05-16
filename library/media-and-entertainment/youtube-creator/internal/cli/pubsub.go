// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel-feature command (Phase 3).

package cli

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

const defaultWebSubHub = "https://pubsubhubbub.appspot.com/"

func newPubsubCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pubsub",
		Short: "Subscribe to YouTube channel upload notifications via PubSubHubbub (WebSub)",
		Long: `PubSubHubbub (WebSub) is YouTube's push-notification system for channel uploads.
Instead of polling for new videos (quota-burning), subscribe a webhook callback
that receives an HTTP POST when a target channel uploads.

The hub URL is https://pubsubhubbub.appspot.com/ (Google's reference hub).
The topic URL is https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCxxx.

Your callback must respond to a verification GET with the hub.challenge param.
Use 'pubsub verify' to test the callback handshake locally before going live.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newPubsubSubscribeCmd(flags))
	cmd.AddCommand(newPubsubUnsubscribeCmd(flags))
	cmd.AddCommand(newPubsubVerifyCmd(flags))
	return cmd
}

func pubsubTopicForChannel(channelID string) string {
	return "https://www.youtube.com/xml/feeds/videos.xml?channel_id=" + channelID
}

func newPubsubSubscribeCmd(flags *rootFlags) *cobra.Command {
	var channel, callback, hub, secret, leaseSeconds string
	cmd := &cobra.Command{
		Use:   "subscribe",
		Short: "Subscribe a callback URL to a channel's upload feed",
		Long: `Posts a WebSub subscription request to the hub. Your callback URL must be
publicly reachable and respond to the verification GET with the hub.challenge param.

The hub will POST Atom XML to your callback whenever the target channel uploads
a new video. n8n's Webhook node handles both the verification GET and the
content POST out of the box.`,
		Example: "  youtube-creator-pp-cli pubsub subscribe --channel UCxxx --callback https://n8n.example.com/webhook/yt-upload\n" +
			"  youtube-creator-pp-cli pubsub subscribe --channel UCxxx --callback https://example/webhook --secret mySharedSecret",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if channel == "" || callback == "" {
				if flags.dryRun {
					return nil
				}
				return usageErr(fmt.Errorf("--channel and --callback are required"))
			}
			if hub == "" {
				hub = defaultWebSubHub
			}
			topic := pubsubTopicForChannel(channel)
			form := url.Values{
				"hub.callback":     {callback},
				"hub.topic":        {topic},
				"hub.verify":       {"async"},
				"hub.mode":         {"subscribe"},
				"hub.verify_token": {channel},
			}
			if secret != "" {
				form.Set("hub.secret", secret)
			}
			if leaseSeconds != "" {
				form.Set("hub.lease_seconds", leaseSeconds)
			}

			if flags.dryRun {
				return flags.printJSON(cmd, map[string]any{
					"would_post":   hub,
					"form":         form.Encode(),
					"topic":        topic,
					"callback":     callback,
					"verify_async": true,
					"next_step":    "Hub will GET your callback with hub.challenge; respond with that exact value",
				})
			}

			resp, err := http.PostForm(hub, form)
			if err != nil {
				return apiErr(fmt.Errorf("posting to hub: %w", err))
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			result := map[string]any{
				"hub":         hub,
				"topic":       topic,
				"callback":    callback,
				"status_code": resp.StatusCode,
				"response":    strings.TrimSpace(string(body)),
				"next_step":   "Hub will GET your callback with hub.challenge within seconds; respond with that exact value",
			}
			if resp.StatusCode >= 400 {
				return apiErr(fmt.Errorf("hub returned %d: %s", resp.StatusCode, body))
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "Target YouTube channel ID (UCxxxxx) — required")
	cmd.Flags().StringVar(&callback, "callback", "", "Publicly reachable URL the hub will POST to — required")
	cmd.Flags().StringVar(&hub, "hub", defaultWebSubHub, "WebSub hub URL")
	cmd.Flags().StringVar(&secret, "secret", "", "Optional shared secret for HMAC payload verification")
	cmd.Flags().StringVar(&leaseSeconds, "lease-seconds", "", "Subscription lifetime; hub may cap (typical: 432000 = 5 days)")
	return cmd
}

func newPubsubUnsubscribeCmd(flags *rootFlags) *cobra.Command {
	var channel, callback, hub string
	cmd := &cobra.Command{
		Use:         "unsubscribe",
		Short:       "Cancel a WebSub subscription",
		Example:     "  youtube-creator-pp-cli pubsub unsubscribe --channel UCxxx --callback https://example.com/webhook",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if channel == "" || callback == "" {
				if flags.dryRun {
					return nil
				}
				return usageErr(fmt.Errorf("--channel and --callback are required"))
			}
			if hub == "" {
				hub = defaultWebSubHub
			}
			topic := pubsubTopicForChannel(channel)
			form := url.Values{
				"hub.callback":     {callback},
				"hub.topic":        {topic},
				"hub.verify":       {"async"},
				"hub.mode":         {"unsubscribe"},
				"hub.verify_token": {channel},
			}
			if flags.dryRun {
				return flags.printJSON(cmd, map[string]any{
					"would_post": hub,
					"form":       form.Encode(),
				})
			}
			resp, err := http.PostForm(hub, form)
			if err != nil {
				return apiErr(fmt.Errorf("posting to hub: %w", err))
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			return flags.printJSON(cmd, map[string]any{
				"hub":         hub,
				"topic":       topic,
				"status_code": resp.StatusCode,
				"response":    strings.TrimSpace(string(body)),
			})
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "Target channel ID (UCxxxxx) — required")
	cmd.Flags().StringVar(&callback, "callback", "", "Callback URL to unsubscribe — required")
	cmd.Flags().StringVar(&hub, "hub", defaultWebSubHub, "WebSub hub URL")
	return cmd
}

func newPubsubVerifyCmd(flags *rootFlags) *cobra.Command {
	var callback, channel string
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Probe a callback URL to confirm it handles the WebSub verification GET correctly",
		Long: `Sends a synthetic verification GET to the callback URL with hub.mode=subscribe,
hub.topic, hub.challenge, and hub.verify_token, then checks that the callback
echoes back the challenge string in its response body.

This is the same handshake the real hub will perform during 'pubsub subscribe',
so use this to validate your n8n webhook (or other callback) before subscribing.`,
		Example:     "  youtube-creator-pp-cli pubsub verify --callback https://n8n.example.com/webhook/yt --channel UCxxx",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if callback == "" || channel == "" {
				if flags.dryRun {
					return nil
				}
				return usageErr(fmt.Errorf("--callback and --channel are required"))
			}
			challenge := "pp-cli-challenge-" + channel
			topic := pubsubTopicForChannel(channel)
			params := url.Values{
				"hub.mode":          {"subscribe"},
				"hub.topic":         {topic},
				"hub.challenge":     {challenge},
				"hub.lease_seconds": {"432000"},
				"hub.verify_token":  {channel},
			}
			probeURL := callback + "?" + params.Encode()
			if strings.Contains(callback, "?") {
				probeURL = callback + "&" + params.Encode()
			}

			if flags.dryRun {
				return flags.printJSON(cmd, map[string]any{
					"would_get":     probeURL,
					"expected_body": challenge,
				})
			}

			resp, err := http.Get(probeURL)
			if err != nil {
				return apiErr(fmt.Errorf("probing callback: %w", err))
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			gotBody := strings.TrimSpace(string(body))
			pass := resp.StatusCode == 200 && gotBody == challenge

			return flags.printJSON(cmd, map[string]any{
				"callback":      callback,
				"status_code":   resp.StatusCode,
				"expected_body": challenge,
				"actual_body":   gotBody,
				"verification":  pass,
				"hint":          "If verification=false, your callback handler is not echoing the hub.challenge param back in the response body verbatim.",
			})
		},
	}
	cmd.Flags().StringVar(&callback, "callback", "", "Callback URL to probe — required")
	cmd.Flags().StringVar(&channel, "channel", "", "Channel ID used as verify_token — required")
	return cmd
}
