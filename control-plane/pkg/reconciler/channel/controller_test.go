package channel

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	_ "knative.dev/eventing-kafka/pkg/client/injection/informers/messaging/v1beta1/kafkachannel/fake"
	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/trigger/fake"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/pod/fake"
	"knative.dev/pkg/configmap"
	dynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"
	reconcilertesting "knative.dev/pkg/reconciler/testing"

	"knative.dev/eventing-kafka-broker/control-plane/pkg/config"
)

func TestNewController(t *testing.T) {
	ctx, _ := reconcilertesting.SetupFakeContext(t)

	configs := &config.Env{
		SystemNamespace:      "cm",
		GeneralConfigMapName: "cm",
	}

	ctx, _ = fakekubeclient.With(
		ctx,
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configs.GeneralConfigMapName,
				Namespace: configs.SystemNamespace,
			},
		},
	)
	dynamicScheme := runtime.NewScheme()
	_ = fakekubeclientset.AddToScheme(dynamicScheme)

	dynamicclient.With(ctx, dynamicScheme)

	controller := NewController(
		ctx,
		configmap.NewStaticWatcher(&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: cmConfig,
			},
		}),
		configs,
	)
	if controller == nil {
		t.Error("failed to create controller: <nil>")
	}
}
