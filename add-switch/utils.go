package addswitch

import (
	moduleregistry "config_service/pkg/module-registry"
	sw "config_service/pkg/store-wrapper"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// get modules from module registry (from etcd)
func GetModulesNames() {
	//getAllKeysInStore(client, "yang-modules")

	client, err := sw.GetEtcdClient()
	if err != nil {
		log.Fatal("Failed to create etcd client:", err)
	}
	defer client.Close()

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
		//fmt.Printf("Inspecting Key: %s\n", key)

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
		fmt.Printf("Module: %s, Structures: %s\n", key, value)
	}

}

func MapConfigStructures(mr *moduleregistry.ModuleRegistry) {
	// open file, read it, store in a map
	// compare the files from the map to the module registry
	// if match is found take the structure value and add it
	// to the map of files as a string

	// Open the JSON file
	data, err := os.ReadFile("add-switch/uploads/proba.json")
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}

	// Declare a variable to hold the unmarshalled data
	var result map[string]interface{}
	switchModel := make(map[string]string)

	// Unmarshal the JSON data into the map
	err = json.Unmarshal(data, &result)
	if err != nil {
		log.Fatalf("Error unmarshalling JSON: %v", err)
	}

	// Print the result
	//fmt.Println(result)

	// Create store prexies from file names and revisions
	for key := range result {
		parts := strings.Split(key, "@")
		switchModel[parts[0]] = ""
		//fmt.Println("Key: ", parts[0])

		// Check if the key exists in the second map
		if innerMap, exists := mr.Modules[parts[0]]; exists {
			if structureValue, found := innerMap["structure"]; found {
				// Replace the value in the first map with the value from the second map
				switchModel[parts[0]] = structureValue
				//fmt.Println("value: ", switchModel[parts[0]])
				fmt.Printf("Match found! %s: %s\n", parts[0], switchModel[parts[0]])
			}
		} else {
			fmt.Println("No match found!")
		}
	}
}

func GetEtcdMap(client *clientv3.Client, mapName string) (map[string]string, error) {
	// The map is stored as a set of key-value pairs in etcd, identified by the map name
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get all the key-value pairs associated with the mapName prefix
	resp, err := client.Get(ctx, mapName, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	// Convert the result into a map[string]string
	result := make(map[string]string)
	for _, kv := range resp.Kvs {
		// Store each key-value pair into the result map (removing the mapName prefix from the key)
		result[string(kv.Key[len(mapName):])] = string(kv.Value)
	}

	return result, nil
}
