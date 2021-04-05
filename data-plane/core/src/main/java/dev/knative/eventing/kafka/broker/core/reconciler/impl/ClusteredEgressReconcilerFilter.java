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

package dev.knative.eventing.kafka.broker.core.reconciler.impl;

import dev.knative.eventing.kafka.broker.contract.DataPlaneContract;
import dev.knative.eventing.kafka.broker.core.cluster.Cluster;
import dev.knative.eventing.kafka.broker.core.reconciler.ResourcesReconciler;
import io.vertx.core.Future;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.List;

import static dev.knative.eventing.kafka.broker.core.utils.Logging.keyValue;

/**
 * {@link ClusteredEgressReconcilerFilter} filter resources to reconcile based on the available cluster information.
 */
public class ClusteredEgressReconcilerFilter implements ResourcesReconciler {

  private static final Logger logger = LoggerFactory.getLogger(ClusteredEgressReconcilerFilter.class);

  private final Cluster cluster;
  private final ResourcesReconciler next;

  public ClusteredEgressReconcilerFilter(final Cluster cluster, final ResourcesReconciler next) {
    this.cluster = cluster;
    this.next = next;
  }

  @Override
  public Future<Void> reconcile(Collection<DataPlaneContract.Resource> resources) {
    return next.reconcile(egressesConsideringCluster(resources, cluster));
  }

  private static Collection<DataPlaneContract.Resource> egressesConsideringCluster(
    final Collection<DataPlaneContract.Resource> resources,
    final Cluster cluster) {

    final var nodesSet = cluster.nodes();
    final var me = cluster.me();

    nodesSet.add(me);
    final var nodes = nodesSet.toArray(new String[0]);
    Arrays.sort(nodes);

    if (nodes.length <= 1) {
      return resources;
    }

    final var meIdx = Arrays.binarySearch(nodes, me);
    if (meIdx < 0) {
      throw new IllegalStateException(String.format("My node id %s is not present in the cluster node list %s", me, Arrays.toString(nodes)));
    }

    final var egressesCount = resources.stream()
      .mapToInt(DataPlaneContract.Resource::getEgressesCount)
      .sum();

    final var perNodeEgresses = new int[nodes.length];
    for (int i = 0; i < perNodeEgresses.length; i++) {
      perNodeEgresses[i] = (egressesCount / nodes.length) + (i < /* remaining egresses */ (egressesCount % nodes.length) ? 1 : 0);
    }

    logger.info("Per node egresses {} {}", keyValue("nodes", nodes), keyValue("perNodeEgresses", perNodeEgresses));

    int i = 0;
    final List<DataPlaneContract.Resource> newResources = new ArrayList<>(perNodeEgresses[meIdx]);

    for (DataPlaneContract.Resource resource : resources) {
      final var egresses = new ArrayList<DataPlaneContract.Egress>(resource.getEgressesCount());
      for (DataPlaneContract.Egress egress : resource.getEgressesList()) {
        if (i == meIdx) {
          logger.info("Keeping egress {} of resource {}",
            keyValue("egressUid", egress.getUid()),
            keyValue("resourceUid", resource.getUid())
          );
          egresses.add(egress);
        }

        perNodeEgresses[i]--;
        if (perNodeEgresses[i] <= 0) {
          i++;
        }
        if (i == meIdx && perNodeEgresses[meIdx] == 0) {
          return newResources;
        }
      }
      if (egresses.size() > 0) {
        final var newResourceBuilder = DataPlaneContract.Resource.newBuilder(resource);
        newResourceBuilder.clearEgresses();
        newResourceBuilder.addAllEgresses(egresses);
        newResources.add(newResourceBuilder.build());
      }
    }

    return newResources;
  }
}
