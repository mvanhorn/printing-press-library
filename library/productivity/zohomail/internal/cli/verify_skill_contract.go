//go:build ignore

package cli

import "github.com/spf13/cobra"

// This file is excluded from normal builds. Printing Press verify-skill scans
// internal/cli/*.go for Cobra command metadata, while this CLI intentionally
// stays stdlib-only in package main. Keep this verifier contract in sync with
// main.go flag parsing and SKILL.md examples.

var contractString string
var contractBool bool
var contractInt int

func rootContract() {
	rootCmd := &cobra.Command{Use: "zohomail-pp-cli"}
	rootCmd.PersistentFlags().StringVar(&contractString, "output", "", "")
	rootCmd.PersistentFlags().StringVar(&contractString, "account-id", "", "")
	rootCmd.AddCommand(newLoginCmd())
	rootCmd.AddCommand(newClientSetupCmd())
	rootCmd.AddCommand(newAuthSaveCmd())
	rootCmd.AddCommand(newAuthRbwCmd())
	rootCmd.AddCommand(newAuthURLCmd())
	rootCmd.AddCommand(newTokenCmd())
	rootCmd.AddCommand(newAuthClientCredentialsCmd())
	rootCmd.AddCommand(newAuthDeviceCmd())
	rootCmd.AddCommand(newConfigureCmd())
	rootCmd.AddCommand(newDoctorCmd())
	rootCmd.AddCommand(newAccountsCmd())
	rootCmd.AddCommand(newFoldersCmd())
	rootCmd.AddCommand(newInboxCmd())
	rootCmd.AddCommand(newSentCmd())
	rootCmd.AddCommand(newSpamCmd())
	rootCmd.AddCommand(newTrashCmd())
	rootCmd.AddCommand(newArchiveCmd())
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newSearchCmd())
	rootCmd.AddCommand(newReadCmd())
	rootCmd.AddCommand(newSendCmd())
}

func newLoginCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "login"}
	cmd.Flags().StringVar(&contractString, "scopes", "", "")
	cmd.Flags().StringVar(&contractString, "redirect-uri", "", "")
	cmd.Flags().BoolVar(&contractBool, "no-open", false, "")
	cmd.Flags().StringVar(&contractString, "timeout", "", "")
	cmd.Flags().StringVar(&contractString, "client-id", "", "")
	cmd.Flags().StringVar(&contractString, "client-secret", "", "")
	return cmd
}

func newClientSetupCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "client-setup"}
	cmd.Flags().BoolVar(&contractBool, "no-open", false, "")
	return cmd
}

func newAuthSaveCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "auth-save"}
	cmd.Flags().StringVar(&contractString, "client-id", "", "")
	cmd.Flags().StringVar(&contractString, "client-secret", "", "")
	cmd.Flags().StringVar(&contractString, "refresh-token", "", "")
	cmd.Flags().BoolVar(&contractBool, "no-discover", false, "")
	return cmd
}

func newAuthRbwCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "auth-rbw"}
	cmd.Flags().StringVar(&contractString, "item", "", "")
	cmd.Flags().StringVar(&contractString, "rbw-bin", "", "")
	cmd.Flags().StringVar(&contractString, "client-id-field", "", "")
	cmd.Flags().StringVar(&contractString, "client-secret-field", "", "")
	cmd.Flags().StringVar(&contractString, "refresh-token-field", "", "")
	cmd.Flags().BoolVar(&contractBool, "no-discover", false, "")
	return cmd
}

func newAuthURLCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "auth-url"}
	cmd.Flags().StringVar(&contractString, "redirect-uri", "", "")
	cmd.Flags().StringVar(&contractString, "scopes", "", "")
	cmd.Flags().StringVar(&contractString, "client-id", "", "")
	return cmd
}

func newTokenCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "token"}
	cmd.Flags().StringVar(&contractString, "code", "", "")
	cmd.Flags().StringVar(&contractString, "redirect-uri", "", "")
	cmd.Flags().BoolVar(&contractBool, "self-client", false, "")
	cmd.Flags().BoolVar(&contractBool, "save", false, "")
	cmd.Flags().StringVar(&contractString, "client-id", "", "")
	cmd.Flags().StringVar(&contractString, "client-secret", "", "")
	return cmd
}

func newAuthClientCredentialsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "auth-client-credentials"}
	cmd.Flags().StringVar(&contractString, "client-id", "", "")
	cmd.Flags().StringVar(&contractString, "client-secret", "", "")
	cmd.Flags().StringVar(&contractString, "scopes", "", "")
	cmd.Flags().BoolVar(&contractBool, "save", false, "")
	return cmd
}

func newAuthDeviceCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "auth-device"}
	cmd.Flags().StringVar(&contractString, "client-id", "", "")
	cmd.Flags().StringVar(&contractString, "client-secret", "", "")
	cmd.Flags().StringVar(&contractString, "scopes", "", "")
	cmd.Flags().BoolVar(&contractBool, "no-open", false, "")
	cmd.Flags().StringVar(&contractString, "interval", "", "")
	cmd.Flags().StringVar(&contractString, "timeout", "", "")
	return cmd
}

func newConfigureCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "configure"}
	cmd.Flags().StringVar(&contractString, "account-id", "", "")
	cmd.Flags().StringVar(&contractString, "inbox-folder-id", "", "")
	cmd.Flags().StringVar(&contractString, "sent-folder-id", "", "")
	cmd.Flags().StringVar(&contractString, "spam-folder-id", "", "")
	cmd.Flags().StringVar(&contractString, "trash-folder-id", "", "")
	cmd.Flags().StringVar(&contractString, "archive-folder-id", "", "")
	cmd.Flags().BoolVar(&contractBool, "save-token", false, "")
	return cmd
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{Use: "doctor"}
}

func newAccountsCmd() *cobra.Command {
	return &cobra.Command{Use: "accounts"}
}

func newFoldersCmd() *cobra.Command {
	return &cobra.Command{Use: "folders"}
}

func newInboxCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "inbox"}
	cmd.Flags().IntVar(&contractInt, "start", 0, "")
	cmd.Flags().IntVar(&contractInt, "limit", 0, "")
	return cmd
}

func newSentCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "sent"}
	cmd.Flags().IntVar(&contractInt, "start", 0, "")
	cmd.Flags().IntVar(&contractInt, "limit", 0, "")
	return cmd
}

func newSpamCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "spam"}
	cmd.Flags().IntVar(&contractInt, "start", 0, "")
	cmd.Flags().IntVar(&contractInt, "limit", 0, "")
	return cmd
}

func newTrashCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "trash"}
	cmd.Flags().IntVar(&contractInt, "start", 0, "")
	cmd.Flags().IntVar(&contractInt, "limit", 0, "")
	return cmd
}

func newArchiveCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "archive"}
	cmd.Flags().IntVar(&contractInt, "start", 0, "")
	cmd.Flags().IntVar(&contractInt, "limit", 0, "")
	return cmd
}

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "list"}
	cmd.Flags().StringVar(&contractString, "folder", "", "")
	cmd.Flags().StringVar(&contractString, "folder-id", "", "")
	cmd.Flags().StringVar(&contractString, "view", "", "")
	cmd.Flags().IntVar(&contractInt, "start", 0, "")
	cmd.Flags().IntVar(&contractInt, "limit", 0, "")
	cmd.Flags().StringVar(&contractString, "sort-by", "", "")
	return cmd
}

func newSearchCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "search"}
	cmd.Flags().StringVar(&contractString, "search-key", "", "")
	cmd.Flags().IntVar(&contractInt, "start", 0, "")
	cmd.Flags().IntVar(&contractInt, "limit", 0, "")
	return cmd
}

func newReadCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "read"}
	cmd.Flags().StringVar(&contractString, "folder", "", "")
	cmd.Flags().StringVar(&contractString, "folder-id", "", "")
	cmd.Flags().StringVar(&contractString, "message-id", "", "")
	cmd.Flags().StringVar(&contractString, "mode", "", "")
	return cmd
}

func newSendCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "send"}
	cmd.Flags().StringVar(&contractString, "from", "", "")
	cmd.Flags().StringVar(&contractString, "to", "", "")
	cmd.Flags().StringVar(&contractString, "subject", "", "")
	cmd.Flags().StringVar(&contractString, "content", "", "")
	cmd.Flags().StringVar(&contractString, "content-file", "", "")
	return cmd
}
