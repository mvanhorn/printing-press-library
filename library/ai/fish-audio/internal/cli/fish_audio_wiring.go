// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel commands: pre-flight validation, the combined wallet view, and the
// registration hook that attaches every hand-written command to the generated
// tree without editing root.go.
// pp:data-source live
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/fishaudio"
	"github.com/spf13/cobra"
)

// init registers the hand-written commands. Wiring through the generated hook
// instead of editing root.go is what lets a regeneration keep both the source
// and its attachment points.
func init() {
	registerNovelCommand(attachFishAudioCommands)
}

// attachFishAudioCommands hangs every hand-written command off its parent.
//
// The hook runs before root.go attaches the `render` and `voice` groups, so
// those two are built here with their subcommands already in place; root.go's
// own addNovelCommandIfAbsent then finds the name taken and leaves them alone.
// That is also where two generated parent Shorts are corrected: both had
// leaked their capability-group label ("Local state that compounds", "Verify
// before you ship") into user-facing help.
func attachFishAudioCommands(root *cobra.Command, flags *rootFlags) {
	for _, parent := range root.Commands() {
		switch parent.Name() {
		case "tts":
			addNovelCommandIfAbsent(parent, newFishTtsResolveCmd(flags))
		case "asr":
			addNovelCommandIfAbsent(parent, newFishAsrTranscribeCmd(flags))
		case "wallet":
			addNovelCommandIfAbsent(parent, newFishWalletBalanceCmd(flags))
			addNovelCommandIfAbsent(parent, newFishWalletCreditCmd(flags))
		}
	}

	renderGroup := newNovelRenderCmd(flags)
	renderGroup.Short = "Local render history, spend, and diffs"
	addNovelCommandIfAbsent(root, renderGroup)

	voiceGroup := newNovelVoiceCmd(flags)
	voiceGroup.Short = "Clone, design, discover, and verify voices"
	addNovelCommandIfAbsent(voiceGroup, newFishVoiceCloneCmd(flags))
	addNovelCommandIfAbsent(voiceGroup, newFishVoiceDesignCmd(flags))
	addNovelCommandIfAbsent(voiceGroup, newFishVoiceDesignSaveCmd(flags))
	addNovelCommandIfAbsent(voiceGroup, newFishVoiceDiscoverCmd(flags))
	addNovelCommandIfAbsent(root, voiceGroup)
}

// newFishWalletCreditCmd exposes the API credit ledger under the name the
// capability index uses. It reuses the generated `wallet api-credit get`
// command verbatim, renamed, so there is exactly one implementation of the
// call and its flags.
func newFishWalletCreditCmd(flags *rootFlags) *cobra.Command {
	cmd := newWalletApiCreditGetCmd(flags)
	cmd.Use = "credit"
	cmd.Short = "Get the developer API credit balance"
	cmd.Example = strings.Trim(`
  fish-audio-pp-cli wallet credit
  fish-audio-pp-cli wallet credit --check-free-credit --json
`, "\n")
	return cmd
}

// walletLedger is one of the two balances `wallet balance` reports.
type walletLedger struct {
	Path  string          `json:"path"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

// walletBalanceView is the JSON contract `wallet balance` prints. The note is
// part of the contract, not decoration: conflating the two ledgers is the
// single most common billing mistake on this API.
type walletBalanceView struct {
	APICredit walletLedger `json:"api_credit"`
	Package   walletLedger `json:"package"`
	Note      string       `json:"note"`
}

// packagePath is the subscription package ledger.
const packagePath = "/wallet/self/package"

// twoLedgerNote states the distinction in one line.
const twoLedgerNote = "two separate ledgers: api_credit pays for developer API bytes, package is the subscription credit pool spent by the web app"

func newFishWalletBalanceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "balance",
		Short: "Show both wallet ledgers side by side",
		Long: `Reads the developer API credit ledger and the subscription package ledger in
one command and labels which is which.

They are separate balances. API renders draw on api_credit; a full package
balance does not pay for a single API byte. Conflating them is the most common
billing surprise on this API, so this command never merges them into one
number.

A ledger that fails to read is reported with its error while the other still
prints, so one failure cannot hide the balance you can see.`,
		Example: strings.Trim(`
  fish-audio-pp-cli wallet balance
  fish-audio-pp-cli wallet balance --json
  fish-audio-pp-cli wallet balance --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,4,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "wallet balance")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			view := walletBalanceView{
				APICredit: readLedger(ctx, c, apiCreditPath),
				Package:   readLedger(ctx, c, packagePath),
				Note:      twoLedgerNote,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if printErr := printJSONFiltered(cmd.OutOrStdout(), view, flags); printErr != nil {
					return printErr
				}
			} else {
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "api credit (developer API, per byte)\n  %s\n", ledgerLine(view.APICredit))
				fmt.Fprintf(out, "package (subscription pool, web app)\n  %s\n", ledgerLine(view.Package))
				fmt.Fprintf(out, "\n%s\n", view.Note)
			}
			// Both ledgers failing is a real failure; one is a partial result
			// the caller can still act on.
			if view.APICredit.Error != "" && view.Package.Error != "" {
				return apiErr(fmt.Errorf("neither wallet ledger could be read: %s", view.APICredit.Error))
			}
			if view.APICredit.Error != "" || view.Package.Error != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: one of the two wallet ledgers could not be read; the other is reported above")
			}
			return nil
		},
	}
	return cmd
}

