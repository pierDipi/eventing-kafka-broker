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

package security

import (
	"context"
	"fmt"

	"github.com/Shopify/sarama"
	corev1 "k8s.io/api/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	bindings "knative.dev/eventing-kafka/pkg/apis/bindings/v1beta1"
)

// NetSpecSecretProviderFunc creates a SecretProviderFunc from a provided bindings.KafkaNetSpec.
//
// The returned SecretProviderFunc creates an in-memory secret with the format expected by the
// NewSaramaSecurityOptionFromSecret function.
func NetSpecSecretProviderFunc(lister corelisters.SecretLister, namespace string, netSpec bindings.KafkaNetSpec) SecretProviderFunc {
	return func(ctx context.Context, _, _ string) (*corev1.Secret, error) {
		saslUser, err := resolveSecret(lister, namespace, netSpec.SASL.User.SecretKeyRef)
		if err != nil {
			return nil, err
		}
		saslPassword, err := resolveSecret(lister, namespace, netSpec.SASL.Password.SecretKeyRef)
		if err != nil {
			return nil, err
		}

		saslType, err := resolveSecret(lister, namespace, netSpec.SASL.Type.SecretKeyRef)
		if err != nil {
			return nil, err
		}

		tlsCert, err := resolveSecret(lister, namespace, netSpec.TLS.Cert.SecretKeyRef)
		if err != nil {
			return nil, err
		}

		tlsKey, err := resolveSecret(lister, namespace, netSpec.TLS.Key.SecretKeyRef)
		if err != nil {
			return nil, err
		}

		tlsCACert, err := resolveSecret(lister, namespace, netSpec.TLS.CACert.SecretKeyRef)
		if err != nil {
			return nil, err
		}

		secretData := make(map[string]string, 10)
		if netSpec.TLS.Enable {
			if tlsCACert != "" {
				secretData[CaCertificateKey] = tlsCACert
			}
			if tlsKey != "" && tlsCert != "" {
				secretData[UserKey] = tlsKey
				secretData[UserCertificate] = tlsCert
			}
		}
		if netSpec.SASL.Enable {
			secretData[SaslMechanismKey] = SaslPlain
			if saslType == sarama.SASLTypeSCRAMSHA256 {
				secretData[SaslMechanismKey] = SaslScramSha256
			}
			if saslType == sarama.SASLTypeSCRAMSHA512 {
				secretData[SaslMechanismKey] = SaslScramSha512
			}
			secretData[SaslUserKey] = saslUser
			secretData[SaslPasswordKey] = saslPassword
		}
		return &corev1.Secret{StringData: secretData}, nil
	}
}

// resolveSecret resolves the secret reference
func resolveSecret(lister corelisters.SecretLister, ns string, ref *corev1.SecretKeySelector) (string, error) {
	if ref == nil || ref.Name == "" {
		return "", nil
	}
	secret, err := lister.Secrets(ns).Get(ref.Name)
	if err != nil {
		return "", fmt.Errorf("failed to read secret: %w", err)
	}

	value, ok := secret.Data[ref.Key]
	if !ok || len(value) == 0 {
		return "", fmt.Errorf("missing secret key or empty secret value (%s/%s)", ref.Name, ref.Key)
	}
	return string(value), nil
}
