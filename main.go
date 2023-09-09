package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
)

var resource *sdkresource.Resource
var initResourcesOnce sync.Once

func initResource() *sdkresource.Resource {
	initResourcesOnce.Do(func() {
		extraResources, _ := sdkresource.New(
			context.Background(),
			sdkresource.WithOS(),
			sdkresource.WithProcess(),
			sdkresource.WithContainer(),
			sdkresource.WithHost(),
			sdkresource.WithAttributes(attribute.String("service.name", "oteltestsource")),
		)
		resource, _ = sdkresource.Merge(
			sdkresource.Default(),
			extraResources,
		)
	})
	return resource
}

func initMeterProvider() *sdkmetric.MeterProvider {
	ctx := context.Background()

	exporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithInsecure())
	if err != nil {
		log.Fatalf("new otlp metric grpc exporter failed: %v", err)
		return nil
	}

	v := sdkmetric.NewView(sdkmetric.Instrument{
		Name: "*histogram",
		Kind: sdkmetric.InstrumentKindHistogram,
	}, sdkmetric.Stream{
		Aggregation: sdkmetric.AggregationBase2ExponentialHistogram{MaxSize: 160, NoMinMax: true, MaxScale: 20},
	})

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		sdkmetric.WithResource(initResource()),
		sdkmetric.WithView(v),
	)

	otel.SetMeterProvider(mp)
	return mp
}

func main() {
	mp := initMeterProvider()
	defer func() {
		if err := mp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down meter provider: %v", err)
		}
	}()

	meter := mp.Meter("test-meter")
	histogram, err := meter.Float64Histogram(
		"otel_manual_histogram",
		metric.WithUnit("ms"),
		metric.WithDescription("test histogram"),
	)
	if err != nil {
		log.Fatalf("new histogram failed: %v", err)
		return
	}
	
	v := rand.Float64()*1000
	scanner := bufio.NewScanner(os.Stdin)
	scan := func() bool {
		fmt.Printf("Last observation: %v\n", v)
		return scanner.Scan()
	}
	for scan() {
		histogram.Record(context.Background(), v)
		v = rand.Float64()*1000
	}
}