package dev.knative.eventing.kafka.broker.core.metrics;

import io.micrometer.core.instrument.Clock;
import io.micrometer.core.instrument.MeterRegistry;
import io.micrometer.prometheus.PrometheusConfig;
import io.micrometer.prometheus.PrometheusMeterRegistry;
import io.prometheus.client.Collector;
import io.prometheus.client.CollectorRegistry;
import io.prometheus.client.exporter.common.TextFormat;

import javax.annotation.Nonnull;
import java.io.IOException;
import java.io.StringWriter;
import java.io.Writer;
import java.util.ArrayList;
import java.util.Enumeration;
import java.util.List;
import java.util.Map;
import java.util.NoSuchElementException;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;

public class CompositePrometheusMeterRegistry extends PrometheusMeterRegistry {

  private final Map<String, PrometheusMeterRegistry> registries;
  private final PrometheusConfig prometheusConfig;

  public CompositePrometheusMeterRegistry(final PrometheusConfig config) {
    this(config, Clock.SYSTEM);
  }

  public CompositePrometheusMeterRegistry(final PrometheusConfig config,
                                          final Clock clock) {
    super(config, new CollectorRegistry(), clock);
    this.prometheusConfig = config;
    this.registries = new ConcurrentHashMap<>();
  }

  public MeterRegistry addRegistry(final String key) {
    return this.registries.compute(key, (k, v) -> v == null ?
      new PrometheusMeterRegistry(this.prometheusConfig, new CollectorRegistry(), clock) :
      v
    );
  }

  public PrometheusMeterRegistry removeRegistry(final String key) {
    return this.registries.remove(key);
  }

  @Override
  public void scrape(final @Nonnull Writer writer, final @Nonnull String contentType) throws IOException {
    scrape(writer, contentType, null);
  }

  @Override
  @Nonnull
  public String scrape() {
    return scrape(TextFormat.CONTENT_TYPE_004);
  }

  @Override
  @Nonnull
  public String scrape(final @Nonnull String contentType) {
    Writer writer = new StringWriter();
    try {
      scrape(writer, contentType);
    } catch (IOException e) {
      throw new RuntimeException(e);
    }
    return writer.toString();
  }

  @Override
  public void scrape(final @Nonnull Writer writer) throws IOException {
    scrape(writer, TextFormat.CONTENT_TYPE_004);
  }

  @Override
  @Nonnull
  public String scrape(final @Nonnull String contentType, final Set<String> includedNames) {
    Writer writer = new StringWriter();
    try {
      scrape(writer, contentType, includedNames);
    } catch (IOException e) {
      // This actually never happens since StringWriter::write() doesn't throw any IOException
      throw new RuntimeException(e);
    }
    return writer.toString();
  }

  @Override
  public void scrape(final @Nonnull Writer writer,
                     final @Nonnull String contentType,
                     final Set<String> includedNames) throws IOException {
    final var agg = new ArrayList<Enumeration<Collector.MetricFamilySamples>>(this.registries.size());
    for (final PrometheusMeterRegistry meterRegistry : this.registries.values()) {
      agg.add(metricsFamilySamples(includedNames, meterRegistry));
    }
    TextFormat.writeFormat(contentType, writer, new MetricsFamilySamplesAggregator(agg));
  }

  private Enumeration<Collector.MetricFamilySamples> metricsFamilySamples(final Set<String> includedNames,
                                                                          final PrometheusMeterRegistry promRegistry) {
    if (includedNames != null) {
      return promRegistry.getPrometheusRegistry().filteredMetricFamilySamples(includedNames);
    }
    return promRegistry.getPrometheusRegistry().metricFamilySamples();
  }

  private static final class MetricsFamilySamplesAggregator implements Enumeration<Collector.MetricFamilySamples> {

    private final List<Enumeration<Collector.MetricFamilySamples>> samples;

    private MetricsFamilySamplesAggregator(final List<Enumeration<Collector.MetricFamilySamples>> samples) {
      this.samples = samples;
    }

    @Override
    public boolean hasMoreElements() {
      for (Enumeration<Collector.MetricFamilySamples> sample : samples) {
        if (sample.hasMoreElements()) {
          return true;
        }
      }
      return false;
    }

    @Override
    public Collector.MetricFamilySamples nextElement() {
      for (final Enumeration<Collector.MetricFamilySamples> sample : samples) {
        if (sample.hasMoreElements()) {
          return sample.nextElement();
        }
      }
      throw new NoSuchElementException();
    }
  }
}
