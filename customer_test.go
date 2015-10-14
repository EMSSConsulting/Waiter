package waiter

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
)

func getTestAPIClient() *api.Client {
	apiConfig := api.DefaultConfig()

	apiConfig.Address = "127.0.0.1:8500"
	apiConfig.WaitTime = 10 * time.Second

	client, _ := api.NewClient(apiConfig)

	return client
}

func TestCustomer_Normal(t *testing.T) {
	stateCh := make(chan string)
	client := getTestAPIClient()

	cust := NewCustomer(client, "wait/test", "cust1", stateCh)
	sess, err := NewSession(client, "cust1")

	if err != nil {
		t.Fatalf("Failed to create session: %s", err)
		return
	}

	defer sess.Close()

	go func() {
		err := cust.Run(sess)
		if err != nil {
			t.Fatal(err)
		}
	}()

	stateCh <- "busy"

	valueWaiter := newValueAwaiter("wait/test/cust1", "busy")

	err = valueWaiter.wait(client, 5*time.Second)

	if err != nil {
		t.Fatalf("Failed to retrieve value of customer entry: %s", err)
	}

	close(stateCh)
}

func TestCustomer_MultiState(t *testing.T) {
	stateCh := make(chan string)
	client := getTestAPIClient()

	cust := NewCustomer(client, "wait/test", "cust1", stateCh)
	sess, err := NewSession(client, "cust1")

	if err != nil {
		t.Fatalf("Failed to create session: %s", err)
		return
	}

	defer sess.Close()

	go func() {
		err := cust.Run(sess)
		if err != nil {
			t.Fatal(err)
		}
	}()

	stateCh <- "busy"

	valueWaiter := newValueAwaiter("wait/test/cust1", "busy")
	err = valueWaiter.wait(client, 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to retrieve value of customer entry: %s", err)
	}

	stateCh <- "ready"

	valueWaiter = newValueAwaiter("wait/test/cust1", "ready")
	err = valueWaiter.wait(client, 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to retrieve value of customer entry: %s", err)
	}

	close(stateCh)
}

func TestCustomer_Shutdown(t *testing.T) {
	stateCh := make(chan string)
	client := getTestAPIClient()

	cust := NewCustomer(client, "wait/test", "cust1", stateCh)
	sess, err := NewSession(client, "cust1")

	if err != nil {
		t.Fatalf("Failed to create session: %s", err)
		return
	}

	defer sess.Close()

	go func() {
		err := cust.Run(sess)
		if err != nil {
			t.Fatal(err)
		}
	}()

	stateCh <- "busy"

	valueWaiter := newValueAwaiter("wait/test/cust1", "busy")

	err = valueWaiter.wait(client, 5*time.Second)

	if err != nil {
		t.Fatalf("Failed to retrieve value of customer entry: %s", err)
	}

	valueWaiter = newValueAwaiter("wait/test/cust1", "")

	close(stateCh)
	err = valueWaiter.wait(client, 5*time.Second)

	if err != nil {
		t.Fatalf("Expected customer entry to be removed: %s", err)
	}
}
