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

package autoscaler

import "knative.dev/eventing-kafka-broker/control-plane/pkg/contract"

type Merger interface {
	Merge(ct1 *contract.Contract, ct2 *contract.Contract) (out *contract.Contract, err error)
}

type MergerFunc func(ct1 *contract.Contract, ct2 *contract.Contract) (out *contract.Contract, err error)

func (m MergerFunc) Merge(ct1 *contract.Contract, ct2 *contract.Contract) (*contract.Contract, error) {
	return m(ct1, ct2)
}
