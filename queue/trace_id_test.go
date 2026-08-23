package queue

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"
)

func TestRequestHeadersPropagatesTraceID(t *testing.T) {
	ctx := ContextWithTraceID(context.Background(), "queue-trace-1")
	headers := requestHeaders(ctx, EnqueueRequest{Headers: map[string]string{"job_id": "job-1"}})

	require.Equal(t, "queue-trace-1", headers[TraceIDHeader])
	require.Equal(t, "job-1", headers["job_id"])
}

func TestRequestHeadersPreservesExplicitTraceID(t *testing.T) {
	headers := requestHeaders(ContextWithTraceID(context.Background(), "context-trace"), EnqueueRequest{
		Headers: map[string]string{TraceIDHeader: "header-trace"},
	})

	require.Equal(t, "header-trace", headers[TraceIDHeader])
}

func TestTraceContextRestoresTaskTraceID(t *testing.T) {
	task := asynq.NewTaskWithHeaders("job.test", []byte(`{}`), map[string]string{
		TraceIDHeader: "worker-trace-1",
	})

	ctx := traceContext(context.Background(), task)
	require.Equal(t, "worker-trace-1", TraceIDFromContext(ctx))
}
