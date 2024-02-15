package clientpool

import (
	"sync"

	"github.com/IBM/sarama"
	corev1 "k8s.io/api/core/v1"
)

// client is a proxy object for sarama.Client
//
// It keeps track of callers that are actively using the client using incrementCallers() and Close().
type client struct {
	client sarama.Client

	isFatalError func(err error) bool
	onFatalError func(err error)

	callersWg sync.WaitGroup

	secret    *corev1.Secret
}

var _ sarama.Client = &client{}

func (c *client) Config() *sarama.Config {
	return c.client.Config()
}

func (c *client) Controller() (*sarama.Broker, error) {
	x, err := c.client.Controller()
	if c.isFatalError(err) {
		c.onFatalError(err)
	}
	return x, err
}

func (c *client) RefreshController() (*sarama.Broker, error) {
	x, err := c.client.RefreshController()
	if c.isFatalError(err) {
		c.onFatalError(err)
	}
	return x, err
}

func (c *client) Brokers() []*sarama.Broker {
	return c.client.Brokers()
}

func (c *client) Broker(brokerID int32) (*sarama.Broker, error) {
	x, err := c.client.Broker(brokerID)
	if c.isFatalError(err) {
		c.onFatalError(err)
	}
	return x, err
}

func (c *client) Topics() ([]string, error) {
	x, err := c.client.Topics()
	if c.isFatalError(err) {
		c.onFatalError(err)
	}
	return x, err
}

func (c *client) Partitions(topic string) ([]int32, error) {
	x, err := c.client.Partitions(topic)
	if c.isFatalError(err) {
		c.onFatalError(err)
	}
	return x, err
}

func (c *client) WritablePartitions(topic string) ([]int32, error) {
	x, err := c.client.WritablePartitions(topic)
	if c.isFatalError(err) {
		c.onFatalError(err)
	}
	return x, err
}

func (c *client) Leader(topic string, partitionID int32) (*sarama.Broker, error) {
	x, err := c.client.Leader(topic, partitionID)
	if c.isFatalError(err) {
		c.onFatalError(err)
	}
	return x, err
}

func (c *client) LeaderAndEpoch(topic string, partitionID int32) (*sarama.Broker, int32, error) {
	x, y, err := c.client.LeaderAndEpoch(topic, partitionID)
	if c.isFatalError(err) {
		c.onFatalError(err)
	}
	return x, y, err
}

func (c *client) Replicas(topic string, partitionID int32) ([]int32, error) {
	x, err := c.client.Replicas(topic, partitionID)
	if c.isFatalError(err) {
		c.onFatalError(err)
	}
	return x, err
}

func (c *client) InSyncReplicas(topic string, partitionID int32) ([]int32, error) {
	x, err := c.client.InSyncReplicas(topic, partitionID)
	if c.isFatalError(err) {
		c.onFatalError(err)
	}
	return x, err
}

func (c *client) OfflineReplicas(topic string, partitionID int32) ([]int32, error) {
	x, err := c.client.OfflineReplicas(topic, partitionID)
	if c.isFatalError(err) {
		c.onFatalError(err)
	}
	return x, err
}

func (c *client) RefreshBrokers(addrs []string) error {
	err := c.client.RefreshBrokers(addrs)
	if c.isFatalError(err) {
		c.onFatalError(err)
	}
	return err
}

func (c *client) RefreshMetadata(topics ...string) error {
	err := c.client.RefreshMetadata(topics...)
	if c.isFatalError(err) {
		c.onFatalError(err)
	}
	return err
}

func (c *client) GetOffset(topic string, partitionID int32, time int64) (int64, error) {
	x, err := c.client.GetOffset(topic, partitionID, time)
	if c.isFatalError(err) {
		c.onFatalError(err)
	}
	return x, err
}

func (c *client) Coordinator(consumerGroup string) (*sarama.Broker, error) {
	x, err := c.client.Coordinator(consumerGroup)
	if c.isFatalError(err) {
		c.onFatalError(err)
	}
	return x, err
}

func (c *client) RefreshCoordinator(consumerGroup string) error {
	err := c.client.RefreshCoordinator(consumerGroup)
	if c.isFatalError(err) {
		c.onFatalError(err)
	}
	return err
}

func (c *client) TransactionCoordinator(transactionID string) (*sarama.Broker, error) {
	x, err := c.client.TransactionCoordinator(transactionID)
	if c.isFatalError(err) {
		c.onFatalError(err)
	}
	return x, err
}

func (c *client) RefreshTransactionCoordinator(transactionID string) error {
	err := c.client.RefreshTransactionCoordinator(transactionID)
	if c.isFatalError(err) {
		c.onFatalError(err)
	}
	return err
}

func (c *client) InitProducerID() (*sarama.InitProducerIDResponse, error) {
	x, err := c.client.InitProducerID()
	if c.isFatalError(err) {
		c.onFatalError(err)
	}
	return x, err
}

func (c *client) LeastLoadedBroker() *sarama.Broker {
	return c.client.LeastLoadedBroker()
}

func (c *client) Close() error {
	c.callersWg.Done()
	return nil
}

func (c *client) Closed() bool {
	return c.client.Closed()
}

func (c *client) incrementCallers() {
	c.callersWg.Add(1)
}
