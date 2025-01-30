package main

import (
	//"bufio"

	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type FileInfo struct {
	Directory string
	FileName  string
}

func ModuleRegistry() {
	// Replace with the path to your directory
	dirPath := "../pkg/yang_modules"

	// Read file names from the directory
	files, err := getFilesWithSubdirectories(dirPath)
	if err != nil {
		fmt.Printf("Error reading files: %v\n", err)
		return
	}

	// Set up the etcd client configuration
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"http://127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})

	if err != nil {
		log.Fatal("Failed to create etcd client:", err)
	}
	defer client.Close()

	// Create store prexies from file names and revisions
	prefix0 := "yang-modules/"
	//counter := 1
	for _, file := range files {
		parts := strings.Split(file.FileName, "@")
		//prefix := prefix1 + strconv.Itoa(counter)
		prefix := prefix0 + parts[0]

		//fmt.Println(prefix + "/name" + ": " + parts[0])
		//setKey(client, prefix+"/name", parts[0])
		if len(parts) == 2 {
			//fmt.Println(prefix + "/revision" + ": " + parts[1])
			setKey(client, prefix+"/revision", parts[1])

		} else {
			//fmt.Println(prefix + "/revision: No revision tag.")
			setKey(client, prefix+"/revision", "No Revision tag found.")
		}
		//fmt.Println(prefix + "/structure" + ": " + file.Directory)
		setKey(client, prefix+"/structure", file.Directory)
		//counter++
	}
}

func getFilesWithSubdirectories(dirPath string) ([]FileInfo, error) {
	var filesInfo []FileInfo

	// Walk through the directory and its subdirectories
	err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories but note their names
		if d.IsDir() {
			return nil
		}

		if filepath.Ext(d.Name()) == ".yang" {
			// Get the subdirectory name (relative to the root directory)
			subdirectory := filepath.Base(filepath.Dir(path))

			// Add the file info with the subdirectory name
			filesInfo = append(filesInfo, FileInfo{
				Directory: subdirectory,
				FileName:  strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())),
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return filesInfo, nil
}

// getUserInput reads a string input from the user and returns it
/*func getUserInput(prompt string) string {
	// Create a new reader for user input
	reader := bufio.NewReader(os.Stdin)

	// Display the prompt
	fmt.Print(prompt)

	// Read the user input until a newline character
	userInput, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading input:", err)
		return ""
	}

	// Trim any extra whitespace or newline characters
	return strings.TrimSpace(userInput)
}*/

// setKey is a helper function to set a key-value pair with a prefix (store).
func setKey(client *clientv3.Client, key, value string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.Put(ctx, key, value)
	if err != nil {
		//log.Printf("Failed to put key %s: %v", key, err)
	} else {
		//fmt.Printf("Set key %s with value %s\n", key, value)
	}
}
