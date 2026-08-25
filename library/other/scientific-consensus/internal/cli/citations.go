// Hand-authored novel command: citation graph. Not generated.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/internal/source"
	"github.com/spf13/cobra"
)

// graphNode is one work in the citation network.
type graphNode struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Year    int    `json:"year,omitempty"`
	CitedBy int    `json:"cited_by_count"`
}

// graphEdge is a directed citation: From cites To.
type graphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// citationGraph is the full network, shaped for both the human summary and a
// future web visualization (flat nodes + edges arrays).
type citationGraph struct {
	Seed      string      `json:"seed"`
	SeedTitle string      `json:"seed_title,omitempty"`
	Depth     int         `json:"depth"`
	Direction string      `json:"direction"`
	NodeCount int         `json:"node_count"`
	EdgeCount int         `json:"edge_count"`
	Nodes     []graphNode `json:"nodes"`
	Edges     []graphEdge `json:"edges"`
	Note      string      `json:"note,omitempty"`
}

func newCitationsCmd(flags *rootFlags) *cobra.Command {
	var doi, pmid, id string
	var depth, maxNodes int
	var direction string

	cmd := &cobra.Command{
		Use:   "citations",
		Short: "Build a citation network around a seed work (its references and the works citing it)",
		Long: "Given a seed work (by --doi, --pmid, or --id), expand a citation network using\n" +
			"OpenAlex: the works it references and/or the works that cite it. Bounded by\n" +
			"--depth (hops) and --max-nodes (a hard cap that also limits API calls). The\n" +
			"--json output is flat nodes + edges arrays, ready to feed a network\n" +
			"visualization; the default output is a compact summary. Keyless; reuses the\n" +
			"existing OpenAlex client, cache, and rate limit.",
		Example: "  scientific-consensus-pp-cli citations --doi 10.1136/bmj.i6583\n" +
			"  scientific-consensus-pp-cli citations --id W2741809807 --depth 2 --max-nodes 80 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would build a citation graph")
				return nil
			}

			seeds := 0
			for _, v := range []string{doi, pmid, id} {
				if v != "" {
					seeds++
				}
			}
			if seeds != 1 {
				return usageErr(errors.New("provide exactly one of --doi, --pmid, or --id"))
			}
			switch direction {
			case "both", "cited-by", "references":
			default:
				return usageErr(fmt.Errorf("invalid --direction %q: must be both, cited-by, or references", direction))
			}
			// Bound depth and node count defensively (also caps API calls).
			if depth < 1 {
				depth = 1
			}
			if depth > 2 {
				depth = 2
			}
			if maxNodes < 1 {
				maxNodes = 1
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var entity string
			switch {
			case doi != "":
				entity = "doi:" + source.NormDOI(doi)
			case pmid != "":
				entity = "pmid:" + normPMID(pmid)
			default:
				entity = shortID(id)
			}

			graph, err := buildCitationGraph(ctx, c, flags, entity, depth, maxNodes, direction)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return emit(cmd, flags, graph, func(w io.Writer) { renderCitations(w, graph) })
		},
	}
	cmd.Flags().StringVar(&doi, "doi", "", "seed work DOI")
	cmd.Flags().StringVar(&pmid, "pmid", "", "seed work PMID")
	cmd.Flags().StringVar(&id, "id", "", "seed work OpenAlex ID (W...)")
	cmd.Flags().IntVar(&depth, "depth", 1, "hops to expand out from the seed (max 2)")
	cmd.Flags().IntVar(&maxNodes, "max-nodes", 50, "hard cap on total nodes (also bounds API calls)")
	cmd.Flags().StringVar(&direction, "direction", "both", "which edges to follow: both | cited-by | references")
	return cmd
}

