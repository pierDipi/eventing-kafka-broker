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

package dev.knative.eventing.kafka.broker.dispatcher.impl.consumer;

import io.cloudevents.CloudEvent;
import io.cloudevents.core.message.impl.GenericStructuredMessageReader;
import io.cloudevents.core.message.impl.MessageUtils;
import io.cloudevents.kafka.impl.KafkaBinaryMessageReaderImpl;
import io.cloudevents.kafka.impl.KafkaHeaders;
import org.apache.kafka.common.header.Headers;
import org.apache.kafka.common.serialization.Deserializer;

public class CloudEventDeserializer implements Deserializer<CloudEvent> {

  @Override
  public CloudEvent deserialize(final String topic, final byte[] data) {
    throw new UnsupportedOperationException("CloudEventDeserializer supports only the signature deserialize(String, Headers, byte[])");
  }

  /**
   * Deserialize a record value from a byte array into a CloudEvent.
   *
   * This is an adapted form of io.cloudevents.kafka.CloudEventDeserializer to handle invalid CloudEvents in a different
   * way.
   * In particular
   *
   * @param topic   topic associated with the data
   * @param headers headers associated with the record; may be empty.
   * @param data    serialized bytes; may be null; implementations are recommended to handle null by returning a value or null rather than throwing an exception.
   * @return deserialized typed data; may be null
   */
  @Override
  public CloudEvent deserialize(final String topic, final Headers headers, byte[] data) {

    String ctHeader = null;
    final var specVersionHeader = KafkaHeaders.getParsedKafkaHeader(headers, KafkaHeaders.SPEC_VERSION);
    if (specVersionHeader == null) {
      ctHeader = KafkaHeaders.getParsedKafkaHeader(headers, KafkaHeaders.CONTENT_TYPE);
    }
    if (ctHeader == null && specVersionHeader == null) {
      // Record is not in binary nor structured format.
      return new InvalidCloudEvent(data);
    }

    final var contentTypeHeader = ctHeader; // Make content type header final.

    final var reader = MessageUtils.parseStructuredOrBinaryMessage(
      () -> contentTypeHeader,
      format -> new GenericStructuredMessageReader(format, data),
      () -> specVersionHeader,
      sv -> new KafkaBinaryMessageReaderImpl(sv, headers, data)
    );

    return reader.toEvent();
  }

}
