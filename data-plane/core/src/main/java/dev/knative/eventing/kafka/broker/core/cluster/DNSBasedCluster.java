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

import dev.knative.eventing.kafka.broker.core.utils.CollectionsUtils;
import io.vertx.core.Vertx;
import io.vertx.core.dns.DnsClient;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.HashSet;
import java.util.List;
import java.util.Random;
import java.util.Set;
import java.util.concurrent.atomic.AtomicReference;

import static dev.knative.eventing.kafka.broker.core.utils.Logging.keyValue;

public class DNSBasedCluster implements Cluster {

  private static final Logger logger = LoggerFactory.getLogger(DNSBasedCluster.class);

  private final DnsClient client;
  private final String address;
  private final String me;
  private ClusterChangedListener clusterChangedListener;

  private final AtomicReference<Set<String>> nodes;

  public DNSBasedCluster(final Vertx vertx, final DNSBasedClusterOptions options) {
    this.client = vertx.createDnsClient(options.getDnsClientOptions());
    this.me = options.getMe();
    this.address = options.getDnsRecord();

    nodes = new AtomicReference<>(Set.of(me));

    vertx.setPeriodic(options.getPollMs() + jitterMs(), t -> updateNodes());

    // Update nodes as soon as we instantiate a cluster.
    updateNodes();
  }

  private static long jitterMs() {
    // This prevents that all replicas make a DNS request at the same time.
    return new Random().nextInt(/* at most 1 second */ 1000);
  }

  private void updateNodes() {
    client.resolveA(address)
      .onSuccess(this::newNodes)
      .onFailure(cause -> logger.error("[resolveA] Failed to resolve IP addresses of A record: " + address, cause));
  }

  private void newNodes(final List<String> nodes) {

    final var newNodes = new HashSet<>(nodes);
    final var oldNodes = this.nodes.get();
    final var diff = CollectionsUtils.diff(oldNodes, newNodes);
    if (diff.getAdded().isEmpty() && diff.getRemoved().isEmpty()) {
      return;
    }

    this.nodes.set(newNodes);

    logger.info("Cluster members {} {} {}", keyValue("address", address), keyValue("me", me), keyValue("nodes", newNodes));

    if (clusterChangedListener != null) {
      clusterChangedListener.clusterChanged(newNodes);
    }
  }

  @Override
  public String me() {
    return this.me;
  }

  @Override
  public Set<String> nodes() {
    return this.nodes.get();
  }

  @Override
  public void nodeListener(ClusterChangedListener clusterChangedListener) {
    this.clusterChangedListener = clusterChangedListener;
  }
}
