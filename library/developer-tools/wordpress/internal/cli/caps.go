// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/cliutil"
)

const capsWorkerCount = 8

type wordpressRESTIndex struct {
	Routes     map[string]json.RawMessage `json:"routes"`
	Namespaces []string                   `json:"namespaces"`
}

type capsRouteResult struct {
	Route            string   `json:"route"`
	Anonymous        []string `json:"anonymous_methods"`
	Credentials      []string `json:"credential_methods"`
	Delta            []string `json:"delta"`
	Error            string   `json:"error,omitempty"`
	AnonStatus       int      `json:"anonymous_status,omitempty"`
	CredentialStatus int      `json:"credential_status,omitempty"`
}

type capsOutput struct {
	Target        string            `json:"target"`
	Namespace     string            `json:"namespace"`
	Routes        []capsRouteResult `json:"routes"`
	ScannedRoutes int               `json:"scanned_routes"`
	MaxScanRoutes int               `json:"max_scan_routes"`
	FailedRoutes  int               `json:"failed_routes"`
	Note          string            `json:"note,omitempty"`
}

type optionsResult struct {
	methods []string
	status  int
	err     error
}

func newCapsCmd(flags *rootFlags) *cobra.Command {
	namespace := "wp/v2"
	maxScanRoutes := 200
	if cliutil.IsDogfoodEnv() {
		maxScanRoutes = 25
	}
	cmd := &cobra.Command{
		Use:   "caps",
		Short: "Probe route permissions without performing writes",
		Example: "  wordpress-pp-cli caps --json\n" +
			"  wordpress-pp-cli caps --namespace wc/v3 --max-scan-routes 100",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would probe WordPress route capabilities with anonymous and configured credentials")
				return nil
			}
			if len(args) != 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("caps does not accept positional arguments"))
			}
			if strings.TrimSpace(namespace) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--namespace must not be empty"))
			}
			if maxScanRoutes <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--max-scan-routes must be greater than zero"))
			}

			runtime, err := resolveWordPressRuntime(flags, "")
			if err != nil {
				return configErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			httpClient := &http.Client{Timeout: flags.timeout}
			out, err := runCaps(ctx, httpClient, runtime, strings.Trim(namespace, "/"), maxScanRoutes)
			if err != nil {
				return apiErr(err)
			}
			if out.FailedRoutes > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: capability probes failed for %d route(s)\n", out.FailedRoutes)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			return printCapsHuman(cmd, out)
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", namespace, "REST namespace to scan")
	cmd.Flags().IntVar(&maxScanRoutes, "max-scan-routes", maxScanRoutes, "maximum collection routes to probe")
	return cmd
}

// newNovelCapsCmd keeps the generated root's scaffold hook buildable while
// init replaces that scaffold with the hand-authored command above.
func newNovelCapsCmd(flags *rootFlags) *cobra.Command {
	return newCapsCmd(flags)
}

func runCaps(ctx context.Context, httpClient *http.Client, runtime wordpressRuntime, namespace string, maxScanRoutes int) (capsOutput, error) {
	out := capsOutput{
		Target: runtime.Origin, Namespace: namespace,
		Routes: make([]capsRouteResult, 0), MaxScanRoutes: maxScanRoutes,
	}
	indexProbe := executeProbe(ctx, httpClient, runtime, "rest-root", runtimeProbeForm(runtime), http.MethodGet, runtimeRESTURL(runtime, "/", nil), runtime.HasAuth)
	if !indexProbe.OK {
		return out, probeError("fetch REST route index", indexProbe)
	}
	var index wordpressRESTIndex
	if err := json.Unmarshal(indexProbe.body, &index); err != nil {
		return out, fmt.Errorf("decode REST route index: %w", err)
	}

	prefix := "/" + strings.Trim(namespace, "/") + "/"
	eligible := make([]string, 0)
	for route := range index.Routes {
		if strings.HasPrefix(route, prefix) && !strings.Contains(route, "(?P<") {
			eligible = append(eligible, route)
		}
	}
	sort.Strings(eligible)
	if len(eligible) > maxScanRoutes {
		eligible = eligible[:maxScanRoutes]
		out.Note = fmt.Sprintf("scan capped at %d routes; widen it with --max-scan-routes", maxScanRoutes)
	}
	out.ScannedRoutes = len(eligible)
	if len(eligible) == 0 {
		return out, nil
	}

	type job struct {
		index int
		route string
	}
	type result struct {
		index int
		value capsRouteResult
	}
	jobs := make(chan job)
	results := make(chan result, len(eligible))
	workerCount := capsWorkerCount
	if len(eligible) < workerCount {
		workerCount = len(eligible)
	}
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for worker := 0; worker < workerCount; worker++ {
		go func() {
			defer workers.Done()
			for item := range jobs {
				results <- result{index: item.index, value: probeRouteCapabilities(ctx, httpClient, runtime, item.route)}
			}
		}()
	}
	go func() {
		for index, route := range eligible {
			jobs <- job{index: index, route: route}
		}
		close(jobs)
		workers.Wait()
		close(results)
	}()

	ordered := make([]capsRouteResult, len(eligible))
	for item := range results {
		ordered[item.index] = item.value
		if item.value.Error != "" {
			out.FailedRoutes++
		}
	}
	out.Routes = ordered
	return out, nil
}

