package upgrade

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/k8s"
)

type Topology []string

func (t Topology) apply(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		if err := apply(t...); err != nil {
			return fmt.Errorf("failed to apply topology %v: %w", t, err)
		}
		return nil
	}
}

type Test struct {
	Ingress  duckv1.KReference `json:"ingressRef"`
	Topology Topology          `json:"topology"`
	Versions []Version         `json:"versions"`
	Sacura   []string          `json:"sacura"`

	JobConfig    duckv1.KReference `json:"jobConfigRef"`
	JobNamespace string            `json:"jobNamespace"`
}

func (t *Test) Validate() error {
	if len(t.Versions) == 0 {
		return apis.ErrInvalidValue(t.Versions, "steps")
	}
	if len(t.Sacura) == 0 {
		return apis.ErrInvalidValue(t.Sacura, "sacura")
	}
	if len(t.Topology) == 0 {
		return apis.ErrInvalidValue(t.Topology, "topology")
	}
	return nil
}

func (t *Test) Run(ctx context.Context) {

}

type Version struct {
	Name      string   `json:"name"`
	Version   string   `json:"version"`
	Manifests []string `json:"manifests"`
}

func (v *Version) apply(ctx context.Context) error {

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:

		header(log.Printf, v)

		err := apply(v.Manifests...)
		if err != nil {
			return v.errorOnContext(errors.New("failed to apply manifests"))
		}

		err = objectsReady(ctx, dynamicclient.Get(ctx), v.Manifests)
		if err != nil {
			return v.errorOnContext(err)
		}
	}

	return nil
}

func (v *Version) errorOnContext(err error) error {
	return fmt.Errorf("failed: Name: %s - Version: %s - Manifests %v: %w", v.Name, v.Version, v.Manifests, err)
}

type Listener interface {
	OnPreRun(ctx context.Context) error
	OnPreStep(ctx context.Context, uid int, v Version) error
	OnPostStep(ctx context.Context, uid int, v Version) error
	OnPostRun(ctx context.Context) error
}

func Run(t *Test, hooks Listener) error {

	if hooks == nil {
		hooks = noopListener{}
	}

	if err := t.Validate(); err != nil {
		return fmt.Errorf("t is not valid: %w", err)
	}

	// environment.InitFlags registers state and level filter flags.
	environment.InitFlags(flag.CommandLine)
	// We get a chance to parse flags to include the framework flags for the
	// framework as well as any additional flags included in the integration.
	flag.Parse()

	// EnableInjectionOrDie will enable client injection, this is used by the
	// testing framework for namespace management, and could be leveraged by
	// features to pull Kubernetes clients or the test environment out of the
	// context passed in the features.
	ctx, startInformers := injection.EnableInjectionOrDie(nil, nil)
	startInformers()

	// global is used to make instances of Environments, NewGlobalEnvironment
	// is passing and saving the client injection enabled context for use later.
	global := environment.NewGlobalEnvironment(ctx)

	ctx, _ = global.Environment()

	// Run pre-run hook so that people can add their custom logic.
	if err := hooks.OnPreRun(ctx); err != nil {
		return err
	}

	// Apply the first version
	if err := t.Versions[0].apply(ctx); err != nil {
		return t.Versions[0].errorOnContext(err)
	}

	// Apply COs topology (Sources, Channels, Brokers, Knative Services, etc).
	if err := t.Topology.apply(ctx); err != nil {
		return err
	}

	for i, s := range t.Versions[1:] {

		stepCtx, stepCtxCancel := context.WithCancel(ctx)

		// Run pre-step hook so that people can add their custom logic.
		if err := hooks.OnPreStep(stepCtx, i, s); err != nil {
			return err
		}

		if err := s.apply(stepCtx); err != nil {
			return err
		}

		stepCtxCancel()

		// Run post-step hook so that people can add their custom logic.
		if err := hooks.OnPostStep(ctx, i, s); err != nil {
			return err
		}
	}

	// Run post-run hook so that people can add their custom logic.
	return hooks.OnPostRun(ctx)
}

func objectsReady(ctx context.Context, dc dynamic.Interface, manifests []string) error {

	var namespaces []string
	for _, m := range manifests {
		namespaces = append(namespaces, extractNamespaces(m)...)
	}

	for _, ns := range namespaces {
		ls, err := dc.
			Resource(schema.GroupVersionResource{Group: "", Version: "", Resource: ""}).
			Namespace(ns).
			List(ctx, metav1.ListOptions{})

		if apierrors.IsNotFound(err) {
			// we probably matched a namespace that is not a namespace, continue.
			log.Printf("Failed to list resources in namespace %s: %v\n", ns, err)
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to list resources in namespace %s: %w", ns, err)
		}

		for _, r := range ls.Items {
			ref := corev1.ObjectReference{
				Kind:            r.GetKind(),
				Namespace:       r.GetNamespace(),
				Name:            r.GetName(),
				UID:             r.GetUID(),
				APIVersion:      r.GetAPIVersion(),
				ResourceVersion: r.GetResourceVersion(),
			}
			if err := k8s.WaitForReadyOrDone(ctx, ref, time.Second, 5*time.Minute); err != nil {
				return fmt.Errorf("failed waiting for %v ready or done: %w", ref, err)
			}
		}
	}
	return nil
}

var (
	regex = regexp.MustCompile(`namespace: (?P<last>[a-zA-Z0-9-]+)`)
)

func extractNamespaces(m string) []string {

	matches := regex.FindAllStringSubmatch(m, -1)
	r := make([]string, 0, len(matches))
	for _, match := range matches {
		r = append(r, match[1])
	}

	return r
}

// header prints a message like:
// ============================================================================
//
// Applying v0.19.0 (Latest)
//
// ============================================================================
func header(logf func(format string, args ...interface{}), version *Version) {
	name := ""
	if version.Name != "" {
		name = fmt.Sprintf("(%s)", version.Name)
	}

	logf(`
============================================================================

Applying %s %s

============================================================================
`, version.Version, name)
}

func apply(manifests ...string) error {
	for _, m := range manifests {
		if err := exec.Command(kubectl(), "apply", "-f", m).Run(); err != nil {
			return err
		}
	}
	return nil
}

func kubectl() string {
	if cmd, ok := os.LookupEnv("KUBECTL"); ok { // Allow use other flavors, like oc.
		return cmd
	}
	return "kubectl"
}

type noopListener struct {
}

func (i noopListener) OnPreRun(ctx context.Context) error {
	return nil
}

func (i noopListener) OnPreStep(ctx context.Context, uid int, v Version) error {
	return nil
}

func (i noopListener) OnPostStep(ctx context.Context, uid int, v Version) error {
	return nil
}

func (i noopListener) OnPostRun(ctx context.Context) error {
	return nil
}

var _ Listener = noopListener{}
