// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/xml"
	"fmt"

	"github.com/spf13/cobra"
)

// rssEnvelope is the relevant slice of Redfin's RSS-style XML feeds.
type rssEnvelope struct {
	XMLName xml.Name `xml:"rss"`
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title    string `xml:"title"`
	Link     string `xml:"link"`
	Date     string `xml:"pubDate"`
	GUID     string `xml:"guid"`
	Source   string `xml:"source"`
	Address  string `xml:"address"`
	City     string `xml:"city"`
	State    string `xml:"state"`
	Region   string `xml:"region"`
	RegionID string `xml:"regionId"`
	Price    string `xml:"price"`
}

// urlsetEnvelope is the slice of Redfin's sitemap-style XML feeds.
type urlsetEnvelope struct {
	XMLName xml.Name `xml:"urlset"`
	Urls    []struct {
		Loc      string `xml:"loc"`
		LastMod  string `xml:"lastmod"`
		Priority string `xml:"priority"`
	} `xml:"url"`
}

// feedItem normalizes both shapes (RSS and sitemap urlset) into one row.
type feedItem struct {
	URL      string `json:"url"`
	Title    string `json:"title,omitempty"`
	Date     string `json:"date,omitempty"`
	Region   string `json:"region,omitempty"`
	RegionID string `json:"region_id,omitempty"`
	City     string `json:"city,omitempty"`
	State    string `json:"state,omitempty"`
	Price    string `json:"price,omitempty"`
}

func newFeedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "feed",
		Short:       "Read Redfin's public XML feeds (newest listings, latest updates).",
		Long:        `Subcommands for parsing the public RSS/sitemap-style XML feeds Redfin publishes for newest listings and latest updates.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newFeedNewCmd(flags))
	cmd.AddCommand(newFeedUpdatesCmd(flags))
	return cmd
}

func newFeedNewCmd(flags *rootFlags) *cobra.Command {
	var regionID string
	cmd := &cobra.Command{
		Use:         "new",
		Short:       "Parse the newest_listings.xml feed.",
		Example:     `  redfin-pp-cli feed new --region-id 30772 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, gerr := c.Get("/newest_listings.xml", nil)
			if gerr != nil {
				return classifyAPIError(gerr)
			}
			var env rssEnvelope
			if err := xml.Unmarshal(data, &env); err != nil {
				return apiErr(fmt.Errorf("decoding RSS: %w", err))
			}
			var out []feedItem
			for _, it := range env.Channel.Items {
				if regionID != "" && it.RegionID != regionID {
					continue
				}
				out = append(out, feedItem{
					URL:      it.Link,
					Title:    it.Title,
					Date:     it.Date,
					Region:   it.Region,
					RegionID: it.RegionID,
					City:     it.City,
					State:    it.State,
					Price:    it.Price,
				})
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&regionID, "region-id", "", "Filter to one region ID (when populated by the feed)")
	return cmd
}

func newFeedUpdatesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "updates",
		Short:       "Parse the sitemap_com_latest_updates.xml feed.",
		Example:     `  redfin-pp-cli feed updates --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, gerr := c.Get("/sitemap_com_latest_updates.xml", nil)
			if gerr != nil {
				return classifyAPIError(gerr)
			}
			var env urlsetEnvelope
			if err := xml.Unmarshal(data, &env); err != nil {
				return apiErr(fmt.Errorf("decoding sitemap: %w", err))
			}
			out := make([]feedItem, 0, len(env.Urls))
			for _, u := range env.Urls {
				out = append(out, feedItem{URL: u.Loc, Date: u.LastMod})
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}
