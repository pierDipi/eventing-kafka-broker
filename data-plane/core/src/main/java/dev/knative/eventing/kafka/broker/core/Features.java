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

package dev.knative.eventing.kafka.broker.core;

import java.io.FileInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.nio.file.Path;

public class Features {

  public static final String CONSUMER_PARTITIONING = "consumer-partitioning";

  public static final String FEATURES_CONFIG_PATH = System.getenv("FEATURES_CONFIG_PATH");

  private static final String ENABLED = "enabled".toLowerCase();

  public static boolean isConsumerPartitioningEnabled() throws IOException {
    return featureState(Features.CONSUMER_PARTITIONING).equalsIgnoreCase(ENABLED);
  }

  private static String featureState(final String name) throws IOException {
    try (final var f = new FileInputStream(pathOf(FEATURES_CONFIG_PATH, name).toFile().toString())) {
      return trim(f).toLowerCase();
    }
  }

  public static Path pathOf(final String root, final String key) {
    if (root.endsWith("/")) {
      return Path.of(root + key);
    }
    return Path.of(root + "/" + key);
  }

  public static String trim(InputStream in) throws IOException {
    return new String(in.readAllBytes()).trim();
  }
}
