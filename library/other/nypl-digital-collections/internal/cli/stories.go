package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type storyDiscoveryPlan struct {
	Subject  string          `json:"subject"`
	Intent   string          `json:"intent"`
	Queries  []string        `json:"queries"`
	Clusters []storyCluster  `json:"clusters"`
	DryRun   bool            `json:"dry_run,omitempty"`
	Results  []storyEvidence `json:"results,omitempty"`
}

type storyCluster struct {
	ID                string             `json:"id"`
	Label             string             `json:"label"`
	Why               string             `json:"why"`
	Searches          []string           `json:"searches"`
	SimilarCharacters []similarCharacter `json:"similar_characters,omitempty"`
}

type similarCharacter struct {
	Name   string   `json:"name"`
	Why    string   `json:"why"`
	Search []string `json:"search"`
}

type storyEvidence struct {
	Cluster string              `json:"cluster"`
	Query   string              `json:"query"`
	Items   []storyEvidenceItem `json:"items"`
	Error   string              `json:"error,omitempty"`
}

type storyEvidenceItem struct {
	Title          string `json:"title,omitempty"`
	TypeOfResource string `json:"type_of_resource,omitempty"`
	ItemLink       string `json:"item_link,omitempty"`
	ImageID        string `json:"image_id,omitempty"`
	UUID           string `json:"uuid,omitempty"`
}

func newStoriesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stories",
		Short: "Discover story-rich NYPL searches and similar character archetypes",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
	}
	cmd.AddCommand(newStoriesDiscoverCmd(flags))
	cmd.AddCommand(newStoriesDossierCmd(flags))
	return cmd
}

