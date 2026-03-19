package telemetry

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func ProvideTracer(lifecycle fx.Lifecycle, logger *zap.Logger) (*sdktrace.TracerProvider, error) {
	serviceName := os.Getenv("APP_NAME")
	if serviceName == "" {
		serviceName = "unknown-service"
	}
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") // expects URL, e.g. http://jaeger:4318

	opts := []otlptracehttp.Option{
		otlptracehttp.WithInsecure(), // Dev only
	}
	if endpoint != "" {
		opts = append(opts, otlptracehttp.WithEndpointURL(endpoint))
	}

	exporter, err := otlptracehttp.New(context.Background(), opts...)
	if err != nil {
		return nil, err
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			"",
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set globals
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Info("Shutting down Tracer Provider")
			return tp.Shutdown(ctx)
		},
	})

	return tp, nil
}
