/*
 * Copyright © 2018 Knative Authors (knative-dev@googlegroups.com)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package dev.knative.eventing.kafka.broker.core.cluster;

import dev.knative.eventing.kafka.broker.core.Features;
import io.vertx.core.Vertx;
import io.vertx.core.dns.DnsClientOptions;

import java.io.FileInputStream;
import java.io.IOException;
import java.util.function.Function;

import static dev.knative.eventing.kafka.broker.core.Features.pathOf;
import static dev.knative.eventing.kafka.broker.core.Features.trim;

public class ClusterFactory {

  static Function<Vertx, Cluster> from(final String dispatcherClusterConfigPath) throws IOException {
    if (!Features.isConsumerPartitioningEnabled()) {
      return null;
    }

    return vertx -> unchecked(() -> new DNSBasedCluster(
      vertx,
      DNSBasedClusterConfigParser.fromDir(dispatcherClusterConfigPath)
    ));
  }

  static class DNSBasedClusterConfigParser {

    static DNSBasedClusterOptions fromDir(final String dir) throws IOException {

      var dnsRecord = "";
      try (final var f = new FileInputStream(pathOf(dir, "dns.record").toFile().toString())) {
        dnsRecord = trim(f);
      }

      var pollMs = -1;
      try (final var f = new FileInputStream(pathOf(dir, "dns.poll.ms").toFile().toString())) {
        pollMs = Integer.parseInt(trim(f));
      }

      return new DNSBasedClusterOptions(
        new DnsClientOptions(),
        System.getenv("IP_ADDRESS"),
        dnsRecord,
        pollMs
      );
    }
  }

  @FunctionalInterface
  interface SupplierUnchecked<T> {

    default T get() {
      try {
        return getThrows();
      } catch (Exception ex) {
        throw new RuntimeException(ex);
      }
    }

    T getThrows() throws Exception;
  }

  private static <T> T unchecked(SupplierUnchecked<T> supplier) {
    try {
      return supplier.get();
    } catch (Throwable cause) {
      throw new RuntimeException(cause);
    }
  }
}
