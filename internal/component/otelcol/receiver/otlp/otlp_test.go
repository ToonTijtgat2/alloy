package otlp_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/internal/fakeconsumer"
	"github.com/grafana/agent/internal/component/otelcol/receiver/otlp"
	"github.com/grafana/agent/internal/flow/componenttest"
	"github.com/grafana/agent/internal/flow/logging/level"
	"github.com/grafana/agent/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/dskit/backoff"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"gotest.tools/assert"
)

// Test performs a basic integration test which runs the otelcol.receiver.otlp
// component and ensures that it can receive and forward data.
func Test(t *testing.T) {
	httpAddr := getFreeAddr(t)

	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.receiver.otlp")
	require.NoError(t, err)

	cfg := fmt.Sprintf(`
		http {
			endpoint = "%s"
		}

		output {
			// no-op: will be overridden by test code.
		}
	`, httpAddr)

	require.NoError(t, err)

	var args otlp.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	// Override our settings so traces get forwarded to traceCh.
	traceCh := make(chan ptrace.Traces)
	args.Output = makeTracesOutput(traceCh)

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second))

	// Send traces in the background to our receiver.
	go func() {
		request := func() error {
			f, err := os.Open("testdata/payload.json")
			require.NoError(t, err)
			defer f.Close()

			tracesURL := fmt.Sprintf("http://%s/v1/traces", httpAddr)
			_, err = http.DefaultClient.Post(tracesURL, "application/json", f)
			return err
		}

		bo := backoff.New(ctx, backoff.Config{
			MinBackoff: 10 * time.Millisecond,
			MaxBackoff: 100 * time.Millisecond,
		})
		for bo.Ongoing() {
			if err := request(); err != nil {
				level.Error(l).Log("msg", "failed to send traces", "err", err)
				bo.Wait()
				continue
			}

			return
		}
	}()

	// Wait for our client to get a span.
	select {
	case <-time.After(time.Second):
		require.FailNow(t, "failed waiting for traces")
	case tr := <-traceCh:
		require.Equal(t, 1, tr.SpanCount())
	}
}

// makeTracesOutput returns ConsumerArguments which will forward traces to the
// provided channel.
func makeTracesOutput(ch chan ptrace.Traces) *otelcol.ConsumerArguments {
	traceConsumer := fakeconsumer.Consumer{
		ConsumeTracesFunc: func(ctx context.Context, t ptrace.Traces) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- t:
				return nil
			}
		},
	}

	return &otelcol.ConsumerArguments{
		Traces: []otelcol.Consumer{&traceConsumer},
	}
}

func getFreeAddr(t *testing.T) string {
	t.Helper()

	portNumber, err := freeport.GetFreePort()
	require.NoError(t, err)

	return fmt.Sprintf("localhost:%d", portNumber)
}

func TestUnmarshalGrpc(t *testing.T) {
	riverCfg := `
		grpc {
			endpoint = "/v1/traces"
		}

		output {
		}
	`
	var args otlp.Arguments
	err := syntax.Unmarshal([]byte(riverCfg), &args)
	require.NoError(t, err)
}

func TestUnmarshalHttp(t *testing.T) {
	riverCfg := `
		http {
			endpoint = "/v1/traces"
		}

		output {
		}
	`
	var args otlp.Arguments
	err := syntax.Unmarshal([]byte(riverCfg), &args)
	require.NoError(t, err)
	assert.Equal(t, "/v1/logs", args.HTTP.LogsURLPath)
	assert.Equal(t, "/v1/metrics", args.HTTP.MetricsURLPath)
	assert.Equal(t, "/v1/traces", args.HTTP.TracesURLPath)
}

func TestUnmarshalHttpUrls(t *testing.T) {
	riverCfg := `
		http {
			endpoint = "/v1/traces"
			traces_url_path = "custom/traces"
			metrics_url_path = "custom/metrics"
			logs_url_path = "custom/logs"
		}

		output {
		}
	`
	var args otlp.Arguments
	err := syntax.Unmarshal([]byte(riverCfg), &args)
	require.NoError(t, err)
	assert.Equal(t, "custom/logs", args.HTTP.LogsURLPath)
	assert.Equal(t, "custom/metrics", args.HTTP.MetricsURLPath)
	assert.Equal(t, "custom/traces", args.HTTP.TracesURLPath)
}

func TestDebugMetricsConfig(t *testing.T) {
	tests := []struct {
		testName string
		agentCfg string
		expected otelcol.DebugMetricsArguments
	}{
		{
			testName: "default",
			agentCfg: `
			grpc {
				endpoint = "/v1/traces"
			}
			output {}
			`,
			expected: otelcol.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
			},
		},
		{
			testName: "explicit_false",
			agentCfg: `
			grpc {
				endpoint = "/v1/traces"
			}
			debug_metrics {
				disable_high_cardinality_metrics = false
			}
			output {}
			`,
			expected: otelcol.DebugMetricsArguments{
				DisableHighCardinalityMetrics: false,
			},
		},
		{
			testName: "explicit_true",
			agentCfg: `
			grpc {
				endpoint = "/v1/traces"
			}
			debug_metrics {
				disable_high_cardinality_metrics = true
			}
			output {}
			`,
			expected: otelcol.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args otlp.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tc.agentCfg), &args))
			_, err := args.Convert()
			require.NoError(t, err)

			require.Equal(t, tc.expected, args.DebugMetricsConfig())
		})
	}
}
