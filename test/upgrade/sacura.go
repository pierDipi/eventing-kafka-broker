package upgrade

import (
	"context"
	"errors"
	"fmt"
	"sync"

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/system"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
)

type step struct {
	t *Test

	muErr sync.Mutex
	err   error

	muJobs sync.Mutex
	jobs   []types.NamespacedName
}

var _ Listener = &step{}

func (s *step) OnPreStep(ctx context.Context, _ int, v Version) error {
	s.muErr.Lock()
	err := s.err
	s.muErr.Unlock()

	if err != nil {
		return s.err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				err := ctx.Err()
				if !errors.Is(err, context.Canceled) {
					s.setError(err)
					return
				}

				// TODO Check Last job
			default:
				// TODO Check last job

				s.muErr.Lock()
				err := s.err
				s.muErr.Unlock()

				if err != nil {
					return
				}

				job := types.NamespacedName{
					Name:      feature.MakeRandomK8sName(v.Name),
					Namespace: s.t.JobNamespace,
				}

				d := duckv1.Destination{

				}

				ns, err := kubeclient.Get(ctx).CoreV1().Namespaces().Get(ctx, system.Namespace(), metav1.GetOptions{})
				if err != nil {
					s.setError(v.errorOnContext(fmt.Errorf("failed to get system namespace %s: %w", system.Namespace(), err)))
					return
				}

				r := resolver.NewURIResolver(ctx, func(name types.NamespacedName) {})
				u, err := r.URIFromDestinationV1(ctx, d, ns)
				if err != nil {
					s.setError(v.errorOnContext(fmt.Errorf("failed to get Ingress URI: %w", err)))
					return
				}

				for _, m := range s.t.Sacura {

					p, err := manifest.ParseTemplates(m, nil, map[string]interface{}{
						"target": u.String(),
					})
					if err != nil {
						s.setError(v.errorOnContext(fmt.Errorf("failed to parse teplate %s: %w", p, err)))
						return
					}

				}

				// TODO find the function to do it

				kc := kubeclient.Get(ctx)

				j := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      job.Name,
						Namespace: job.Namespace,
					},
					Spec: batchv1.JobSpec{

					},
				}

				_, err = kc.BatchV1().Jobs(j.Namespace).Create(ctx, j, metav1.CreateOptions{})
				if err != nil && !errors.Is(err, context.Canceled) && !apierrors.IsNotFound(err) {
					s.setError(v.errorOnContext(fmt.Errorf("failed to create job %s/%s: %w", job.Namespace, job.Name, err)))
					return
				}

				s.muJobs.Lock()
				s.jobs = append(s.jobs, job)
				s.muJobs.Unlock()
			}
		}
	}()

	return nil
}

func (s *step) OnPostStep(_ context.Context, _ int, _ Version) error {
	s.muErr.Lock()
	defer s.muErr.Unlock()

	return s.err
}

func (s *step) OnPreRun(_ context.Context) error {
	s.muErr.Lock()
	defer s.muErr.Unlock()

	return s.err
}

func (s *step) OnPostRun(_ context.Context) error {
	s.muErr.Lock()
	defer s.muErr.Unlock()

	return s.err
}

func (s *step) setError(err error) {
	s.muErr.Lock()
	defer s.muErr.Unlock()

	if s.err != nil { // Keep the first error.
		s.err = err
	}
}
