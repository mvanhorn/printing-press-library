// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

func TestClassifyChannelBucket(t *testing.T) {
	cases := []struct {
		name string
		ch   store.Channel
		want string
	}{
		{"im is dm", store.Channel{IsIM: true, Name: "dm:U1"}, bucketDM},
		{"mpim is dm", store.Channel{IsMPIM: true, Name: "mpdm-a-b"}, bucketDM},
		{"general is broadcast", store.Channel{Name: "general"}, bucketBroadcast},
		{"announcements is broadcast", store.Channel{Name: "company-announcements"}, bucketBroadcast},
		{"partner channel", store.Channel{Name: "partner-acme"}, bucketPartner},
		{"ext channel is partner", store.Channel{Name: "ext-customer-x"}, bucketPartner},
		{"plain internal", store.Channel{Name: "engineering"}, bucketInternal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyChannelBucket(tc.ch); got != tc.want {
				t.Errorf("classifyChannelBucket(%q) = %q, want %q", tc.ch.Name, got, tc.want)
			}
		})
	}
}

func TestEmptyUnreadsReport(t *testing.T) {
	r := emptyUnreadsReport()
	if r.TotalUnread != 0 {
		t.Errorf("empty report TotalUnread = %d, want 0", r.TotalUnread)
	}
	for _, b := range unreadBucketOrder {
		if _, ok := r.BucketCounts[b]; !ok {
			t.Errorf("bucket %q missing from BucketCounts", b)
		}
		if items, ok := r.Buckets[b]; !ok || items == nil {
			t.Errorf("bucket %q missing or nil in Buckets — should be empty non-nil slice", b)
		}
	}
}
