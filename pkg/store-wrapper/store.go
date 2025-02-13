/*
Running etcd in the Background
If you want to run etcd in the background as a service, you can add the & at the end of the command:
etcd --name node1 \
     --data-dir /var/lib/etcd \
     --listen-client-urls http://localhost:2379 \
     --advertise-client-urls http://localhost:2379

To stop etcd:
	sudo systemctl stop etcd

To read using etcdctl:
	etcdctl get "" --prefix
	etcdctl put $KEY $VALUE
	etcdctl del $KEY

*/

package storewrapper

import (
	"context"
	"fmt"
	"log"
	"time"

	"git.cs.kau.se/hamzchah/opencnc_kafka-exporter/logger/pkg/logger"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// CreateEtcdClient creates and returns an etcd client
func CreateEtcdClient() (*clientv3.Client, error) {
	// Initialize the etcd client with provided configuration
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"http://127.0.0.1:2379"}, // List of etcd endpoints
		DialTimeout: 10 * time.Second,                  // Timeout for the dial
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %v", err)
	}

	// Return the created etcd client
	return client, nil
}

// setKey is a helper function to set a key-value pair with a prefix (store).
func SetKey(client *clientv3.Client, key, value string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.Put(ctx, key, value)
	if err != nil {
		log.Printf("Failed to put key %s: %v", key, err)
	} else {
		//fmt.Printf("Set key %s with value %s\n", key, value)
	}
}

// GetKey is a helper function to get the value for a key from a store.
func GetKey(client *clientv3.Client, key string) (string, error) {
	var log = logger.GetLogger()

	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get the key-value pair from the etcd store
	resp, err := client.Get(ctx, key)
	if err != nil {
		// Log the error if the key retrieval fails
		log.Infof("Failed to get key %s: %v", key, err)
		return "", err
	}

	// Check if the key exists in the response
	if len(resp.Kvs) == 0 {
		// If no key is found, return an empty string and an error
		return "", fmt.Errorf("key %s not found", key)
	}

	// Return the value of the first key found in the response
	return string(resp.Kvs[0].Value), nil
}

// getAllKeysInStore is a helper function to retrieve all keys in a specific store prefix.
func GetAllKeysInStore(client *clientv3.Client, prefix string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := client.Get(ctx, prefix, clientv3.WithPrefix())

	if err != nil {
		log.Printf("Failed to get keys with prefix %s: %v", prefix, err)
		return
	}
	fmt.Printf("\nKeys in store with prefix %s:\n", prefix)

	if len(resp.Kvs) == 0 {
		log.Printf("No keys found with prefix %s", prefix)
		return
	}

	for _, kv := range resp.Kvs {
		fmt.Printf("Key: %s, Value: %s\n", kv.Key, kv.Value)

	}
	fmt.Println()
}
