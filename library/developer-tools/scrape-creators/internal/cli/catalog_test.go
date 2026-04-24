package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestResolveResourceSpecAlias(t *testing.T) {
	spec, ok := resolveResourceSpec("tiktok_videos")
	if !ok {
		t.Fatal("expected tiktok_videos alias to resolve")
	}
	if spec.Name != "tiktok" {
		t.Fatalf("resolveResourceSpec returned %q, want tiktok", spec.Name)
	}
	if spec.Path != "/v1/tiktok/videos/popular" {
		t.Fatalf("resolveResourceSpec path = %q", spec.Path)
	}
}

func TestApplyPlatformRootMetadataHidesShortcutFlags(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	tiktok := &cobra.Command{Use: "tiktok", Short: "Product Reviews", Long: "Shortcut for 'tiktok list-shop'. Product Reviews"}
	tiktok.Flags().String("url", "", "legacy shortcut flag")
	tiktok.AddCommand(&cobra.Command{Use: "profile"})
	root.AddCommand(tiktok)

	applyPlatformRootMetadata(root)

	if got := tiktok.Short; got != platformRootSummaries["tiktok"] {
		t.Fatalf("Short = %q, want %q", got, platformRootSummaries["tiktok"])
	}
	flag := tiktok.Flags().Lookup("url")
	if flag == nil || !flag.Hidden {
		t.Fatalf("expected tiktok shortcut flag to be hidden after metadata application")
	}
}

func TestAPIInterfacesExcludesUtilityCommands(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.AddCommand(&cobra.Command{Use: "tiktok", Short: "old"})
	root.Commands()[0].AddCommand(&cobra.Command{Use: "profile"})
	root.AddCommand(&cobra.Command{Use: "archive"})
	root.Commands()[1].AddCommand(&cobra.Command{Use: "status"})
	root.AddCommand(&cobra.Command{Use: "search"})
	root.Commands()[2].AddCommand(&cobra.Command{Use: "trends"})
	root.AddCommand(&cobra.Command{Use: "completion"})
	root.Commands()[3].AddCommand(&cobra.Command{Use: "zsh"})

	interfaces := apiInterfaces(root)
	if len(interfaces) != 1 || interfaces[0].Name() != "tiktok" {
		t.Fatalf("apiInterfaces returned %v, want only tiktok", interfaces)
	}
}

func TestNewArchiveCmdUsesArchiveNameAndWorkflowAlias(t *testing.T) {
	cmd := newArchiveCmd(&rootFlags{})
	if cmd.Name() != "archive" {
		t.Fatalf("Name() = %q, want archive", cmd.Name())
	}
	aliases := cmd.Aliases
	if len(aliases) != 1 || aliases[0] != "workflow" {
		t.Fatalf("Aliases = %v, want [workflow]", aliases)
	}
	if sub, _, err := cmd.Find([]string{"status"}); err != nil || sub == nil || sub.Name() != "status" {
		t.Fatalf("Find(status) = %v, %v, want status subcommand", sub, err)
	}
}

func TestIsDryRunPayload(t *testing.T) {
	if !isDryRunPayload([]byte(`{"dry_run":true}`)) {
		t.Fatal("expected dry-run payload to be detected")
	}
	if isDryRunPayload([]byte(`{"results":[]}`)) {
		t.Fatal("did not expect non-dry-run payload to be detected")
	}
}

func TestResourceAPIPathPrefersKnownResourceMapping(t *testing.T) {
	if got := resourceAPIPath("tiktok"); got != "/v1/tiktok/videos/popular" {
		t.Fatalf("resourceAPIPath(tiktok) = %q", got)
	}
	if got := resourceAPIPath("tiktok_videos"); got != "/v1/tiktok/videos/popular" {
		t.Fatalf("resourceAPIPath(alias) = %q", got)
	}
	if got := resourceAPIPath("custom"); got != "/custom" {
		t.Fatalf("resourceAPIPath(custom) = %q", got)
	}
}

func TestArchiveableResourceNames(t *testing.T) {
	got := archiveableResourceNames()
	if len(got) != 1 || got[0] != "account" {
		t.Fatalf("archiveableResourceNames() = %v, want [account]", got)
	}
}
