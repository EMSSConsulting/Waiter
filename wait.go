// Package waiter provides "wait for all" semantics for users of Consul.
// This allows you to wait on numerous asynchronous operations, blocking
// until they have all completed, across multiple different nodes and with
// no knowledge of which nodes will respond beforehand.
package waiter

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
)

// Wait provides a means to wait for multiple nodes to set a key within a prefix
// prior to completing. It can be used in conjunction with sessions to ensure that
// all nodes have finished an operation before continuing.
type Wait struct {
	// The key prefix under which nodes will create their entries.
	Prefix string
	// The minimum number of nodes that are required to write a ready entry before
	// the wait will complete.
	MinimumNodes int
	// A function which determines whether the entry a node has written represents
	// a ready state.
	IsReady func(n *WaitNode) (isReady bool)

	NodeUpdate <-chan WaitNodeUpdate
	NodeReady  <-chan WaitNode
	AllReady   <-chan []WaitNode

	nodeUpdate chan WaitNodeUpdate
	nodeReady  chan WaitNode
	allReady   chan []WaitNode

	client    *api.Client
	kv        *api.KV
	startTime time.Time
}

// WaitNode represents a single node's current state, where the state is determined
// by the value written to the key and node is the name of the key that was written.
type WaitNode struct {
	Node  string
	State string
}

// WaitNodeUpdate represents the change in state of a particular node, affected
// by the changing of the value portion of a key.
type WaitNodeUpdate struct {
	Node      string
	State     string
	LastState string
}

// NewWaiter creates a new Wait entry with a sensible default isReady function if
// nil is provided in its place.
func NewWaiter(client *api.Client, prefix string, minimumNodes int, isReady func(n *WaitNode) bool) *Wait {
	if isReady == nil {
		isReady = func(n *WaitNode) bool {
			return true
		}
	}

	nodeUpdateCh := make(chan WaitNodeUpdate, 2)
	nodeReadyCh := make(chan WaitNode, 2)
	allReadyCh := make(chan []WaitNode, 2)

	return &Wait{
		Prefix:       prefix,
		MinimumNodes: minimumNodes,
		IsReady:      isReady,

		NodeUpdate: nodeUpdateCh,
		NodeReady:  nodeReadyCh,
		AllReady:   allReadyCh,

		nodeUpdate: nodeUpdateCh,
		nodeReady:  nodeReadyCh,
		allReady:   allReadyCh,

		client: client,
		kv:     client.KV(),
	}
}

// Wait begins a blocking wait for which will complete once all of the nodes have
// set their keys to a ready state.
func (w *Wait) Wait(timeout time.Duration) (bool, error) {
	w.startTime = time.Now()

	err := w.createTreeEntry()
	if err != nil {
		return false, err
	}

	var lastWaitIndex uint64
	nodes := map[string]WaitNode{}

	for !w.shouldStop(timeout) {
		newNodes, newWaitIndex, err := w.getNodeList(lastWaitIndex)
		if err != nil {
			return false, err
		}

		lastWaitIndex = newWaitIndex

		// The above command could take some time, so check whether we timed out
		// once it completes.
		if w.shouldStop(timeout) {
			return false, nil
		}

		allReady, nodesCount := w.updateNodes(nodes, newNodes)
		if allReady && nodesCount >= w.MinimumNodes {
			select {
			case w.allReady <- newNodes:
			default:
			}

			return true, nil
		}
	}

	return false, nil
}

func (w *Wait) shouldStop(timeout time.Duration) bool {
	return time.Now().Sub(w.startTime) > timeout
}

func (w *Wait) createTreeEntry() error {
	p := &api.KVPair{
		Key:   fmt.Sprintf("%s/", strings.Trim(w.Prefix, "/")),
		Flags: 0,
		Value: make([]byte, 0),
	}

	_, err := w.kv.Put(p, nil)
	return err
}

func (w *Wait) getNodeList(waitIndex uint64) ([]WaitNode, uint64, error) {
	list, meta, err := w.kv.List(w.Prefix, &api.QueryOptions{
		WaitIndex:  waitIndex,
		AllowStale: true,
	})

	if err != nil {
		return nil, 0, err
	}

	nodes := []WaitNode{}

	for i := 0; i < len(list); i++ {
		nodeName := strings.Trim(list[i].Key[len(w.Prefix):], "/")
		if nodeName != "" {
			nodes = append(nodes, WaitNode{
				Node:  nodeName,
				State: string(list[i].Value),
			})
		}
	}

	return nodes, meta.LastIndex, nil
}

func (w *Wait) updateNodes(nodes map[string]WaitNode, newNodes []WaitNode) (allReady bool, nodeCount int) {
	allReady = true
	nodeCount = len(newNodes)

	for nodeName, node := range nodes {
		contains := false
		for i := 0; i < len(newNodes); i++ {
			if newNodes[i].Node == nodeName {
				contains = true
				break
			}
		}

		if !contains {
			nodeUpdate := WaitNodeUpdate{
				Node:      nodeName,
				State:     "",
				LastState: node.State,
			}
			select {
			case w.nodeUpdate <- nodeUpdate:
			default:
			}
			delete(nodes, nodeName)
		}
	}

	for i := 0; i < len(newNodes); i++ {
		node, exists := nodes[newNodes[i].Node]

		lastState := ""
		hasChanged := false

		if !exists {
			hasChanged = true
			node = newNodes[i]
		}

		if node.State != newNodes[i].State {
			hasChanged = true
			lastState = node.State
			node = newNodes[i]
		}

		if hasChanged {
			nodes[node.Node] = node

			nodeUpdate := WaitNodeUpdate{
				Node:      node.Node,
				State:     node.State,
				LastState: lastState,
			}
			select {
			case w.nodeUpdate <- nodeUpdate:
			default:
			}

			if w.IsReady(&node) {
				select {
				case w.nodeReady <- node:
				default:
				}
			} else {
				allReady = false
			}
		}
	}

	return allReady, nodeCount
}
