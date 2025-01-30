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

*/

package storewrapper

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func CreateClient() {
	// Create the etcd client
	client, err := clientv3.New(clientv3.Config{
		//for local connection:
		Endpoints: []string{"http://127.0.0.1:2379"},

		//for Kubernetes
		//Endpoints: []string{"http://etcd-service.default.svc.cluster.local:2379"},

		DialTimeout: 10 * time.Second,
	})

	if err != nil {
		log.Fatal("Failed to create etcd client:", err)
	}
	defer client.Close()

	//getAllKeysInStore(client, "yang-modules")

	// Read data from etcd under the prefix "yang-modules/"
	resp, err := client.Get(context.Background(), "yang-modules/", clientv3.WithPrefix())
	if err != nil {
		log.Fatal(err)
	}

	// Define a map to store the extracted data
	dataMap := make(map[string]string)

	// Regular expression to match keys like "yang-modules/{key}/structure"
	re := regexp.MustCompile(`yang-modules/([^/]+)/structure`)

	// Iterate through the keys returned from etcd and find all matches
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		value := string(kv.Value)

		// Log the key to see if it's in the expected format
		fmt.Printf("Inspecting Key: %s\n", key)

		// Use the regex to extract {key} from the full key path
		matches := re.FindStringSubmatch(key)
		if len(matches) > 1 {
			// The first capturing group contains the {key}
			extractedKey := matches[1]

			dataMap[extractedKey] = value

			// Print the match
			//fmt.Printf("Match Found: Key: %s, Structure Value: %s\n", extractedKey, value)
		} else {
			// If no match, print a message
			//fmt.Println("No match found for key:", key)
		}
	}

	// Output the map to see the result
	fmt.Println("Extracted Data:")
	for key, value := range dataMap {
		fmt.Printf("Key: %s, Structures Value: %s\n", key, value)
	}

}

// getAllKeysInStore is a helper function to retrieve all keys in a specific store prefix.
func getAllKeysInStore(client *clientv3.Client, prefix string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		log.Printf("Failed to get keys with prefix %s: %v", prefix, err)
		return
	}
	fmt.Printf("\nKeys in store with prefix %s:\n", prefix)
	for _, kv := range resp.Kvs {
		fmt.Printf("Key: %s, Value: %s\n", kv.Key, kv.Value)

	}
	fmt.Println()
}
