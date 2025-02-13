package storewrapper

import (
	"fmt"
	"log"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type EtcdClient struct {
	client *clientv3.Client
	mu     sync.Mutex
}

var instance *EtcdClient
var once sync.Once

// GetInstance returns the singleton instance of the EtcdClient
func GetInstance() *EtcdClient {
	once.Do(func() {
		instance = &EtcdClient{}
	})
	return instance
}

// Initialize initializes the etcd client connection.
func (e *EtcdClient) Initialize() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.client != nil {
		// If the client is already initialized, return
		return nil
	}

	// Initialize a new client
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	})

	if err != nil {
		return fmt.Errorf("failed to connect to etcd: %v", err)
	}

	e.client = client
	log.Println("etcd client initialized successfully.")
	return nil
}

// GetClient returns the etcd client instance
func (e *EtcdClient) GetClient() *clientv3.Client {
	return e.client
}

// Close closes the client connection
func (e *EtcdClient) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.client != nil {
		e.client.Close()
		e.client = nil
		log.Println("etcd client closed.")
	}
}