func newStoriesDiscoverCmd(flags *rootFlags) *cobra.Command {
	var perCluster int
	var publicDomainOnly bool
	cmd := &cobra.Command{
		Use:   "discover <character-or-topic>",
		Short: "Build a story discovery plan and optionally fetch Digital Collections evidence",
		Long: `Build a story discovery plan for a character or topic.

The command expands one seed into NYPL-friendly searches, narrative clusters,
and similar historical/literary characters. Use --dry-run to print the plan
without calling the API. Without --dry-run it searches NYPL Digital Collections
for the cluster queries and returns compact evidence links.`,
		Example: `  nypl-digital-collections-pp-cli stories discover "Anne Boleyn" --dry-run --json
  nypl-digital-collections-pp-cli stories discover "Anne Boleyn" --per-cluster 2 --public-domain-only --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if perCluster < 1 {
				return usageErr(fmt.Errorf("--per-cluster must be at least 1"))
			}
			plan := buildStoryDiscoveryPlan(args[0], perCluster)
			if flags.dryRun {
				plan.DryRun = true
				return printStoryPlan(cmd, flags, plan)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			for _, cluster := range plan.Clusters {
				for _, query := range cluster.Searches {
					params := map[string]string{
						"q":        query,
						"per_page": fmt.Sprintf("%d", perCluster),
					}
					if publicDomainOnly {
						params["publicDomainOnly"] = "true"
					}
					data, err := c.Get(cmd.Context(), "/items/search", params)
					evidence := storyEvidence{Cluster: cluster.ID, Query: query}
					if err != nil {
						evidence.Error = err.Error()
					} else {
						evidence.Items = extractStoryEvidenceItems(data, perCluster)
					}
					plan.Results = append(plan.Results, evidence)
				}
			}
			return printStoryPlan(cmd, flags, plan)
		},
	}
	cmd.Flags().IntVar(&perCluster, "per-cluster", 3, "Maximum searches/results per narrative cluster")
	cmd.Flags().BoolVar(&publicDomainOnly, "public-domain-only", false, "Restrict live Digital Collections evidence searches to public-domain results")
	return cmd
}

func buildStoryDiscoveryPlan(subject string, perCluster int) storyDiscoveryPlan {
	subject = strings.TrimSpace(subject)
	if perCluster < 1 {
		perCluster = 1
	}
	if strings.EqualFold(subject, "Anne Boleyn") || strings.EqualFold(subject, "Anna Bullen") {
		return limitStoryPlan(storyDiscoveryPlan{
			Subject: subject,
			Intent:  "Find NYPL story materials about Anne Boleyn, then branch into characters with matching court-intrigue, succession, reputation, and execution motifs.",
			Queries: []string{
				"Anne Boleyn fiction",
				"The secret diary of Anne Boleyn",
				"Anne Boleyn a king's obsession",
				"The other Boleyn girl",
				"Condemnation of Anne Boleyn",
				"King Henry VIII Anne Boleyn",
			},
			Clusters: []storyCluster{
				{
					ID:       "anne_boleyn_core",
					Label:    "Anne Boleyn stories and portraits",
					Why:      "Direct fiction, reception, portraits, and scenes around Anne Boleyn.",
					Searches: []string{"Anne Boleyn fiction", "The secret diary of Anne Boleyn", "Anne Boleyn a king's obsession", "The other Boleyn girl", "Condemnation of Anne Boleyn", "King Henry VIII Anne Boleyn"},
				},
				{
					ID:       "executed_queens",
					Label:    "Executed queens and accused consorts",
					Why:      "Women near sovereign power whose sexuality, loyalty, or legitimacy became a capital political charge.",
					Searches: []string{"Catherine Howard fiction", "Mary Queen of Scots fiction", "Lady Jane Grey fiction"},
					SimilarCharacters: []similarCharacter{
						{Name: "Catherine Howard", Why: "Another executed wife of Henry VIII, shaped by accusation, faction, sexuality, and youth.", Search: []string{"Catherine Howard fiction", "Katharine Howard"}},
						{Name: "Mary, Queen of Scots", Why: "A queen framed through sexual scandal, faction, captivity, execution, and posthumous myth.", Search: []string{"Mary Queen of Scots fiction", "Mary Queen of Scots"}},
						{Name: "Lady Jane Grey", Why: "A young woman used as a dynastic instrument, briefly elevated, then executed.", Search: []string{"Lady Jane Grey fiction", "nine days queen"}},
					},
				},
				{
					ID:       "henry_viii_wives",
					Label:    "Henry VIII wife foils",
					Why:      "The six wives form a compact story graph around power, succession, religion, reputation, and survival.",
					Searches: []string{"Catherine of Aragon fiction", "Jane Seymour Henry VIII", "Anne of Cleves fiction"},
					SimilarCharacters: []similarCharacter{
						{Name: "Catherine of Aragon", Why: "Lawful first queen displaced by dynastic pressure and religious rupture.", Search: []string{"Catherine of Aragon fiction"}},
						{Name: "Jane Seymour", Why: "The dynastic-success foil: politically quieter in myth because she produced the male heir.", Search: []string{"Jane Seymour Henry VIII"}},
						{Name: "Anne of Cleves", Why: "A survival foil: a queen whose failed marriage ended without execution.", Search: []string{"Anne of Cleves fiction"}},
					},
				},
				{
					ID:       "maligned_queens",
					Label:    "Maligned queens and hostile memory",
					Why:      "Women remembered through propaganda about seduction, ambition, foreignness, adultery, or witchcraft.",
					Searches: []string{"Marie Antoinette fiction", "Cleopatra fiction", "Guinevere fiction"},
					SimilarCharacters: []similarCharacter{
						{Name: "Marie Antoinette", Why: "A queen turned into a symbol of national anxiety through sexualized political propaganda.", Search: []string{"Marie Antoinette fiction"}},
						{Name: "Cleopatra", Why: "A powerful ruler filtered by hostile narratives of seduction, ambition, and foreignness.", Search: []string{"Cleopatra fiction"}},
						{Name: "Guinevere", Why: "A queen whose alleged sexual betrayal becomes a political and mythic catastrophe.", Search: []string{"Guinevere fiction"}},
					},
				},
			},
		}, perCluster)
	}

	return limitStoryPlan(storyDiscoveryPlan{
		Subject: subject,
		Intent:  "Find direct story materials, visual artifacts, and nearby character archetypes in NYPL-friendly search language.",
		Queries: []string{subject + " fiction", subject + " biography", subject + " portrait", subject + " drama"},
		Clusters: []storyCluster{
			{ID: "direct_stories", Label: "Direct stories", Why: "Fiction, drama, biography, and adaptations about the seed subject.", Searches: []string{subject + " fiction", subject + " drama", subject + " biography"}},
			{ID: "visual_sources", Label: "Visual source material", Why: "Portraits, engravings, and scenes that help build character/world references.", Searches: []string{subject + " portrait", subject + " illustration", subject + " scene"}},
			{ID: "nearby_archetypes", Label: "Nearby archetypes", Why: "Broader character-discovery prompts for analogous figures and motifs.", Searches: []string{subject + " similar characters", subject + " historical fiction", subject + " court intrigue"}},
		},
	}, perCluster)
}

func limitStoryPlan(plan storyDiscoveryPlan, perCluster int) storyDiscoveryPlan {
	seen := map[string]bool{}
	plan.Queries = nil
	for i := range plan.Clusters {
		if len(plan.Clusters[i].Searches) > perCluster {
			plan.Clusters[i].Searches = plan.Clusters[i].Searches[:perCluster]
		}
		for _, query := range plan.Clusters[i].Searches {
			if !seen[query] {
				seen[query] = true
				plan.Queries = append(plan.Queries, query)
			}
		}
	}
	return plan
}

func extractStoryEvidenceItems(data json.RawMessage, limit int) []storyEvidenceItem {
	var envelope struct {
		NYPLAPI struct {
			Response struct {
				Result []struct {
					Title          string `json:"title"`
					TypeOfResource string `json:"typeOfResource"`
					ItemLink       string `json:"itemLink"`
					ImageID        string `json:"imageID"`
					UUID           string `json:"uuid"`
				} `json:"result"`
			} `json:"response"`
		} `json:"nyplAPI"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil
	}
	items := make([]storyEvidenceItem, 0, len(envelope.NYPLAPI.Response.Result))
	for _, item := range envelope.NYPLAPI.Response.Result {
		items = append(items, storyEvidenceItem{Title: item.Title, TypeOfResource: item.TypeOfResource, ItemLink: item.ItemLink, ImageID: item.ImageID, UUID: item.UUID})
		if len(items) >= limit {
			break
		}
	}
	return items
}

func printStoryPlan(cmd *cobra.Command, flags *rootFlags, plan storyDiscoveryPlan) error {
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		_, err = cmd.OutOrStdout().Write(append(data, '\n'))
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n", plan.Intent)
	for _, cluster := range plan.Clusters {
		fmt.Fprintf(cmd.OutOrStdout(), "## %s\n%s\n", cluster.Label, cluster.Why)
		for _, query := range cluster.Searches {
			fmt.Fprintf(cmd.OutOrStdout(), "- search: %s\n", query)
		}
		for _, char := range cluster.SimilarCharacters {
			fmt.Fprintf(cmd.OutOrStdout(), "- similar: %s — %s\n", char.Name, char.Why)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}
	if len(plan.Results) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "## Evidence")
		for _, result := range plan.Results {
			fmt.Fprintf(cmd.OutOrStdout(), "- %s / %s\n", result.Cluster, result.Query)
			for _, item := range result.Items {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s %s\n", item.Title, item.ItemLink)
			}
		}
	}
	return nil
}
