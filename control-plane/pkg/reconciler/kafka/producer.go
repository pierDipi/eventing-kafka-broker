/*
 * Copyright 2020 The Knative Authors
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

package kafka

import (
	"fmt"
	"time"

	"github.com/Shopify/sarama"
	"github.com/golang/protobuf/proto"

	"knative.dev/eventing-kafka-broker/control-plane/pkg/contract"
)

type NewProducerFunc func(addrs []string, config *sarama.Config) (sarama.SyncProducer, error)

// GetProducer creates a new producer.
func GetProducer(producerFunc NewProducerFunc, bootstrapServers []string) (sarama.SyncProducer, func() error, error) {
	return GetProducerFromConfig(producerFunc, Config(), bootstrapServers)
}

// GetProducerFromConfig creates a new producer.
func GetProducerFromConfig(producerFunc NewProducerFunc, config *sarama.Config, servers []string) (sarama.SyncProducer, func() error, error) {

	p, err := producerFunc(servers, config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create producer: %w", err)
	}

	return p, func() error { return p.Close() }, err
}

// Send sends r using the given producer to the given topic.
func Send(p sarama.SyncProducer, topic, uid string, r *contract.Object) error {

	var encoder sarama.Encoder = nil
	if r != nil {
		value, err := proto.Marshal(r)
		if err != nil {
			return fmt.Errorf("failed to marshal resource: %w", err)
		}

		encoder = sarama.ByteEncoder(value)
	}

	_, _, err := p.SendMessage(&sarama.ProducerMessage{
		Topic:     topic,
		Key:       sarama.StringEncoder(uid),
		Value:     encoder,
		Timestamp: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("failed to send message to topic %s: %w", topic, err)
	}

	return nil
}

func (f NewProducerFunc) Add(bootstrapServers []string, topic, uid string, r *contract.Object) error {
	return f.send(bootstrapServers, topic, uid, r)
}

func (f NewProducerFunc) Remove(s []string, topic, uid string) interface{} {
	return f.send(s, topic, uid, nil)
}

func (f NewProducerFunc) send(bootstrapServers []string, topic, uid string, r *contract.Object) error {
	p, closeFunc, err := GetProducer(f, bootstrapServers)
	if err != nil {
		return err
	}
	defer closeFunc()

	return Send(p, topic, uid, r)
}