// readLedger fetches one wallet endpoint, preserving the error instead of
// aborting so the other ledger still reports.
func readLedger(ctx context.Context, c *client.Client, path string) walletLedger {
	data, err := c.Get(ctx, path, nil)
	if err != nil {
		return walletLedger{Path: path, Error: err.Error()}
	}
	return walletLedger{Path: path, Data: data}
}

// ledgerLine renders one ledger for the human surface.
func ledgerLine(ledger walletLedger) string {
	if ledger.Error != "" {
		return "unavailable: " + ledger.Error
	}
	if len(ledger.Data) == 0 {
		return "no data returned"
	}
	return string(ledger.Data)
}

// ttsResolveView is the JSON contract `tts resolve` prints.
type ttsResolveView struct {
	Voice    string   `json:"voice"`
	Model    string   `json:"model"`
	VoiceOK  bool     `json:"voice_ok"`
	ModelOK  bool     `json:"model_ok"`
	Title    string   `json:"title,omitempty"`
	State    string   `json:"state,omitempty"`
	Warnings []string `json:"warnings"`
}

func newFishTtsResolveCmd(flags *rootFlags) *cobra.Command {
	var (
		flagVoice string
		flagModel string
	)

	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Check that a voice and a model are usable before spending a render",
		Long: `Validates a render's two identity inputs without synthesizing anything.

--model is checked against the closed set the model header accepts. This
matters because the API silently falls back to its default on an unrecognized
value instead of returning an error, so a typo would render in the wrong voice
engine and still look like a success.

--voice is checked with GET /model/{id}. A voice that is still training is
reported through "state" rather than treated as ready.

Warnings never fail the command; voice_ok and model_ok carry the verdict.`,
		Example: strings.Trim(`
  fish-audio-pp-cli tts resolve --voice 7f92f8afb8ec43bf81429cc1c9199cb1
  fish-audio-pp-cli tts resolve --voice 7f92f8afb8ec43bf81429cc1c9199cb1 --model s2.1-pro-free --json
  fish-audio-pp-cli tts resolve --voice 7f92f8afb8ec43bf81429cc1c9199cb1 --model s1 --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--voice=example-model-id;--dry-run",
			"pp:typed-exit-codes": "0,2,3,4,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "tts resolve")
			}
			if strings.TrimSpace(flagVoice) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--voice is required: pass the model_id you plan to render with"))
			}
			view := ttsResolveView{Voice: flagVoice, Model: flagModel, Warnings: make([]string, 0)}

			model, warning, modelErr := fishaudio.ValidateModel(flagModel)
			if modelErr != nil {
				view.Warnings = append(view.Warnings, modelErr.Error())
			} else {
				view.ModelOK = true
				view.Model = model
				if warning != "" {
					view.Warnings = append(view.Warnings, warning)
				}
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, getErr := c.Get(ctx, fishModelPath+"/"+pathParamSegmentValue(flagVoice), nil)
			if getErr != nil {
				view.Warnings = append(view.Warnings, fmt.Sprintf("voice %s did not resolve: %v", flagVoice, getErr))
			} else {
				var model struct {
					Title string `json:"title"`
					State string `json:"state"`
				}
				if json.Unmarshal(data, &model) == nil {
					view.Title = model.Title
					view.State = model.State
				}
				view.VoiceOK = true
				if model.State != "" && model.State != "trained" {
					view.Warnings = append(view.Warnings, fmt.Sprintf("voice %s is in state %q, not \"trained\"; a render may fail or use an unfinished model", flagVoice, model.State))
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "voice %s: %s\n", view.Voice, okWord(view.VoiceOK))
			if view.Title != "" {
				fmt.Fprintf(out, "  title %s, state %s\n", view.Title, orDash(view.State))
			}
			fmt.Fprintf(out, "model %s: %s\n", orDash(view.Model), okWord(view.ModelOK))
			for _, warn := range view.Warnings {
				fmt.Fprintf(out, "  warning: %s\n", warn)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagVoice, "voice", "", "Voice model_id to check with GET /model/{id}")
	cmd.Flags().StringVar(&flagModel, "model", fishaudio.DefaultModel, "TTS model to validate (one of: s1, s2-pro, s2.1-pro, s2.1-pro-free)")
	return cmd
}

// okWord renders a boolean verdict as a word a human reads faster than true or
// false.
func okWord(ok bool) string {
	if ok {
		return "ok"
	}
	return "not usable"
}
