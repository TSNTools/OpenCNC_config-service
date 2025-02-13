package storewrapper

import (
	"context"
	"strings"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// WatchKeys watches for changes on keys with a given prefix and calls the callback on new keys
func WatchKeys(prefix string, callback func(outerKey, innerKey, value string), client *clientv3.Client) {
	watcher := clientv3.NewWatcher(client)
	defer watcher.Close()

	// Watch for changes to keys with the provided prefix
	watchChan := watcher.Watch(context.Background(), prefix, clientv3.WithPrefix())

	// Handle incoming watch events
	for resp := range watchChan {
		for _, ev := range resp.Events {
			// Process only EventTypePut (added or updated keys)
			if ev.Type == clientv3.EventTypePut {
				// Split the key into outer and inner parts (this assumes keys are in a "outer/inner" format)
				keyParts := splitKey(string(ev.Kv.Key))
				outerKey := keyParts[0]
				innerKey := keyParts[1]

				// Call the callback function when a new key is added
				callback(outerKey, innerKey, string(ev.Kv.Value))
			}
		}
	}
}

// Helper function to split the key (assuming it's in "outer/inner" format)
func splitKey(key string) []string {
	parts := strings.Split(key, "/") // Split the key on '/'
	if len(parts) < 2 {
		return []string{key} // Return the original key if it does not contain '/'
	}
	return parts
}
