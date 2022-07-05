/*
 * Copyright 2021 The Knative Authors
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
	"context"
	"fmt"
	"strings"

	"github.com/Shopify/sarama"
	"knative.dev/eventing-kafka/pkg/common/kafka/offset"
)

// InitOffsetsFunc initialize offsets for a provided set of topics and a provided consumer group id.
type InitOffsetsFunc func(ctx context.Context, kafkaClient sarama.Client, kafkaAdminClient sarama.ClusterAdmin, topics []string, consumerGroup string) (int32, error)

var (
	_ InitOffsetsFunc = offset.InitOffsets
)

type Group struct {
	ID           string
	ResourceUUID string
}

type ActiveConsumer struct {
	Topics     map[string][]int32
	ClientId   string
	ClientHost string
}

func (ac ActiveConsumer) String() string {
	return fmt.Sprintf("clientID: %s, clientHost: %s, topicPartitions: %+v", ac.ClientId, ac.ClientHost, ac.Topics)
}

type ActiveConsumers []ActiveConsumer

func (acs ActiveConsumers) String() string {
	var sb strings.Builder
	for _, ac := range acs {
		sb.WriteRune('[')
		sb.WriteString(ac.String())
		sb.WriteRune(']')
		sb.WriteRune(' ')
	}
	return sb.String()
}

func GetActiveConsumers(admin sarama.ClusterAdmin, groups []Group) (ActiveConsumers, error) {
	groupIDs := make([]string, 0, len(groups))
	for _, g := range groups {
		groupIDs = append(groupIDs, g.ID)
	}
	gds, err := admin.DescribeConsumerGroups(groupIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to describe consumer groups %+v: %w", groups, err)
	}
	acs := make([]ActiveConsumer, 0, len(gds))
	for _, gd := range gds {
		if ic := knativeInternalClients(groups, gd.Members); len(ic) > 0 {
			if err := activeConsumers(ic, acs); err != nil {
				return nil, err
			}
		}
	}
	return acs, nil
}

func activeConsumers(g map[string]*sarama.GroupMemberDescription, acs []ActiveConsumer) error {
	for k, v := range g {
		ass, err := v.GetMemberAssignment()
		if err != nil {
			return fmt.Errorf("failed to decode member assignment for %s: %w", k, err)
		}
		acs = append(acs, ActiveConsumer{
			Topics:     ass.Topics,
			ClientId:   v.ClientId,
			ClientHost: v.ClientHost,
		})
	}
	return nil
}

func knativeInternalClients(groups []Group, members map[string]*sarama.GroupMemberDescription) map[string]*sarama.GroupMemberDescription {
	if len(members) == 0 {
		return nil
	}
	ic := make(map[string]*sarama.GroupMemberDescription, len(members))
	for _, g := range groups {
		for k, v := range members {
			if strings.Contains(v.ClientId, g.ResourceUUID) {
				ic[k] = v
			}
		}
	}
	return ic
}
