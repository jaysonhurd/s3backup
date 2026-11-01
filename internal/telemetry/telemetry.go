// Package telemetry instruments operations using caller-configured OTel providers.
package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
)

const scope = "github.com/jaysonhurd/s3backup"

// Start starts a span and returns a completion function that records its outcome.
// Without configured providers, instrumentation remains a no-op.
func Start(ctx context.Context, operation string) (context.Context, func(error)) {
	started := time.Now()
	ctx, span := otel.Tracer(scope).Start(ctx, operation)
	return ctx, func(err error) {
		defer span.End()
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "operation failed")
		}
		meter := otel.Meter(scope)
		attrs := metric.WithAttributes(attribute.String("operation", operation), attribute.Bool("failed", err != nil))
		if counter, metricErr := meter.Int64Counter("s3backup.operations"); metricErr == nil {
			counter.Add(ctx, 1, attrs)
		}
		if duration, metricErr := meter.Float64Histogram("s3backup.operation.duration", metric.WithUnit("s")); metricErr == nil {
			duration.Record(ctx, time.Since(started).Seconds(), attrs)
		}
	}
}
