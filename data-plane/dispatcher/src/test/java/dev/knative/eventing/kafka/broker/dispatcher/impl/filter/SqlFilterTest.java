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
package dev.knative.eventing.kafka.broker.dispatcher.impl.filter;

import static org.assertj.core.api.Assertions.assertThat;

import io.cloudevents.CloudEvent;
import io.cloudevents.core.builder.CloudEventBuilder;

import java.net.URI;
import java.time.OffsetDateTime;
import java.time.ZoneOffset;
import java.util.stream.Stream;

import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

public class SqlFilterTest {
  final static CloudEvent event =
    CloudEventBuilder.v1()
      .withId("123-42")
      .withDataContentType("application/cloudevents+json")
      .withDataSchema(URI.create("/api/schema"))
      .withSource(URI.create("/api/some-source"))
      .withSubject("a-subject-42")
      .withType("type")
      .withTime(OffsetDateTime.of(1985, 4, 12, 23, 20, 50, 0, ZoneOffset.UTC))
      .build();

  @ParameterizedTest
  @MethodSource(value = {"testCases"})
  public void match(CloudEvent event, String expression, boolean shouldMatch) {
    var filter = new SqlFilter(expression);
    assertThat(filter.test(event)).isEqualTo(shouldMatch);
  }

  static Stream<Arguments> testCases() {
    return Stream.of(Arguments.of(event, "'TRUE'", true),
                     Arguments.of(event, "'FALSE'", false),
                     Arguments.of(event, "0", false),
                     Arguments.of(event, "1", false),
                     Arguments.of(event, "id LIKE '123%'", true),
                     Arguments.of(event, "NOT(id LIKE '123%')", false));
  }
}