func probeRouteCapabilities(ctx context.Context, httpClient *http.Client, runtime wordpressRuntime, route string) capsRouteResult {
	result := capsRouteResult{
		Route: route, Anonymous: make([]string, 0),
		Credentials: make([]string, 0), Delta: make([]string, 0),
	}
	target := runtimeRESTURL(runtime, route, nil)
	anonymous := executeOptions(ctx, httpClient, runtime, target, false)
	authenticated := executeOptions(ctx, httpClient, runtime, target, true)
	result.Anonymous = anonymous.methods
	result.Credentials = authenticated.methods
	result.AnonStatus = anonymous.status
	result.CredentialStatus = authenticated.status
	result.Delta = methodDifference(authenticated.methods, anonymous.methods)
	errors := make([]string, 0, 2)
	if anonymous.err != nil {
		errors = append(errors, "anonymous: "+anonymous.err.Error())
	}
	if authenticated.err != nil {
		errors = append(errors, "credentials: "+authenticated.err.Error())
	}
	result.Error = strings.Join(errors, "; ")
	return result
}

func runtimeProbeForm(runtime wordpressRuntime) string {
	if runtime.RestRouteFallback {
		return "rest_route"
	}
	return "pretty"
}

func executeOptions(ctx context.Context, httpClient *http.Client, runtime wordpressRuntime, target string, authenticated bool) optionsResult {
	probe := executeProbe(ctx, httpClient, runtime, "capability", "pretty", http.MethodOptions, target, authenticated)
	result := optionsResult{methods: make([]string, 0), status: probe.Status}
	if probe.Error != "" || !probe.OK {
		result.err = probeError("OPTIONS", probe)
		return result
	}
	result.methods = parseAllowHeader(probe.headers.Get("Allow"))
	return result
}

func parseAllowHeader(value string) []string {
	methods := make([]string, 0)
	seen := make(map[string]struct{})
	for _, item := range strings.Split(value, ",") {
		method := strings.ToUpper(strings.TrimSpace(item))
		if method == "" {
			continue
		}
		if _, ok := seen[method]; ok {
			continue
		}
		seen[method] = struct{}{}
		methods = append(methods, method)
	}
	sort.Strings(methods)
	return methods
}

func methodDifference(left, right []string) []string {
	rightSet := make(map[string]struct{}, len(right))
	for _, method := range right {
		rightSet[method] = struct{}{}
	}
	delta := make([]string, 0)
	for _, method := range left {
		if _, ok := rightSet[method]; !ok {
			delta = append(delta, method)
		}
	}
	sort.Strings(delta)
	return delta
}

func probeError(action string, probe diagnoseProbe) error {
	if probe.Error != "" {
		return fmt.Errorf("%s: %s", action, probe.Error)
	}
	parts := []string{fmt.Sprintf("%s returned HTTP %d", action, probe.Status)}
	if probe.Code != "" {
		parts = append(parts, "code="+probe.Code)
	}
	if message, ok := probe.Details["message"].(string); ok && message != "" {
		parts = append(parts, "message="+message)
	}
	if details, ok := probe.Details["parameter_details"]; ok {
		encoded, _ := json.Marshal(details)
		parts = append(parts, "details="+string(encoded))
	}
	return fmt.Errorf("%s", strings.Join(parts, " "))
}

func printCapsHuman(cmd *cobra.Command, out capsOutput) error {
	fmt.Fprintf(cmd.OutOrStdout(), "caps: scanned %d route(s) in %s (limit %d)\n", out.ScannedRoutes, out.Namespace, out.MaxScanRoutes)
	rows := make([]map[string]any, 0, len(out.Routes))
	for _, route := range out.Routes {
		rows = append(rows, map[string]any{
			"route":       route.Route,
			"anonymous":   strings.Join(route.Anonymous, ","),
			"credentials": strings.Join(route.Credentials, ","),
			"delta":       strings.Join(route.Delta, ","),
			"error":       route.Error,
		})
	}
	if err := printAutoTable(cmd.OutOrStdout(), rows); err != nil {
		return err
	}
	if out.Note != "" {
		fmt.Fprintln(cmd.OutOrStdout(), "note:", out.Note)
	}
	return nil
}
