// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// graph export: dump a unified view of your network as GEXF, DOT, or JSON so
// it can be loaded into Gephi / Cytoscape / Graphviz / any visualization tool.
//
// Nodes are people from the unified `people` table plus (optionally) company
// nodes. Edges are connection_edges rows if present, falling back to
// inferred edges: viewer -> each Happenstance friend with weight == the
// friend's connection_count, viewer -> each LinkedIn 1st-degree person.

package cli

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/store"

	"github.com/spf13/cobra"
)

// viewerNodeID is the stable node identifier for "me" in every export.
const viewerNodeID = "me"

func newGraphCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Export or inspect the unified contact graph",
	}
	cmd.AddCommand(newGraphExportCmd(flags))
	return cmd
}

func newGraphExportCmd(flags *rootFlags) *cobra.Command {
	var format, outputPath string
	var includeCompanies bool
	var limit int

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the unified contact graph as GEXF, DOT, or JSON",
		Long: `Emit the unified network graph in a tool-friendly format.

Formats:
  - gexf (default): for Gephi, Cytoscape, sigma.js
  - dot:            for Graphviz
  - json:           edge-list JSON for custom pipelines`,
		Example: `  contact-goat-pp-cli graph export --format gexf --output graph.gexf
  contact-goat-pp-cli graph export --format dot | dot -Tsvg > graph.svg
  contact-goat-pp-cli graph export --format json --include-companies`,
		RunE: func(cmd *cobra.Command, args []string) error {
			format = strings.ToLower(strings.TrimSpace(format))
			switch format {
			case "gexf", "dot", "json":
				// valid
			default:
				return usageErr(fmt.Errorf("invalid --format %q: must be gexf, dot, or json", format))
			}

			s, err := openP2Store()
			if err != nil {
				return fmt.Errorf("opening local store: %w\nRun `contact-goat-pp-cli sync` first", err)
			}
			if s == nil {
				return fmt.Errorf("no local store found. Run `contact-goat-pp-cli sync` first")
			}
			defer s.Close()

			g, err := buildGraph(s, includeCompanies, limit)
			if err != nil {
				return err
			}
			if len(g.Nodes) == 0 {
				return fmt.Errorf("no graph data available. Run `contact-goat-pp-cli sync` to populate the people table first")
			}

			var w io.Writer = cmd.OutOrStdout()
			if outputPath != "" && outputPath != "-" {
				f, err := os.Create(outputPath)
				if err != nil {
					return fmt.Errorf("creating output file: %w", err)
				}
				defer f.Close()
				w = f
			}

			switch format {
			case "gexf":
				return writeGEXF(w, g)
			case "dot":
				return writeDOT(w, g)
			case "json":
				return writeGraphJSON(w, g, flags)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "gexf", "Output format: gexf, dot, or json")
	cmd.Flags().StringVar(&outputPath, "output", "-", "Output file (- for stdout)")
	cmd.Flags().BoolVar(&includeCompanies, "include-companies", false, "Emit company nodes and person->company edges")
	cmd.Flags().IntVar(&limit, "limit", 5000, "Max people nodes to include")
	return cmd
}

// ---- graph model ----

type graphNode struct {
	ID    string
	Label string
	Kind  string // "viewer" | "person" | "company"
	Attrs map[string]string
}

type graphEdge struct {
	Source  string
	Target  string
	Weight  float64
	Source2 string // "li_1deg", "hp_friend", "person_company" etc — semantic source label
}

type graphModel struct {
	Nodes []graphNode
	Edges []graphEdge
}

func buildGraph(s *store.Store, includeCompanies bool, limit int) (*graphModel, error) {
	g := &graphModel{}
	// Always include the viewer as a root node.
	g.Nodes = append(g.Nodes, graphNode{ID: viewerNodeID, Label: "me", Kind: "viewer"})

	people, err := s.ListPeople(limit)
	if err != nil {
		return nil, fmt.Errorf("listing people: %w", err)
	}

	// Map person_id -> node id for edge resolution.
	personNode := make(map[int64]string, len(people))
	companySeen := make(map[string]bool)
	for _, p := range people {
		id := fmt.Sprintf("p%d", p.ID)
		personNode[p.ID] = id
		attrs := map[string]string{}
		if p.LinkedInURL != "" {
			attrs["linkedin_url"] = p.LinkedInURL
		}
		if p.HappenstanceUUID != "" {
			attrs["happenstance_uuid"] = p.HappenstanceUUID
		}
		if p.Title != "" {
			attrs["title"] = p.Title
		}
		if p.Location != "" {
			attrs["location"] = p.Location
		}
		if len(p.Sources) > 0 {
			attrs["sources"] = strings.Join(p.Sources, ",")
		}
		if p.Company != "" {
			attrs["company"] = p.Company
		}
		label := p.FullName
		if label == "" {
			label = id
		}
		g.Nodes = append(g.Nodes, graphNode{ID: id, Label: label, Kind: "person", Attrs: attrs})

		if includeCompanies && p.Company != "" {
			cid := companyID(p.Company)
			if !companySeen[cid] {
				companySeen[cid] = true
				g.Nodes = append(g.Nodes, graphNode{ID: cid, Label: p.Company, Kind: "company"})
			}
			g.Edges = append(g.Edges, graphEdge{Source: id, Target: cid, Weight: 1, Source2: "person_company"})
		}
	}

	edges, err := s.ListEdges(0)
	if err != nil {
		return nil, fmt.Errorf("listing edges: %w", err)
	}
	if len(edges) > 0 {
		for _, e := range edges {
			dst, ok := personNode[e.PersonID]
			if !ok {
				continue
			}
			weight := e.Strength
			if weight <= 0 {
				weight = 1
			}
			g.Edges = append(g.Edges, graphEdge{
				Source: viewerNodeID, Target: dst, Weight: weight, Source2: e.Source,
			})
		}
	} else {
		// Fallback: infer viewer -> each person edge with weight 1. This is
		// weak but better than an empty graph, and is labeled so downstream
		// tools can filter.
		for _, pid := range personNode {
			g.Edges = append(g.Edges, graphEdge{
				Source: viewerNodeID, Target: pid, Weight: 1, Source2: "inferred",
			})
		}
	}

	return g, nil
}

func companyID(name string) string {
	return "c_" + strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", "_"))
}

// ---- GEXF ----

type gexfAttr struct {
	XMLName xml.Name `xml:"attribute"`
	ID      string   `xml:"id,attr"`
	Title   string   `xml:"title,attr"`
	Type    string   `xml:"type,attr"`
}

type gexfAttValue struct {
	XMLName xml.Name `xml:"attvalue"`
	For     string   `xml:"for,attr"`
	Value   string   `xml:"value,attr"`
}

func writeGEXF(w io.Writer, g *graphModel) error {
	// Minimal GEXF 1.2 emitter. We collect attribute keys across all nodes
	// to emit a stable schema.
	attrKeys := map[string]bool{}
	for _, n := range g.Nodes {
		for k := range n.Attrs {
			attrKeys[k] = true
		}
	}
	keyOrder := make([]string, 0, len(attrKeys))
	for k := range attrKeys {
		keyOrder = append(keyOrder, k)
	}
	// deterministic ordering
	sortStrings(keyOrder)

	fmt.Fprintln(w, `<?xml version="1.0" encoding="UTF-8"?>`)
	fmt.Fprintln(w, `<gexf xmlns="http://www.gexf.net/1.2draft" version="1.2">`)
	fmt.Fprintf(w, "  <meta lastmodifieddate=%q><creator>contact-goat-pp-cli</creator></meta>\n", time.Now().UTC().Format("2006-01-02"))
	fmt.Fprintln(w, `  <graph mode="static" defaultedgetype="directed">`)
	fmt.Fprintln(w, `    <attributes class="node">`)
	// Always include a "kind" attribute plus any we discovered.
	fmt.Fprintln(w, `      <attribute id="kind" title="kind" type="string"/>`)
	for i, k := range keyOrder {
		fmt.Fprintf(w, `      <attribute id="a%d" title=%q type="string"/>`+"\n", i+1, k)
	}
	fmt.Fprintln(w, `    </attributes>`)
	fmt.Fprintln(w, `    <nodes>`)
	for _, n := range g.Nodes {
		fmt.Fprintf(w, "      <node id=%q label=%q>\n", xmlEscape(n.ID), xmlEscape(n.Label))
		fmt.Fprintln(w, `        <attvalues>`)
		fmt.Fprintf(w, `          <attvalue for="kind" value=%q/>`+"\n", xmlEscape(n.Kind))
		for i, k := range keyOrder {
			if v, ok := n.Attrs[k]; ok && v != "" {
				fmt.Fprintf(w, `          <attvalue for="a%d" value=%q/>`+"\n", i+1, xmlEscape(v))
			}
		}
		fmt.Fprintln(w, `        </attvalues>`)
		fmt.Fprintln(w, `      </node>`)
	}
	fmt.Fprintln(w, `    </nodes>`)
	fmt.Fprintln(w, `    <edges>`)
	for i, e := range g.Edges {
		fmt.Fprintf(w, `      <edge id="e%d" source=%q target=%q weight="%g" label=%q/>`+"\n",
			i, xmlEscape(e.Source), xmlEscape(e.Target), e.Weight, xmlEscape(e.Source2))
	}
	fmt.Fprintln(w, `    </edges>`)
	fmt.Fprintln(w, `  </graph>`)
	fmt.Fprintln(w, `</gexf>`)
	return nil
}

// xmlEscape performs just enough escaping for attribute values (avoids bringing
// in encoding/xml's full escape path and keeps output predictable).
func xmlEscape(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&apos;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ---- DOT ----

func writeDOT(w io.Writer, g *graphModel) error {
	fmt.Fprintln(w, "digraph network {")
	fmt.Fprintln(w, `  rankdir=LR;`)
	fmt.Fprintln(w, `  node [shape=box, fontsize=10];`)
	for _, n := range g.Nodes {
		shape := "box"
		switch n.Kind {
		case "viewer":
			shape = "doublecircle"
		case "company":
			shape = "oval"
		}
		fmt.Fprintf(w, "  %q [label=%q, shape=%s];\n", n.ID, n.Label, shape)
	}
	for _, e := range g.Edges {
		fmt.Fprintf(w, "  %q -> %q [weight=%.2f, label=%q];\n",
			e.Source, e.Target, e.Weight, e.Source2)
	}
	fmt.Fprintln(w, "}")
	return nil
}

// ---- JSON ----

func writeGraphJSON(w io.Writer, g *graphModel, flags *rootFlags) error {
	type outNode struct {
		ID    string            `json:"id"`
		Label string            `json:"label"`
		Kind  string            `json:"kind"`
		Attrs map[string]string `json:"attrs,omitempty"`
	}
	type outEdge struct {
		Source string  `json:"source"`
		Target string  `json:"target"`
		Weight float64 `json:"weight"`
		Kind   string  `json:"kind"`
	}
	payload := struct {
		Nodes []outNode `json:"nodes"`
		Edges []outEdge `json:"edges"`
	}{}
	for _, n := range g.Nodes {
		payload.Nodes = append(payload.Nodes, outNode{ID: n.ID, Label: n.Label, Kind: n.Kind, Attrs: n.Attrs})
	}
	for _, e := range g.Edges {
		payload.Edges = append(payload.Edges, outEdge{Source: e.Source, Target: e.Target, Weight: e.Weight, Kind: e.Source2})
	}
	enc := json.NewEncoder(w)
	if flags == nil || !flags.compact {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(payload)
}

func sortStrings(in []string) {
	// tiny insertion sort to avoid pulling in sort for one call path
	for i := 1; i < len(in); i++ {
		for j := i; j > 0 && in[j-1] > in[j]; j-- {
			in[j-1], in[j] = in[j], in[j-1]
		}
	}
}
