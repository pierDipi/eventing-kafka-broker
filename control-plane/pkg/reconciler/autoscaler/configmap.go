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

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"google.golang.org/protobuf/proto"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/retry"
	"knative.dev/pkg/reconciler"

	"knative.dev/eventing-kafka-broker/control-plane/pkg/contract"
	coreconfig "knative.dev/eventing-kafka-broker/control-plane/pkg/core/config"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/reconciler/base"
)

var (
	CMLabelsSelector         = labels.SelectorFromSet(map[string]string{}) // TODO add label
	DispatcherLabelsSelector = labels.SelectorFromSet(map[string]string{}) // TODO add label

	UnbalancedError = errors.New("ConfigMaps and Deployments are unbalanced")
)

// TODO add RBAC

type CMReconciler struct {
	// MinThreshold is the minimum number of bytes of a ConfigMap that we don't want to aggregate.
	// The content of every ConfigMap with a size less than MinThreshold, will be remapped to another existing
	// ConfigMap, iff a CM with a size less than MinThreshold plus a candidate CM size, which is the smallest existing
	// CM, don't exceed MaxThreshold.
	MinThreshold int
	// MaxThreshold is the maximum number of bytes of a ConfigMap that we don't want to split.
	// The content of every ConfigMap with a size greater than MaxThreshold, will be partially rescheduled to a newly
	// created CM, such that the work is split by 2.
	//
	// What "the work is split by 2" means might vary and is an implementation detail.
	// It can be based on the number of objects, some Kafka specific metrics.
	MaxThreshold int

	Splitter Splitter
	Merger   Merger

	// Only one loop at a time.
	mu sync.Mutex

	// Format represents the format of every ConfigMap managed by the reconciler.
	Format         string
	DispatcherName string

	CMLister         corelisters.ConfigMapLister
	DeploymentLister appslisters.DeploymentLister
	KubeClient       kubernetes.Interface

	// Enqueue ConfigMap function.
	Enqueue func(cm *corev1.ConfigMap)
}

func (r *CMReconciler) ReconcileKind(ctx context.Context, cm *corev1.ConfigMap) reconciler.Event {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, ok := cm.Data[base.ConfigMapDataKey]
	if !ok {
		return nil
	}

	l := len([]byte(data))

	if l < r.MinThreshold {
		return r.scaleDown(ctx, cm)
	}
	if l > r.MaxThreshold {
		return r.scaleUp(ctx, cm)
	}

	return nil
}

func (r *CMReconciler) scaleUp(ctx context.Context, cm *corev1.ConfigMap) reconciler.Event {
	if err := r.ensureBalanced(ctx, cm.Namespace); err != nil {
		if errors.Is(err, UnbalancedError) {
			r.Enqueue(cm)
			return nil
		}
		return err
	}

	cmCt, err := getContract(cm, r.Format)
	if err != nil {
		return err
	}

	srcCt, dstCtNew := r.Splitter.Split(cmCt)
	ordinal, nextCMName := nextConfigMapName(cm.Name)

	nextConfigMap, err := r.KubeClient.CoreV1().ConfigMaps(cm.Namespace).Get(ctx, nextCMName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// TODO just create	cm, update old and create deployment
		}
	}

	srcCm, err := marshal(r.Format, srcCt, cm)
	if err != nil {
		return err
	}

	dstCtOriginal := &contract.Contract{}
	if err := base.Unmarshal(r.Format, nextConfigMap.BinaryData[base.ConfigMapDataKey], dstCtOriginal); err != nil {
		return fmt.Errorf("failed to unmarshal ConfigMap %s/%s: %w", nextConfigMap.Namespace, nextConfigMap.Name, err)
	}

	dstCt, err := r.Merger.Merge(dstCtOriginal, dstCtNew)
	if err != nil {
		return fmt.Errorf("unable to merge original contract (ConfigMap %s/%s) and new contract: %w", nextConfigMap.Namespace, nextConfigMap.Name, err)
	}

	nextConfigMap, err = marshal(r.Format, dstCt, nextConfigMap)
	if err != nil {
		return err
	}

	d, err := r.newDispatcherDeployment(generateNameFuncFromOrdinal(ordinal), cm.Namespace)
	if err != nil {
		return err
	}

	// The order of the operations is not casual.
	// Creating a new ConfigMap and updating another one is not an atomic operation. For that reason, we first create
	// a new ConfigMap (dst) and then update the src ConfigMap, so that we don't remove objects from a ConfigMap and
	// then fail to add them to the other one, which might lead to a data loss.
	//
	// 1. Create dstCm -> Make sure we don't lose data
	// 2. Update srcCm -> Cms are balanced
	// 3. Create deployment associated with dstCm ->

	dstCmCreated, err := r.KubeClient.CoreV1().ConfigMaps(cm.Namespace).Create(ctx, nextConfigMap, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create ConfigMap %s/%s: %w", nextConfigMap.Namespace, nextConfigMap.Name, err)
	}
	d.OwnerReferences = append(d.OwnerReferences, *metav1.NewControllerRef(dstCmCreated, dstCmCreated.GroupVersionKind()))

	_, err = r.KubeClient.CoreV1().ConfigMaps(cm.Namespace).Update(ctx, srcCm, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ConfigMap %s/%s: %w", srcCm.Namespace, srcCm.Name, err)
	}

	_, err = r.KubeClient.AppsV1().Deployments(cm.Namespace).Create(ctx, d, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("failed to create Deployment %s/%s: %w", d.Namespace, d.Name, err)
	}

	return nil
}

