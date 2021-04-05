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

import java.util.HashSet;
import java.util.Set;

public class MockCluster implements Cluster {

  private final Set<String> nodes;

  private final String me;

  public MockCluster(Set<String> nodes, String me) {
    this.nodes = new HashSet<>(nodes);
    this.nodes.add(me);
    this.me = me;
  }

  @Override
  public String me() {
    return me;
  }

  @Override
  public Set<String> nodes() {
    return nodes;
  }

  @Override
  public void nodeListener(ClusterChangedListener clusterChangedListener) {
  }
}
