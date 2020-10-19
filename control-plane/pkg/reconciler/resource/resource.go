package resource

import (
	"context"
	"fmt"
	"math"

	"go.uber.org/zap"
	"k8s.io/client-go/util/retry"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/reconciler"

	"knative.dev/eventing-kafka-broker/control-plane/pkg/config"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/contract"
	coreconfig "knative.dev/eventing-kafka-broker/control-plane/pkg/core/config"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/log"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/reconciler/base"
	"knative.dev/eventing-kafka-broker/control-plane/pkg/reconciler/kafka"
)

type TopicConfigProvider func(logger *zap.Logger, obj base.Object) (*kafka.TopicConfig, error)

type Creator func(ctx context.Context, topic string, obj base.Object, config *kafka.TopicConfig) (*contract.Resource, error)

type Reconciler struct {
	*base.Reconciler

	TopicConfigProvider TopicConfigProvider
	TopicPrefix         string

	ResourceCreator Creator

	// ClusterAdmin creates new sarama ClusterAdmin. It's convenient to add this as Reconciler field so that we can
	// mock the function used during the reconciliation loop.
	ClusterAdmin kafka.NewClusterAdminFunc

	Configs *config.Env
}

func (r *Reconciler) Reconcile(ctx context.Context, obj base.Object, setAddress func(u *apis.URL)) reconciler.Event {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.reconcile(ctx, obj, setAddress)
	})
}

func (r *Reconciler) reconcile(ctx context.Context, obj base.Object, setAddress func(u *apis.URL)) reconciler.Event {

	logger := log.Logger(ctx, "reconcile", obj)

	statusConditionManager := base.StatusConditionManager{
		Object:     obj,
		SetAddress: setAddress,
		Configs:    r.Configs,
		Recorder:   controller.GetEventRecorder(ctx),
	}

	if !r.IsReceiverRunning() || !r.IsDispatcherRunning() {
		return statusConditionManager.DataPlaneNotAvailable()
	}
	statusConditionManager.DataPlaneAvailable()

	topicConfig, err := r.TopicConfigProvider(logger, obj)
	if err != nil {
		return statusConditionManager.FailedToResolveConfig(err)
	}
	statusConditionManager.ConfigResolved()

	logger.Debug("config resolved", zap.Any("config", topicConfig))

	topic, err := r.ClusterAdmin.CreateTopic(logger, kafka.Topic(r.TopicPrefix, obj), topicConfig)
	if err != nil {
		return statusConditionManager.FailedToCreateTopic(topic, err)
	}
	statusConditionManager.TopicReady(topic)

	logger.Debug("Topic created", zap.Any("topic", topic))

	// Get contract config map.
	contractConfigMap, err := r.GetOrCreateDataPlaneConfigMap(ctx)
	if err != nil {
		return statusConditionManager.FailedToGetConfigMap(err)
	}

	logger.Debug("Got contract config map")

	// Get contract data.
	ct, err := r.GetDataPlaneConfigMapData(logger, contractConfigMap)
	if err != nil && ct == nil {
		return statusConditionManager.FailedToGetDataFromConfigMap(err)
	}

	logger.Debug(
		"Got contract data from config map",
		zap.Any(base.ContractLogKey, (*log.ContractMarshaller)(ct)),
	)

	// Get resource configuration.
	brokerResource, err := r.ResourceCreator(ctx, topic, obj, topicConfig)
	if err != nil {
		return statusConditionManager.FailedToGetConfig(err)
	}

	brokerIndex := coreconfig.FindResource(ct, obj.GetUID())
	// Update contract data with the new contract configuration
	coreconfig.AddOrUpdateResourceConfig(ct, brokerResource, brokerIndex, logger)

	// Increment volumeGeneration
	ct.Generation = incrementContractGeneration(ct.Generation)

	// Update the configuration map with the new contract data.
	if err := r.UpdateDataPlaneConfigMap(ctx, ct, contractConfigMap); err != nil {
		logger.Error("failed to update data plane config map", zap.Error(
			statusConditionManager.FailedToUpdateConfigMap(err),
		))
		return err
	}
	statusConditionManager.ConfigMapUpdated()

	logger.Debug("Contract config map updated")

	// After #37 we reject events to a non-existing Broker, which means that we cannot consider a Broker Ready if all
	// receivers haven't got the Broker, so update failures to receiver pods is a hard failure.
	// On the other side, dispatcher pods care about Triggers, and the Broker object is used as a configuration
	// prototype for all associated Triggers, so we consider that it's fine on the dispatcher side to receive eventually
	// the update even if here eventually means seconds or minutes after the actual update.

	// Update volume generation annotation of receiver pods
	if err := r.UpdateReceiverPodsAnnotation(ctx, logger, ct.Generation); err != nil {
		logger.Error("Failed to update receiver pod annotation", zap.Error(
			statusConditionManager.FailedToUpdateReceiverPodsAnnotation(err),
		))
		return err
	}

	logger.Debug("Updated receiver pod annotation")

	// Update volume generation annotation of dispatcher pods
	if err := r.UpdateDispatcherPodsAnnotation(ctx, logger, ct.Generation); err != nil {
		// Failing to update dispatcher pods annotation leads to config map refresh delayed by several seconds.
		// Since the dispatcher side is the consumer side, we don't lose availability, and we can consider the Broker
		// ready. So, log out the error and move on to the next step.
		logger.Warn(
			"Failed to update dispatcher pod annotation to trigger an immediate config map refresh",
			zap.Error(err),
		)

		statusConditionManager.FailedToUpdateDispatcherPodsAnnotation(err)
	} else {
		logger.Debug("Updated dispatcher pod annotation")
	}

	return statusConditionManager.Reconciled()
}