func (r *CMReconciler) scaleDown(ctx context.Context, cm *corev1.ConfigMap) reconciler.Event {
	if err := r.ensureBalanced(ctx, cm.Namespace); err != nil {
		if errors.Is(err, UnbalancedError) {
			r.Enqueue(cm)
			return nil
		}
		return err
	}

	// TODO only -1
	cms, err := r.CMLister.ConfigMaps(cm.Namespace).List(CMLabelsSelector)
	if err != nil {
		return fmt.Errorf("failed to list ConfigMaps: %w", err)
	}

	sort.Slice(cms, func(i, j int) bool {
		return cms[i].Name < cms[j].Name
	})

	cmData := cm.BinaryData[base.ConfigMapDataKey]
	var candidate *corev1.ConfigMap
	for _, other := range cms {

		if other.Name == cm.Name || other.Namespace != cm.Namespace {
			continue
		}

		data, ok := other.BinaryData[base.ConfigMapDataKey]
		if !ok {
			return r.copy(ctx, cm, other)
		}
		dl := data

		if r.MaxThreshold < len(dl)+len(cmData) {
			// r.MaxThreshold < len(dl)+len(cmData)	doesn't mean that the serialized version of dl + cmData will be less
			// than MaxThreshold.
			// Moreover, we're using a cached version of data that might have changed to reduce API server calls.
			// Eventually everything will be anyway rebalanced.
			if candidate == nil {
				candidate = other
			} else if len(cmData) > len(dl) {
				candidate = other
			}
		}
	}
	if candidate == nil {
		return nil // We can't scale down.
	}
	if err := r.copy(ctx, cm, candidate); err != nil {
		return err
	}
	return r.cleanUp(ctx, cm)
}

func (r *CMReconciler) ensureBalanced(ctx context.Context, ns string) error {
	ds, err := r.DeploymentLister.Deployments(ns).List(DispatcherLabelsSelector)
	if err != nil {
		return fmt.Errorf("failed to list Deployments in namespace %s: %w", ns, err)
	}
	cms, err := r.CMLister.ConfigMaps(ns).List(CMLabelsSelector)
	if err != nil {
		return fmt.Errorf("failed to list ConfigMaps in namespace %s: %w", ns, err)
	}
	if len(cms) == len(ds) {
		return nil
	}

	for _, cm := range cms {
		cmOrdinal := ordinal(cm.Name)
		var deployment *appsv1.Deployment
		for _, d := range ds {
			if cmOrdinal == ordinal(d.Name) {
				deployment = d
				break
			}
		}
		if deployment == nil {

			// TODO a CM delete should trigger a global re-sync in the Trigger reconciler because the failure
			// 	could happen after the destination ConfigMap update.

			foreground := metav1.DeletePropagationForeground
			err := r.KubeClient.CoreV1().ConfigMaps(ns).Delete(ctx, cm.Name, metav1.DeleteOptions{
				Preconditions:     &metav1.Preconditions{UID: &cm.UID},
				PropagationPolicy: &foreground,
			})
			if err != nil {
				return fmt.Errorf("failed to delete ConfigMap %s/%s: %w", cm.Namespace, cm.Name, err)
			}
		}
	}

	return nil
}

func (r *CMReconciler) copy(ctx context.Context, src *corev1.ConfigMap, dst *corev1.ConfigMap) reconciler.Event {

	srcCt, err := getContract(src, r.Format)
	if err != nil {
		return err
	}

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		dstGot, err := r.KubeClient.CoreV1().ConfigMaps(dst.Namespace).Get(ctx, dst.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get ConfigMap %s/%s: %w", dst.Namespace, dst.Name, err)
		}
		dst = dstGot // Do not modify informer copy.

		dstCt, err := getContract(dstGot, r.Format)
		if err != nil {
			return err
		}

		if err = copyContract(srcCt, dstCt); err != nil {
			return err
		}

		dstCt.Generation = coreconfig.IncrementGeneration(dstCt.Generation)

		return base.UpdateConfigMap(ctx, dstCt, dstGot, r.KubeClient, r.Format)
	})
}

