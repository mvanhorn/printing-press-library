package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/tableau/internal/restclient"
	"github.com/spf13/cobra"
)

const version = "0.1.0"

// Global flags (env-backed defaults applied in PersistentPreRun / flag setup).
type rootFlags struct {
	server     string
	site       string
	patName    string
	patSecret  string
	apiVersion string
	json       bool
}

func main() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitCodeFor(err))
	}
}

type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

const (
	exitOK     = 0
	exitUsage  = 3
	exitIO     = 4
	exitAuth   = 5
	exitRemote = 6
)

func exitCodeFor(err error) int {
	if err == nil {
		return exitOK
	}
	if ee, ok := err.(*exitError); ok {
		return ee.code
	}
	return exitUsage
}

func fail(code int, format string, args ...any) error {
	return &exitError{code: code, msg: fmt.Sprintf(format, args...)}
}

func newRootCmd() *cobra.Command {
	rf := &rootFlags{}

	root := &cobra.Command{
		Use:   "tableau-rest-pp-cli",
		Short: "Agent bridge for Tableau Server/Cloud REST: PAT auth, projects, workbooks, publish",
		Long: `Tableau REST CLI for agents (Track B).

Authenticate with a personal access token (PAT), list projects and workbooks,
download a workbook for local Track A edits, then publish back.

Secrets (PAT secret) are never written to stdout/stderr logs.
Configure via flags or environment variables (see README).`,
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetVersionTemplate("{{.Version}}\n")

	// Defaults from environment — never print secrets.
	root.PersistentFlags().StringVar(&rf.server, "server", envOr("TABLEAU_SERVER", ""), "Tableau Server/Cloud origin (e.g. https://10ay.online.tableau.com)")
	root.PersistentFlags().StringVar(&rf.site, "site", envOr("TABLEAU_SITE_CONTENT_URL", ""), "Site contentUrl (empty = default site)")
	root.PersistentFlags().StringVar(&rf.patName, "pat-name", envOr("TABLEAU_PAT_NAME", ""), "Personal access token name")
	root.PersistentFlags().StringVar(&rf.patSecret, "pat-secret", envOr("TABLEAU_PAT_SECRET", ""), "Personal access token secret (never logged)")
	root.PersistentFlags().StringVar(&rf.apiVersion, "api-version", client.DefaultAPIVersion, "Tableau REST API version")
	root.PersistentFlags().BoolVar(&rf.json, "json", false, "Emit machine-readable JSON on stdout")

	root.AddCommand(newAuthCmd(rf))
	root.AddCommand(newProjectsCmd(rf))
	root.AddCommand(newWorkbooksCmd(rf))
	root.AddCommand(newSitesCmd(rf))
	return root
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func newClient(rf *rootFlags) (*client.Client, error) {
	if rf.server == "" {
		return nil, fail(exitUsage, "server is required (--server or TABLEAU_SERVER)")
	}
	c, err := client.New(client.Config{
		Server:         rf.server,
		SiteContentURL: rf.site,
		PatName:        rf.patName,
		PatSecret:      rf.patSecret,
		APIVersion:     rf.apiVersion,
	})
	if err != nil {
		return nil, fail(exitUsage, "%v", err)
	}
	return c, nil
}

func requireAuth(rf *rootFlags) (*client.Client, error) {
	c, err := newClient(rf)
	if err != nil {
		return nil, err
	}
	if rf.patName == "" || rf.patSecret == "" {
		return nil, fail(exitUsage, "PAT credentials required (--pat-name/--pat-secret or TABLEAU_PAT_NAME/TABLEAU_PAT_SECRET)")
	}
	if _, err := c.SignIn(); err != nil {
		return nil, fail(exitAuth, "%v", err)
	}
	return c, nil
}

// --- auth ---

func newAuthCmd(rf *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication (PAT sign-in)",
	}
	cmd.AddCommand(newAuthLoginCmd(rf))
	cmd.AddCommand(newAuthWhoamiCmd(rf))
	return cmd
}

func newAuthLoginCmd(rf *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Sign in with PAT and print session summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := requireAuth(rf)
			if err != nil {
				return err
			}
			defer c.SignOut()
			cred := c.Credentials()
			out := map[string]string{
				"siteId":     cred.SiteID,
				"contentUrl": cred.ContentURL,
				"userId":     cred.UserID,
				"token":      cred.Token,
				"server":     c.Config().Server,
				"apiVersion": c.Config().APIVersion,
			}
			if rf.json {
				return printJSON(out)
			}
			// Never print PAT secret. Session token is shown for agent chaining.
			fmt.Fprintf(os.Stderr, "signed in to %s (api %s)\n", c.Config().Server, c.Config().APIVersion)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "siteId\t%s\n", cred.SiteID)
			fmt.Fprintf(w, "contentUrl\t%s\n", emptyAsDefault(cred.ContentURL))
			fmt.Fprintf(w, "userId\t%s\n", cred.UserID)
			fmt.Fprintf(w, "token\t%s\n", cred.Token)
			return w.Flush()
		},
	}
}

