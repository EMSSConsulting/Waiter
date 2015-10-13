# Waiter [![GoDoc](https://godoc.org/github.com/EMSSConsulting/Waiter?status.png)](https://godoc.org/github.com/EMSSConsulting/Waiter) [![Build Status](https://travis-ci.org/EMSSConsulting/Waiter.svg)](https://travis-ci.org/EMSSConsulting/Waiter)
**Blocking wait until all semantics for Consul**

Waiter was designed to address a specific requirement of
[Depro](https://github.com/EMSSConsulting/Depro), namely the ability to wait until
all required nodes had completed an operation before continuing. Waiter achieves
this using a watched tree structure in Consul's key value store - in which each
node publishes their state using a unique key to identify themselves.

This results in a structure somewhat like the one described below.

```
+ <prefix>
  - <node1.id>=<node1.state>
  - <node2.id>=<node2.state>
```

## Method of Operation
Waiter will begin watching the `<prefix>` node for any changes, specifically to
its child nodes which will be monitored to determine when new nodes appear, old
ones are removed and the state of an available node changes.

This information is then used to determine which nodes are in a ready state as
well as whether enough nodes have transitioned into a ready state.

## Using Waiter
```go
package consumer

import (
    "fmt"
    "os"
    "github.com/hashicorp/consul/api"
    "github.com/EMSSConsulting/waiter"
)

func isNodeReady(w *waiter.WaitNode) bool {
    return true
}

func getAPIClient() *api.Client {
    apiConfig := api.DefaultConfig()

	apiConfig.Address = "127.0.0.1:8500"
	apiConfig.WaitTime = 10 * time.Second

	client, _ := api.NewClient(apiConfig)

    return client
}

func main() {
    prefix := "waiter/prefix"
    minimumNodes := 1
    waitTimeout := 10 * time.Second

    // Instantiate a new Consul API client
    client := getAPIClient()

    // Setup the waiter with its prefix, minimum number of nodes and the node ready function
    wait := waiter.NewWaiter(client, prefix, minimumNodes, isNodeReady)

    // Block until all the nodes are ready or the wait timeout has expired
    allReady, err := wait.Wait(waitTimeout)

    if err != nil {
        fmt.Printf(err.Error())
        os.Exit(3)
    }

    if !allReady {
        os.Exit(1)
    }

    os.Exit(0)
}
```

## Customer
The waiter package also provides a customer object which makes registering a wait
straightforward. It publishes state changes using a go channel and is intended to
be used in conjunction with the Session object - which ensures that the registered
waits are removed when your application is closed.

### Using Customer
```go
package consumer

import (
    "fmt"
    "os"
    "time"
    "github.com/hashicorp/consul/api"
    "github.com/EMSSConsulting/waiter"
)

func getAPIClient() *api.Client {
    apiConfig := api.DefaultConfig()

	apiConfig.Address = "127.0.0.1:8500"
	apiConfig.WaitTime = 10 * time.Second

	client, _ := api.NewClient(apiConfig)

    return client
}

func main() {
    prefix := "waiter/prefix"
    name := "node1"

    // Instantiate Consul API Client
    client := getAPIClient()

    // Create a session, will remove all customer keys when the application closes
    customerSession, err := waiter.NewSession(client, name)

    // Ensure that we close the session when the application exits
    defer customerSession.Close()

    if err != nil {
        fmt.Printf("Failed to create customer session: %s\n", err)
        os.Exit(3)
    }

    // Create a channel which will receive the new states
    customerState := make(chan string)

    // Create the customer with its prefix, name and state channel
    customer := waiter.NewCustomer(client, prefix, name, customerState)

    // Run the customer asynchronously to process state changes
    go func() {
        err := customer.Run(customerSession)
        if err != nil {
            fmt.Printf("Failed to register customer wait entry: %s\n", err)
            os.Exit(3)
        }
    }()

    // Publish some state changes
    customerState <- "busy"
    time.Sleep(10 * time.Second)
    customerState <- "ready"
}
```