func (r *CMReconciler) cleanUp(ctx context.Context, cm *corev1.ConfigMap) reconciler.Event {

	// TODO implement this by just deleting cm and associated deployment

	return nil
}

// generateNameFunc is function that given an object name, returns a new name.
type generateNameFunc func(name string) string

func (r *CMReconciler) newDispatcherDeployment(generateNameFunc generateNameFunc, namespace string) (*appsv1.Deployment, error) {

	prototypeDeployment, err := r.DeploymentLister.Deployments(namespace).Get(r.DispatcherName)
	if err != nil {
		return nil, fmt.Errorf("failed to get Deployment %s/%s: %w", namespace, r.DispatcherName, err)
	}

	// Do not modify informer copy.
	d := &appsv1.Deployment{}
	prototypeDeployment.DeepCopyInto(d)

	// TODO change labels

	d.Name = generateNameFunc(prototypeDeployment.Name)
	d.OwnerReferences = append(d.OwnerReferences, *metav1.NewControllerRef(prototypeDeployment, prototypeDeployment.GroupVersionKind()))

	return d, nil
}

// TODO put in coreconfig
func splitApart(cmCt *contract.Contract) (*contract.Contract, *contract.Contract) {
	srcCt := &contract.Contract{Generation: coreconfig.IncrementGeneration(cmCt.Generation)}
	dstCt := &contract.Contract{}
	for _, r := range cmCt.Resources {

		egresses := r.Egresses
		r.Egresses = nil

		srcR := proto.Clone(r).(*contract.Resource)
		dstR := proto.Clone(r).(*contract.Resource)

		// For simplicity, we can start by just dividing egresses by 2 (src and dst).
		n := len(egresses) / 2

		srcR.Egresses = egresses[len(egresses)-n:]
		dstR.Egresses = egresses[:len(egresses)-n]

		srcCt.Resources = append(srcCt.Resources, srcR)
		dstCt.Resources = append(dstCt.Resources, dstR)
	}
	return srcCt, dstCt
}

// TODO put in coreconfig (this is sort of merge)
func copyContract(src *contract.Contract, dst *contract.Contract) error {

	for _, r := range src.Resources {
		rIdx := coreconfig.FindResource(dst, types.UID(r.Uid))
		if rIdx != coreconfig.NoResource {
			// TODO this is the problem with ingress in all CMs, we might want to rewrite the address
			if err := copyResource(r, dst.Resources[rIdx]); err != nil {
				return err
			}
		} else {
			dst.Resources = append(dst.Resources, r)
		}
	}

	return nil
}

// TODO put in coreconfig
func copyResource(src *contract.Resource, dst *contract.Resource) error {

	for _, e := range src.Egresses {
		eIdx := coreconfig.FindEgress(dst.Egresses, types.UID(e.Uid))
		if eIdx == coreconfig.NoEgress {
			dst.Egresses = append(dst.Egresses, e)
		}
	}

	return nil
}

// TODO put in coreconfig
func getContract(dstGot *corev1.ConfigMap, format string) (*contract.Contract, error) {
	ct := &contract.Contract{}
	if err := base.Unmarshal(format, dstGot.BinaryData[base.ConfigMapDataKey], ct); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ConfigMap %s/%s: %w", dstGot.Namespace, dstGot.Name, err)
	}
	return ct, nil
}

func nextConfigMapName(name string) (int, string) {
	lastIndex := strings.LastIndex(name, "-")
	ordinal, err := strconv.Atoi(name[lastIndex+1:])
	if err != nil {
		ordinal = 1
	} else {
		name = name[:lastIndex]
	}
	return ordinal, generateName(name, ordinal)
}

func ordinal(name string) int {
	lastIndex := strings.LastIndex(name, "-")
	if lastIndex < 0 {
		return 0
	}
	ordinal, err := strconv.Atoi(name[lastIndex+1:])
	if err != nil {
		panic(err)
	}
	return ordinal
}

func generateName(name string, ordinal int) string {
	if ordinal == 0 {
		return name
	}
	return fmt.Sprintf("%s-%d", name, ordinal)
}

func generateNameFuncFromOrdinal(ordinal int) generateNameFunc {
	return func(name string) string {
		return generateName(name, ordinal)
	}
}

func marshal(format string, dstCt *contract.Contract, dstCm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	dstCtRaw, err := base.Marshal(format, dstCt)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal dstCt: %w", err)
	}
	return &corev1.ConfigMap{
		ObjectMeta: *dstCm.ObjectMeta.DeepCopy(),
		BinaryData: map[string][]byte{
			base.ConfigMapDataKey: dstCtRaw,
		},
	}, nil
}
