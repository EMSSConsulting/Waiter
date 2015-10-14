package waiter

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/api"
)

// Customer provides a means to acquire a wait lock which will be awaited by a listening
// waiter.
type Customer struct {
	Prefix string
	Name   string
	State  <-chan string

	client  *api.Client
	session *Session
	running bool
}

// NewCustomer configures a new wait customer instance correctly, preparing it to run when
// Run() is called.
func NewCustomer(client *api.Client, prefix, name string, state <-chan string) *Customer {
	cust := &Customer{
		Prefix: prefix,
		Name:   name,
		client: client,
		State:  state,
	}

	return cust
}

// Run starts a message pump which will update the customer's wait key whenever
// the state changes, stopping when the state channel is closed.
func (c *Customer) Run(session *Session) error {
	c.running = true
	defer func() { c.running = false }()
	c.session = session

	sessionID := ""
	if c.session != nil {
		sessionID = c.session.ID
	}

	kv := c.client.KV()

	if c.session != nil {
		acquired, _, err := kv.Acquire(&api.KVPair{
			Key:     c.fullKey(),
			Value:   []byte{},
			Session: sessionID,
		}, nil)

		if !acquired && err == nil {
			err = fmt.Errorf("Could not acquire lock on '%s'", c.fullKey())
		}

		if err != nil {
			return err
		}
	}

	for state := range c.State {
		_, err := kv.Put(&api.KVPair{
			Key:     c.fullKey(),
			Value:   []byte(state),
			Session: sessionID,
		}, nil)

		if err != nil {
			return err
		}
	}

	return nil
}

// Remove will remove this customer's entry from Consul. This action can only
// be taken when the customer message pump is not running, you can stop an
// active message pump by closing its state channel.
func (c *Customer) Remove() error {
	if c.running {
		return fmt.Errorf("Cannot remove a customer which is currently running, please close the state channel first.")
	}

	kv := c.client.KV()

	_, err := kv.Delete(c.fullKey(), nil)
	return err
}

func (c *Customer) fullKey() string {
	return fmt.Sprintf("%s/%s", strings.Trim(c.Prefix, "/"), strings.Trim(c.Name, "/"))
}
