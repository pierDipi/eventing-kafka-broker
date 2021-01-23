// +build e2e deletecm

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

package e2e

import (
	"context"
	"log"
	"os"
	"testing"

	"k8s.io/client-go/kubernetes"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/test/zipkin"
)

const (
	SystemNamespace = "knative-eventing"
	LogsDir         = "knative-eventing-logs"
)

func TestMain(t *testing.M) {

	ctx := context.Background()

	// Create a K8s client.
	cfg, err := injection.GetRESTConfig(injection.Flags().ServerURL, injection.Flags().Kubeconfig)
	if err != nil {
		panic(err)
	}
	kc, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}

	if err := zipkin.SetupZipkinTracingFromConfigTracing(ctx, kc, log.Printf, SystemNamespace); err != nil {
		panic(err)
	}
	defer zipkin.CleanupZipkinTracingSetup(log.Printf)

	exit := t.Run()

	testlib.ExportLogs(LogsDir, SystemNamespace)

	os.Exit(exit)
}
