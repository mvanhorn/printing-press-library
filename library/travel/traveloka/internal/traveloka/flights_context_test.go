package traveloka

// Simulated source transports exercise caller-context failures without live requests.
import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestSimulatedCoreFlightCallerContextSurvivesWorkflow(t *testing.T) {
	for _, stage := range []string{"refresh_wait", "prefetch"} {
		for _, cancellation := range []bool{false, true} {
			name := stage + "/deadline"
			if cancellation {
				name = stage + "/cancel"
			}
			t.Run(name, func(t *testing.T) {
				c := simulatedCoreClient(t)
				if err := c.SetRateLimit(0); err != nil {
					t.Fatal(err)
				}
				q := coreFlightQuery()
				q.MaxCandidates, q.Limit = 1, 1
				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				defer cancel()
				initialCalls, prefetchCalls := 0, 0
				c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
					coreRequestData(t, r)
					switch r.URL.Path {
					case flightInitialPath:
						initialCalls++
						if stage == "refresh_wait" {
							data := coreFlightResult([]any{}, false)
							sourceObject(data["meta"])["refreshDelayMillisecond"] = "1000"
							response := coreResponse(t, r, data)
							if cancellation {
								cancel()
							}
							<-ctx.Done()
							return response, nil
						}
						return coreResponse(t, r, coreFlightResult([]any{coreFlightRow("SIMULATED-out", "SIN", "CGK", "10000")}, true)), nil
					case flightPrefetchPath:
						prefetchCalls++
						if cancellation {
							cancel()
						}
						<-ctx.Done()
						return nil, fmt.Errorf("SIMULATED prefetch context failure: %w", ctx.Err())
					default:
						t.Fatalf("caller expiration performed another source operation: %s", r.URL.Path)
						return nil, nil
					}
				}))
				snapshot, err := c.SearchFlights(ctx, q)
				wantCode, wantCause := "TIMEOUT", context.DeadlineExceeded
				if cancellation {
					wantCode, wantCause = "CANCELED", context.Canceled
				}
				var source *APIError
				if snapshot != nil || !errors.As(err, &source) || source.Code != wantCode || !errors.Is(err, wantCause) {
					t.Fatalf("workflow lost caller-context code/cause: snapshot=%v error=%v", snapshot, err)
				}
				if source.Retryable == cancellation {
					t.Fatalf("workflow changed caller-context retryability: %v", err)
				}
				wantPrefetch := 0
				if stage == "prefetch" {
					wantPrefetch = 1
				}
				if initialCalls != 1 || prefetchCalls != wantPrefetch {
					t.Fatalf("wrong workflow stage: initial=%d prefetch=%d", initialCalls, prefetchCalls)
				}
			})
		}
	}
}

func TestSimulatedCoreFlightRefreshBoundKeepsActiveCallerIncomplete(t *testing.T) {
	c := simulatedCoreClient(t)
	calls := 0
	c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		coreRequestData(t, r)
		data := coreFlightResult([]any{}, false)
		sourceObject(data["meta"])["refreshDelayMillisecond"] = "31000"
		return coreResponse(t, r, data), nil
	}))
	ctx := context.Background()
	snapshot, err := c.SearchFlights(ctx, coreFlightQuery())
	if err != nil || ctx.Err() != nil || snapshot == nil || snapshot.Status != "incomplete" || snapshot.SearchComplete || len(snapshot.Warnings) == 0 || calls != 1 {
		t.Fatalf("source refresh bound became a caller timeout: snapshot=%v error=%v calls=%d", snapshot, err, calls)
	}
}