func (r *Reconciler) Finalize(ctx context.Context, obj base.Object) reconciler.Event {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.finalize(ctx, obj)
	})
}

func (r *Reconciler) finalize(ctx context.Context, obj base.Object) reconciler.Event {

	logger := log.Logger(ctx, "finalize", obj)

	// Get contract config map.
	contractConfigMap, err := r.GetOrCreateDataPlaneConfigMap(ctx)
	if err != nil {
		return fmt.Errorf("failed to get contract config map %s: %w", r.Configs.DataPlaneConfigMapAsString(), err)
	}

	logger.Debug("Got contract config map")

	// Get contract data.
	ct, err := r.GetDataPlaneConfigMapData(logger, contractConfigMap)
	if err != nil {
		return fmt.Errorf("failed to get contract: %w", err)
	}

	logger.Debug(
		"Got contract data from config map",
		zap.Any(base.ContractLogKey, (*log.ContractMarshaller)(ct)),
	)

	brokerIndex := coreconfig.FindResource(ct, obj.GetUID())
	if brokerIndex != coreconfig.NoResource {
		coreconfig.DeleteResource(ct, brokerIndex)

		logger.Debug("Broker deleted", zap.Int("index", brokerIndex))

		// Update the configuration map with the new contract data.
		if err := r.UpdateDataPlaneConfigMap(ctx, ct, contractConfigMap); err != nil {
			return err
		}

		logger.Debug("Contract config map updated")

		// There is no need to update volume generation and dispatcher pod annotation, updates to the config map will
		// eventually be seen by the dispatcher pod and resources will be deleted accordingly.
	}

	topicConfig, err := r.TopicConfigProvider(logger, obj)
	if err != nil {
		return fmt.Errorf("failed to resolve broker config: %w", err)
	}

	bootstrapServers := topicConfig.BootstrapServers
	topic, err := r.ClusterAdmin.DeleteTopic(kafka.Topic(r.TopicPrefix, obj), bootstrapServers)
	if err != nil {
		return err
	}

	logger.Debug("Topic deleted", zap.String("topic", topic))

	return nil
}

func incrementContractGeneration(generation uint64) uint64 {
	return (generation + 1) % (math.MaxUint64 - 1)
}
