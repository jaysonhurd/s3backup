package telemetry

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestStart verifies span outcomes and measurements with caller-owned SDKs.
func TestStart(t *testing.T) {
	ctx := context.Background()
	oldTracer, oldMeter := otel.GetTracerProvider(), otel.GetMeterProvider()
	recorder := tracetest.NewSpanRecorder()
	traces := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	reader := sdkmetric.NewManualReader()
	meters := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetTracerProvider(traces)
	otel.SetMeterProvider(meters)
	t.Cleanup(func() {
		otel.SetTracerProvider(oldTracer)
		otel.SetMeterProvider(oldMeter)
		if err := traces.Shutdown(ctx); err != nil {
			t.Error(err)
		}
		if err := meters.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	parent, finish := Start(ctx, "backup.directory")
	_, endUpload := Start(parent, "backup.upload")
	endUpload(errors.New("upload failed"))
	finish(nil)
	spans := recorder.Ended()
	if len(spans) != 2 || spans[0].Status().Code != codes.Error || spans[0].Parent().SpanID() != spans[1].SpanContext().SpanID() {
		t.Fatalf("unexpected spans: %+v", spans)
	}
	var data metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &data); err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, scope := range data.ScopeMetrics {
		for _, value := range scope.Metrics {
			found[value.Name] = true
		}
	}
	if !found["s3backup.operations"] || !found["s3backup.operation.duration"] {
		t.Fatalf("missing measurements: %v", found)
	}
}