func newAuthWhoamiCmd(rf *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Sign in and show current site/user (no secrets beyond session token)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := requireAuth(rf)
			if err != nil {
				return err
			}
			defer c.SignOut()
			cred := c.Credentials()
			out := map[string]string{
				"siteId":     cred.SiteID,
				"contentUrl": cred.ContentURL,
				"userId":     cred.UserID,
				"server":     c.Config().Server,
			}
			if rf.json {
				return printJSON(out)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "server\t%s\n", c.Config().Server)
			fmt.Fprintf(w, "siteId\t%s\n", cred.SiteID)
			fmt.Fprintf(w, "contentUrl\t%s\n", emptyAsDefault(cred.ContentURL))
			fmt.Fprintf(w, "userId\t%s\n", cred.UserID)
			return w.Flush()
		},
	}
}

// --- projects ---

func newProjectsCmd(rf *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "Project operations",
	}
	cmd.AddCommand(newProjectsListCmd(rf))
	return cmd
}

func newProjectsListCmd(rf *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List projects on the signed-in site",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := requireAuth(rf)
			if err != nil {
				return err
			}
			defer c.SignOut()
			projects, err := c.ListProjects()
			if err != nil {
				return fail(exitRemote, "%v", err)
			}
			if rf.json {
				return printJSON(projects)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tPARENT")
			for _, p := range projects {
				fmt.Fprintf(w, "%s\t%s\t%s\n", p.ID, p.Name, p.ParentID)
			}
			return w.Flush()
		},
	}
}

// --- workbooks ---

func newWorkbooksCmd(rf *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workbooks",
		Short: "Workbook list / download / publish",
	}
	cmd.AddCommand(newWorkbooksListCmd(rf))
	cmd.AddCommand(newWorkbooksDownloadCmd(rf))
	cmd.AddCommand(newWorkbooksPublishCmd(rf))
	return cmd
}

