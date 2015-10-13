package waiter

import (
	"fmt"
	"time"

	"github.com/hashicorp/consul/api"
)

type awaitValue struct {
	Key   string
	Value string
}

func newValueAwaiter(key, value string) *awaitValue {
	return &awaitValue{
		Key:   key,
		Value: value,
	}
}

func (w *awaitValue) wait(client *api.Client, timeout time.Duration) error {
	kv := client.KV()

	startTime := time.Now()

	for time.Now().Sub(startTime) < timeout {
		p, _, err := kv.Get(w.Key, nil)
		if err != nil {
			return err
		}

		if p != nil && string(p.Value) == w.Value {
			return nil
		}
	}

	return fmt.Errorf("Timeout expired waiting for '%s'='%s'", w.Key, w.Value)
}
