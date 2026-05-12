// Hand-authored: chrome interaction subcommands (click, type, form-input, scroll).
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newChromeClickCmd(flags *rootFlags) *cobra.Command {
	var ref, button string
	var x, y int
	var double, xSet, ySet bool
	cmd := &cobra.Command{
		Use:   "click <id>",
		Short: "Click an element. Either --ref (from read-page/find) or --x --y coordinates.",
		Long: `Use --ref whenever possible — refs are stable, semantic, and resilient to
layout shifts. Coordinates are a fallback for canvases/maps/charts where the
DOM doesn't expose an element.`,
		Example: `  orgo-pp-cli chrome click <id> --ref ref_3
  orgo-pp-cli chrome click <id> --x 640 --y 360 --button right
  orgo-pp-cli chrome click <id> --ref ref_5 --double`,
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			if ref != "" {
				body["ref"] = ref
			}
			if xSet {
				body["x"] = x
			}
			if ySet {
				body["y"] = y
			}
			if button != "" {
				body["button"] = button
			}
			if double {
				body["double"] = true
			}
			if ref == "" && !(xSet && ySet) {
				return fmt.Errorf("either --ref or both --x and --y are required")
			}
			resp, err := runChromeCall(cmd, flags, args, "POST", "/click", body)
			if err != nil {
				return err
			}
			return printChromeResp(cmd, flags, resp)
		},
	}
	cmd.Flags().StringVar(&ref, "ref", "", "Element ref from read-page or find (e.g., ref_3)")
	cmd.Flags().IntVar(&x, "x", 0, "X coordinate (pixels from left)")
	cmd.Flags().IntVar(&y, "y", 0, "Y coordinate (pixels from top)")
	cmd.Flags().StringVar(&button, "button", "", "Mouse button: left, right, or middle (default: left)")
	cmd.Flags().BoolVar(&double, "double", false, "Double-click")
	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		xSet = cmd.Flags().Changed("x")
		ySet = cmd.Flags().Changed("y")
	}
	return cmd
}

func newChromeTypeCmd(flags *rootFlags) *cobra.Command {
	var text, key string
	cmd := &cobra.Command{
		Use:   "type <id>",
		Short: "Type text into the focused Chrome element (--text), or press a keyboard shortcut (--key).",
		Long: `Use --text for prose ('hello world') and --key for shortcuts ('Enter',
'ctrl+a', 'Backspace'). Exactly one of --text or --key must be set.`,
		Example: `  orgo-pp-cli chrome type <id> --text "hello world"
  orgo-pp-cli chrome type <id> --key Enter
  orgo-pp-cli chrome type <id> --key ctrl+a`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if (text == "" && key == "") || (text != "" && key != "") {
				return fmt.Errorf("provide exactly one of --text or --key")
			}
			var path string
			var body map[string]any
			if key != "" {
				path, body = "/key", map[string]any{"key": key}
			} else {
				path, body = "/type", map[string]any{"text": text}
			}
			resp, err := runChromeCall(cmd, flags, args, "POST", path, body)
			if err != nil {
				return err
			}
			return printChromeResp(cmd, flags, resp)
		},
	}
	cmd.Flags().StringVar(&text, "text", "", "Text to type into the focused element")
	cmd.Flags().StringVar(&key, "key", "", "Key or shortcut to press (e.g. Enter, ctrl+a, Backspace)")
	return cmd
}

func newChromeFormInputCmd(flags *rootFlags) *cobra.Command {
	var ref, value string
	cmd := &cobra.Command{
		Use:   "form-input <id>",
		Short: "Set a form field value by element ref. Works for inputs, selects, and checkboxes.",
		Long: `Sets value directly on a form element by ref (from read-page or find).
For checkboxes pass 'true'/'false'; for selects pass an option value; for
text inputs pass the string. Faster and more reliable than focus+type for
forms.`,
		Example: `  orgo-pp-cli chrome form-input <id> --ref ref_7 --value "hello@example.com"
  orgo-pp-cli chrome form-input <id> --ref ref_8 --value true`,
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := runChromeCall(cmd, flags, args, "POST", "/form_input", map[string]any{
				"ref":   ref,
				"value": value,
			})
			if err != nil {
				return err
			}
			return printChromeResp(cmd, flags, resp)
		},
	}
	cmd.Flags().StringVar(&ref, "ref", "", "Element ref from read-page or find (required)")
	cmd.Flags().StringVar(&value, "value", "", "Value to set on the field (required)")
	_ = cmd.MarkFlagRequired("ref")
	_ = cmd.MarkFlagRequired("value")
	return cmd
}

func newChromeScrollCmd(flags *rootFlags) *cobra.Command {
	var direction string
	var amount, x, y int
	var xSet, ySet bool
	cmd := &cobra.Command{
		Use:     "scroll <id>",
		Short:   "Scroll the Chrome page up/down/left/right.",
		Example: `  orgo-pp-cli chrome scroll <id> --direction down --amount 5`,
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{"direction": direction}
			if amount > 0 {
				body["amount"] = amount
			}
			if xSet {
				body["x"] = x
			}
			if ySet {
				body["y"] = y
			}
			resp, err := runChromeCall(cmd, flags, args, "POST", "/scroll", body)
			if err != nil {
				return err
			}
			return printChromeResp(cmd, flags, resp)
		},
	}
	cmd.Flags().StringVar(&direction, "direction", "", "Scroll direction: up, down, left, or right (required)")
	cmd.Flags().IntVar(&amount, "amount", 0, "Number of scroll ticks (default: 3)")
	cmd.Flags().IntVar(&x, "x", 0, "X coordinate to scroll at")
	cmd.Flags().IntVar(&y, "y", 0, "Y coordinate to scroll at")
	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		xSet = cmd.Flags().Changed("x")
		ySet = cmd.Flags().Changed("y")
	}
	_ = cmd.MarkFlagRequired("direction")
	return cmd
}