// buildCitationGraph performs a bounded breadth-first expansion from the seed.
// Each frontier node is expanded by querying OpenAlex for the works citing it
// (cites: filter -> "cited-by" direction) and/or the works it references
// (cited_by: filter -> "references" direction). Expansion stops cleanly once
// maxNodes is reached; a node that fails to fetch is skipped, not fatal.
func buildCitationGraph(ctx context.Context, c apiGetter, flags *rootFlags, entity string, depth, maxNodes int, direction string) (citationGraph, error) {
	g := citationGraph{Depth: depth, Direction: direction}

	seed, err := fetchGraphSeed(ctx, c, entity)
	if err != nil {
		return g, err
	}
	g.Seed = seed.ID
	g.SeedTitle = seed.Title

	nodes := map[string]graphNode{seed.ID: seed}
	edgeSet := map[string]bool{}
	var edges []graphEdge
	addEdge := func(from, to string) {
		key := from + "|" + to
		if !edgeSet[key] {
			edgeSet[key] = true
			edges = append(edges, graphEdge{From: from, To: to})
		}
	}

	wantCitedBy := direction == "both" || direction == "cited-by"
	wantRefs := direction == "both" || direction == "references"
	perPage := maxNodes
	if perPage > 50 {
		perPage = 50 // bound per-call payload regardless of cap
	}

	prog := newProgress(flags, "expanding graph", maxNodes)
	frontier := []string{seed.ID}

	// addNeighbor records an edge and, if room remains, the neighbor node.
	// edgeFrom/edgeTo encode the citation direction for this neighbor.
	addNeighbor := func(node string, w scWork, citing bool, next *[]string) {
		wid := shortID(w.ID)
		if wid == "" || wid == node {
			return
		}
		if _, ok := nodes[wid]; !ok {
			if len(nodes) >= maxNodes {
				return // at cap: skip the new node and its (dangling) edge
			}
			nodes[wid] = graphNode{ID: wid, Title: w.Title, Year: w.Year, CitedBy: w.CitedBy}
			*next = append(*next, wid)
		}
		if citing {
			addEdge(wid, node) // neighbor cites node
		} else {
			addEdge(node, wid) // node cites neighbor
		}
	}

hops:
	for hop := 0; hop < depth; hop++ {
		var next []string
		for _, node := range frontier {
			if len(nodes) >= maxNodes {
				break hops
			}
			if wantCitedBy {
				if works, ferr := fetchNeighbors(ctx, c, node, "cites", perPage); ferr == nil {
					for _, w := range works {
						addNeighbor(node, w, true, &next)
					}
				}
			}
			// Skip the references call if cited-by already filled the cap — its
			// neighbors would all be dropped anyway, so the API call is wasted.
			if wantRefs && len(nodes) < maxNodes {
				if works, ferr := fetchNeighbors(ctx, c, node, "cited_by", perPage); ferr == nil {
					for _, w := range works {
						addNeighbor(node, w, false, &next)
					}
				}
			}
			prog.update(len(nodes))
		}
		frontier = next
		if len(frontier) == 0 {
			break
		}
	}
	prog.done()

	g.Nodes = make([]graphNode, 0, len(nodes))
	for _, n := range nodes {
		g.Nodes = append(g.Nodes, n)
	}
	// Most-cited first so the human summary and any top-N view are stable.
	sort.SliceStable(g.Nodes, func(i, j int) bool { return g.Nodes[i].CitedBy > g.Nodes[j].CitedBy })
	g.Edges = edges
	g.NodeCount = len(g.Nodes)
	g.EdgeCount = len(g.Edges)
	if g.NodeCount <= 1 {
		g.Note = "no neighbors found; the seed may have no indexed references or citations in OpenAlex, or --direction excluded them"
	}
	return g, nil
}

// fetchGraphSeed resolves and loads the seed work's node fields. entity is a
// namespaced OpenAlex identifier ("doi:...", "pmid:...", or a bare "W...").
func fetchGraphSeed(ctx context.Context, c apiGetter, entity string) (graphNode, error) {
	raw, err := c.Get(ctx, "/works/"+entity, map[string]string{
		"select": "id,title,display_name,publication_year,cited_by_count",
		"mailto": defaultMailto,
	})
	if err != nil {
		return graphNode{}, err
	}
	var w struct {
		ID              string `json:"id"`
		Title           string `json:"title"`
		DisplayName     string `json:"display_name"`
		PublicationYear int    `json:"publication_year"`
		CitedByCount    int    `json:"cited_by_count"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return graphNode{}, fmt.Errorf("parsing seed work: %w", err)
	}
	id := shortID(w.ID)
	if id == "" {
		return graphNode{}, fmt.Errorf("could not resolve seed work %q", entity)
	}
	title := w.Title
	if title == "" {
		title = w.DisplayName
	}
	return graphNode{ID: id, Title: title, Year: w.PublicationYear, CitedBy: w.CitedByCount}, nil
}

// fetchNeighbors returns works related to wid via the given OpenAlex filter key
// ("cites" for works citing wid, "cited_by" for works wid references).
func fetchNeighbors(ctx context.Context, c apiGetter, wid, filterKey string, perPage int) ([]scWork, error) {
	works, _, err := fetchWorks(ctx, c, "", filterKey+":"+wid, "cited_by_count:desc", perPage)
	return works, err
}

// shortID strips any OpenAlex URL prefix to the bare "W..." id.
func shortID(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	return s
}

func renderCitations(w io.Writer, g citationGraph) {
	fmt.Fprintf(w, "Citation graph for: %s\n", g.Seed)
	if g.SeedTitle != "" {
		fmt.Fprintf(w, "  Seed:      %s\n", truncate(g.SeedTitle, 80))
	}
	fmt.Fprintf(w, "  Direction: %s   Depth: %d\n", g.Direction, g.Depth)
	fmt.Fprintf(w, "  Nodes:     %d   Edges: %d\n", g.NodeCount, g.EdgeCount)
	top := g.Nodes
	if len(top) > 5 {
		top = top[:5]
	}
	if len(top) > 0 {
		fmt.Fprintln(w, "\n  Top cited nodes:")
		for _, n := range top {
			fmt.Fprintf(w, "    • [%d, cites=%d] %s\n", n.Year, n.CitedBy, truncate(n.Title, 70))
		}
	}
	if g.Note != "" {
		fmt.Fprintf(w, "\n  Note: %s\n", g.Note)
	}
}
