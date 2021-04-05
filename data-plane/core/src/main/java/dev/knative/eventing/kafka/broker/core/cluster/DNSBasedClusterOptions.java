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

import io.vertx.core.dns.DnsClientOptions;

public class DNSBasedClusterOptions {

  private final DnsClientOptions dnsClientOptions;

  private final String me;

  private final String dnsRecord;

  private final long pollMs;

  public DNSBasedClusterOptions(final DnsClientOptions dnsClientOptions,
                                final String me,
                                final String dnsRecord,
                                long pollMs) {
    if (me == null || me.isBlank()) {
      throw new IllegalArgumentException("me cannot be null or empty");
    }
    if (dnsRecord == null || dnsRecord.isBlank()) {
      throw new IllegalArgumentException("dnsRecord cannot be null or empty");
    }
    if (pollMs <= 0) {
      throw new IllegalArgumentException("pollMs cannot be negative or 0: " + pollMs);
    }
    this.dnsClientOptions = dnsClientOptions;
    this.me = me;
    this.dnsRecord = dnsRecord;
    this.pollMs = pollMs;
  }

  public DnsClientOptions getDnsClientOptions() {
    return dnsClientOptions;
  }

  public String getMe() {
    return me;
  }

  public String getDnsRecord() {
    return dnsRecord;
  }

  public long getPollMs() {
    return pollMs;
  }
}
