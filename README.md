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

func main() {
    prefix := "waiter/prefix"
    minimumNodes := 1
    waitTimeout := 10 * time.Second

    apiConfig := api.DefaultConfig()

	apiConfig.Address = "127.0.0.1:8500"
	apiConfig.WaitTime = 10 * time.Second

	client, _ := api.NewClient(apiConfig)

    wait := waiter.NewWait(client, prefix, minimumNodes, nil)

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
