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

package config

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/resolver"

	"knative.dev/eventing-kafka-broker/control-plane/pkg/contract"
)

const (
	NoEgress = NoResource
)

// FindEgress finds the egress with the given UID in the given egresses list.
func FindEgress(egresses []*contract.Egress, egress types.UID) int {

	for i, t := range egresses {
		if t.Uid == string(egress) {
			return i
		}
	}

	return NoEgress
}

func EgressConfigFromDelivery(ctx context.Context, resolver *resolver.URIResolver, delivery *eventingduck.DeliverySpec, defaultBackoffDelay uint64, path string, parent interface{}) (*contract.EgressConfig, error) {
	if delivery == nil {
		return nil, nil
	}

	var egressConfig *contract.EgressConfig

	if delivery.DeadLetterSink != nil {

		deadLetterSinkURL, err := resolver.URIFromDestinationV1(ctx, *delivery.DeadLetterSink, parent)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve %s.DeadLetterSink: %w", path, err)
		}

		egressConfig = ensureNonNilEgressConfig(egressConfig)
		egressConfig.DeadLetter = deadLetterSinkURL.String()
	}

	if delivery.Retry != nil {
		egressConfig = ensureNonNilEgressConfig(egressConfig)
		egressConfig.Retry = uint32(*delivery.Retry)
		var err error
		delay, err := BackoffDelayFromISO8601String(delivery.BackoffDelay, defaultBackoffDelay)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s.BackoffDelay: %w", path, err)
		}
		egressConfig.BackoffDelay = delay
		egressConfig.BackoffPolicy = BackoffPolicyFromString(delivery.BackoffPolicy)
	}

	return egressConfig, nil
}

func ensureNonNilEgressConfig(config *contract.EgressConfig) *contract.EgressConfig {
	if config == nil {
		config = &contract.EgressConfig{}
	}
	return config
}
