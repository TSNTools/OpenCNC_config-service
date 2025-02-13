package moduleregistry

import (
	sw "config-service/pkg/store-wrapper"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type FileInfo struct {
	Directory string
	FileName  string
}

// ModuleRegistry represents an in-memory data structure: a map of maps.
type ModuleRegistry struct {
	Modules map[string]map[string]string
}

// NewModuleRegistry creates and returns a new instance of ModuleRegistry
func NewModuleRegistry() *ModuleRegistry {
	// Create the initial instance of ModuleRegistry
	mr := &ModuleRegistry{
		Modules: make(map[string]map[string]string),
	}

	// Call a function to populate the struct (e.g., initialize items)
	mr.createModuleRegistry()

	// Return the populated struct
	return mr
}

// UpdateModuleRegistry updates the current registry
func (mr *ModuleRegistry) UpdateModuleRegistry(outerKey, innerKey, innerValue string) {
	// Check if the outerKey exists, if not, create a new inner map
	if _, exists := mr.Modules[outerKey]; !exists {
		mr.Modules[outerKey] = make(map[string]string)
	}

	// Update the inner map with the new key-value pair
	mr.Modules[outerKey][innerKey] = innerValue
	//fmt.Printf("Registry updated: Outer Key = %s, Inner Key = %s, Value = %s\n", outerKey, innerKey, innerValue)
}

// Get retrieves the value for a given inner key under the specified outer key
func (mr *ModuleRegistry) Get(outerKey, innerKey string) string {
	if outerMap, exists := mr.Modules[outerKey]; exists {
		return outerMap[innerKey]
	}
	return ""
}

// PrintRegistry prints the entire registry (for debugging or visualization purposes)
func (mr *ModuleRegistry) PrintModuleRegistry() {
	fmt.Println("Current Module Registry:")
	for outerKey, innerMap := range mr.Modules {
		fmt.Printf("Outer Key: %s\n", outerKey)
		for innerKey, value := range innerMap {
			fmt.Printf("  Inner Key: %s, Value: %s\n", innerKey, value)
		}
	}
}

func (mr *ModuleRegistry) createModuleRegistry() {
	// Replace with the path to your directory
	dirPath := "pkg/yang_modules/gen_structures"

	// Read file names from the directory
	files, err := getFilesWithSubdirectories(dirPath)
	if err != nil {
		fmt.Printf("Error reading files: %v\n", err)
		return
	}

	// Call CreateEtcdClient and handle error
	client, err := sw.CreateEtcdClient()
	if err != nil {
		log.Fatal("Failed to create etcd client:", err)
	}
	defer client.Close()

	// Create store prexies from file names and revisions
	prefix0 := "yang-modules/"
	//counter := 1
	for _, file := range files {
		parts := strings.Split(file.FileName, "@")
		prefix := prefix0 + parts[0]

		if len(parts) == 2 {
			sw.SetKey(client, prefix+"/revision", parts[1])
			mr.UpdateModuleRegistry(parts[0], "revision", parts[1])

		} else {
			sw.SetKey(client, prefix+"/revision", "No Revision tag found.")
			mr.UpdateModuleRegistry(parts[0], "revision", "No Revision tag found.")
		}
		sw.SetKey(client, prefix+"/structure", file.Directory)
		mr.UpdateModuleRegistry(parts[0], "structure", file.Directory)
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
