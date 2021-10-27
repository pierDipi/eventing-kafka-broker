package security

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bindings "knative.dev/eventing-kafka/pkg/apis/bindings/v1beta1"
	secretinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/secret/fake"
	reconcilertesting "knative.dev/pkg/reconciler/testing"
)

func TestNetSpecSecretProviderFunc(t *testing.T) {
	tests := []struct {
		name      string
		secrets   []*corev1.Secret
		namespace string
		netSpec   bindings.KafkaNetSpec
		want      *corev1.Secret
		wantErr   bool
	}{
		{
			name: "SASL_SSL - SCRAM-SHA-512",
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "user"},
					StringData: map[string]string{"key": "key"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "password"},
					StringData: map[string]string{"key": "key"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "type"},
					StringData: map[string]string{"key": "SCRAM-SHA-512"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cert"},
					StringData: map[string]string{"key": "key"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "key"},
					StringData: map[string]string{"key": "key"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cacert"},
					StringData: map[string]string{"key": "key"},
				},
			},
			namespace: "ns",
			netSpec: bindings.KafkaNetSpec{
				SASL: bindings.KafkaSASLSpec{
					Enable: true,
					User: bindings.SecretValueFromSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "user"},
							Key:                  "key",
						},
					},
					Password: bindings.SecretValueFromSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "password"},
							Key:                  "key",
						},
					},
					Type: bindings.SecretValueFromSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "type"},
							Key:                  "key",
						},
					},
				},
				TLS: bindings.KafkaTLSSpec{
					Enable: true,
					Cert: bindings.SecretValueFromSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "cert"},
							Key:                  "key",
						},
					},
					Key: bindings.SecretValueFromSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "key"},
							Key:                  "key",
						},
					},
					CACert: bindings.SecretValueFromSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "cacert"},
							Key:                  "key",
						},
					},
				},
			},
			want: &corev1.Secret{StringData: map[string]string{
				CaCertificateKey: "key",
				UserCertificate:  "key",
				UserKey:          "key",
				SaslMechanismKey: SaslScramSha512,
				SaslUserKey:      "key",
				SaslPasswordKey:  "key",
			}},
		},
		{
			name: "SASL_SSL - SCRAM-SHA-256",
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "user"},
					StringData: map[string]string{"key": "key"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "password"},
					StringData: map[string]string{"key": "key"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "type"},
					StringData: map[string]string{"key": "SCRAM-SHA-256"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cert"},
					StringData: map[string]string{"key": "key"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "key"},
					StringData: map[string]string{"key": "key"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cacert"},
					StringData: map[string]string{"key": "key"},
				},
			},
			namespace: "ns",
			netSpec: bindings.KafkaNetSpec{
				SASL: bindings.KafkaSASLSpec{
					Enable: true,
					User: bindings.SecretValueFromSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "user"},
							Key:                  "key",
						},
					},
					Password: bindings.SecretValueFromSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "password"},
							Key:                  "key",
						},
					},
					Type: bindings.SecretValueFromSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "type"},
							Key:                  "key",
						},
					},
				},
				TLS: bindings.KafkaTLSSpec{
					Enable: true,
					Cert: bindings.SecretValueFromSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "cert"},
							Key:                  "key",
						},
					},
					Key: bindings.SecretValueFromSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "key"},
							Key:                  "key",
						},
					},
					CACert: bindings.SecretValueFromSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "cacert"},
							Key:                  "key",
						},
					},
				},
			},
			want: &corev1.Secret{StringData: map[string]string{
				CaCertificateKey: "key",
				UserCertificate:  "key",
				UserKey:          "key",
				SaslMechanismKey: SaslScramSha256,
				SaslUserKey:      "key",
				SaslPasswordKey:  "key",
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := reconcilertesting.SetupFakeContext(t)
			lister := secretinformer.Get(ctx)
			for _, s := range tt.secrets {
				cp := s.DeepCopy()
				if cp.Data == nil {
					cp.Data = map[string][]byte{}
				}
				for k, v := range cp.StringData {
					cp.Data[k] = []byte(v)
				}
				_ = lister.Informer().GetStore().Add(cp)
			}

			got, err := NetSpecSecretProviderFunc(lister.Lister(), tt.namespace, tt.netSpec)(context.Background(), "", "")
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Error("(-want, +got)", diff)
			}
		})
	}
}
