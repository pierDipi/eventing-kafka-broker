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
import dev.knative.eventing.kafka.broker.core.cluster.MockCluster;
import org.junit.jupiter.api.Test;

import java.util.Arrays;
import java.util.Collections;
import java.util.Set;

import static dev.knative.eventing.kafka.broker.core.testing.CoreObjects.egress1;
import static dev.knative.eventing.kafka.broker.core.testing.CoreObjects.egress2;
import static dev.knative.eventing.kafka.broker.core.testing.CoreObjects.egress3;
import static dev.knative.eventing.kafka.broker.core.testing.CoreObjects.resource1;
import static dev.knative.eventing.kafka.broker.core.testing.CoreObjects.resource2;
import static org.assertj.core.api.Assertions.assertThat;

public class ClusteredEgressReconcilerFilterTest {

  @Test
  public void shouldPassAllEgressesWhenThereIs1Instance() {
    final var cluster = new MockCluster(Set.of("1"), "1");
    final var next = new MockResourceReconciler();
    final var reconciler = new ClusteredEgressReconcilerFilter(cluster, next);

    final var resources = Arrays.asList(resource1(), resource2());
    reconciler.reconcile(resources);

    assertThat(next.getReconciledResources()).isEqualTo(Collections.singletonList(resources));
  }

  @Test
  public void shouldSplitEgressesWhenThereAre2InstancesAndBeingFirst() {
    final var cluster = new MockCluster(Set.of("1", "2"), "1");
    final var next = new MockResourceReconciler();
    final var reconciler = new ClusteredEgressReconcilerFilter(cluster, next);

    final var resourceToSplit = DataPlaneContract.Resource.newBuilder()
      .setUid("abcd-1234")
      .addEgresses(egress1())
      .addEgresses(egress2())
      .addEgresses(egress3())
      .build();

    final var resources = Arrays.asList(resource1(), resourceToSplit, resource2());
    reconciler.reconcile(resources);

    final var expectedResourceToSplit =
      DataPlaneContract.Resource.newBuilder()
        .setUid("abcd-1234")
        .addEgresses(egress1())
        .addEgresses(egress2())
        .build();

    assertThat(next.getReconciledResources())
      .isEqualTo(Collections.singletonList(Arrays.asList(resource1(), expectedResourceToSplit)));
  }

  @Test
  public void shouldSplitEgressesWhenThereAre2InstancesAndBeingSecond() {
    final var cluster = new MockCluster(Set.of("1", "2"), "2");
    final var next = new MockResourceReconciler();
    final var reconciler = new ClusteredEgressReconcilerFilter(cluster, next);

    final var resourceToSplit = DataPlaneContract.Resource.newBuilder()
      .setUid("abcd-1234")
      .addEgresses(egress1())
      .addEgresses(egress2())
      .addEgresses(egress3())
      .build();

    final var resources = Arrays.asList(resource1(), resourceToSplit, resource2());
    reconciler.reconcile(resources);

    final var expectedResourceToSplit =
      DataPlaneContract.Resource.newBuilder()
        .setUid("abcd-1234")
        .addEgresses(egress3())
        .build();

    assertThat(next.getReconciledResources())
      .isEqualTo(Collections.singletonList(Arrays.asList(expectedResourceToSplit, resource2())));
  }
}