func newWorkbooksListCmd(rf *rootFlags) *cobra.Command {
	var projectID string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workbooks (optionally filtered by project)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := requireAuth(rf)
			if err != nil {
				return err
			}
			defer c.SignOut()
			workbooks, err := c.ListWorkbooks(projectID)
			if err != nil {
				return fail(exitRemote, "%v", err)
			}
			if rf.json {
				return printJSON(workbooks)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tPROJECT\tUPDATED")
			for _, wb := range workbooks {
				proj := wb.ProjectName
				if proj == "" {
					proj = wb.ProjectID
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", wb.ID, wb.Name, proj, wb.UpdatedAt)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&projectID, "project-id", "", "Filter by project LUID")
	return cmd
}

func newWorkbooksDownloadCmd(rf *rootFlags) *cobra.Command {
	var id, output string
	cmd := &cobra.Command{
		Use:   "download",
		Short: "Download a workbook to a local .twb / .twbx path",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == "" {
				return fail(exitUsage, "--id is required")
			}
			if output == "" {
				return fail(exitUsage, "--output is required")
			}
			c, err := requireAuth(rf)
			if err != nil {
				return err
			}
			defer c.SignOut()
			if err := c.DownloadWorkbook(id, output); err != nil {
				return fail(exitIO, "%v", err)
			}
			if rf.json {
				return printJSON(map[string]string{"id": id, "output": output})
			}
			fmt.Fprintf(os.Stderr, "downloaded workbook %s -> %s\n", id, output)
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Workbook LUID")
	cmd.Flags().StringVar(&output, "output", "", "Output path (e.g. path.twbx)")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("output")
	return cmd
}

func newWorkbooksPublishCmd(rf *rootFlags) *cobra.Command {
	var file, projectID, name string
	var dryRun, overwrite bool
	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish a local .twb / .twbx to a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fail(exitUsage, "--file is required")
			}
			if projectID == "" {
				return fail(exitUsage, "--project-id is required")
			}
			if name == "" {
				return fail(exitUsage, "--name is required")
			}
			info, err := os.Stat(file)
			if err != nil {
				return fail(exitIO, "publish file: %v", err)
			}
			if info.IsDir() {
				return fail(exitUsage, "publish file is a directory: %s", file)
			}
			ext := strings.ToLower(filepath.Ext(file))
			if ext != ".twb" && ext != ".twbx" {
				return fail(exitUsage, "unsupported workbook extension %q (want .twb or .twbx)", ext)
			}

			if dryRun {
				plan := map[string]any{
					"dryRun":    true,
					"file":      file,
					"sizeBytes": info.Size(),
					"projectId": projectID,
					"name":      name,
					"overwrite": overwrite,
					"server":    rf.server,
					"site":      rf.site,
				}
				if rf.json {
					return printJSON(plan)
				}
				fmt.Fprintf(os.Stderr, "dry-run: would publish %s (%d bytes) as %q to project %s on %s site %q\n",
					file, info.Size(), name, projectID, rf.server, emptyAsDefault(rf.site))
				return nil
			}

			c, err := requireAuth(rf)
			if err != nil {
				return err
			}
			defer c.SignOut()
			res, err := c.PublishWorkbook(file, projectID, name, overwrite)
			if err != nil {
				return fail(exitRemote, "%v", err)
			}
			if rf.json {
				return printJSON(res)
			}
			fmt.Fprintf(os.Stderr, "published workbook %s (%s)\n", res.Name, res.ID)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "id\t%s\n", res.ID)
			fmt.Fprintf(w, "name\t%s\n", res.Name)
			fmt.Fprintf(w, "contentUrl\t%s\n", res.ContentURL)
			fmt.Fprintf(w, "projectId\t%s\n", res.ProjectID)
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "Local .twb or .twbx path")
	cmd.Flags().StringVar(&projectID, "project-id", "", "Destination project LUID")
	cmd.Flags().StringVar(&name, "name", "", "Published workbook name")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate inputs and print plan; do not upload")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite existing workbook with the same name")
	_ = cmd.MarkFlagRequired("file")
	_ = cmd.MarkFlagRequired("project-id")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

// --- sites ---

func newSitesCmd(rf *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sites",
		Short: "Site operations",
	}
	cmd.AddCommand(newSitesListCmd(rf))
	return cmd
}

func newSitesListCmd(rf *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List sites (server admin) or current site",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := requireAuth(rf)
			if err != nil {
				return err
			}
			defer c.SignOut()
			sites, err := c.ListSites()
			if err != nil {
				return fail(exitRemote, "%v", err)
			}
			if rf.json {
				return printJSON(sites)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tCONTENT_URL\tSTATE")
			for _, s := range sites {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.ID, s.Name, emptyAsDefault(s.ContentURL), s.State)
			}
			return w.Flush()
		},
	}
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func emptyAsDefault(s string) string {
	if s == "" {
		return "(default)"
	}
	return s
}
