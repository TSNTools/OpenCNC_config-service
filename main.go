package main

import (
	//trees "config_service/pkg/yang_modules/trees"

	storewrapper "config-service/pkg/store-wrapper"

	"git.cs.kau.se/hamzchah/opencnc_kafka-exporter/logger/pkg/logger"
)

var log = logger.GetLogger()

// The main entry point
func main() {
	log.Infof("Starting config service")

	// Example usage: Pass the file path to Parse
	//trees.TreeParser("pkg/yang_modules/yang/ieee802-dot1ab-lldp.yang")

	moduleregistry, _ := storewrapper.GetModuleRegistry()

	moduleregistry.PrintModuleRegistry()

	devicemodelregistry, _ := storewrapper.GetDeviceModelRegistry()

	devicemodelregistry.Print()

	topology, _ := storewrapper.GetTopology()

	topology.Print()
}

/*
	// Path to the YANG model files
	yangModelPath := "./path/to/yang/models"
	// Output directory for the generated Go code
	outputDir := "./generated_code"

	// Specify the input YANG files and the output Go file
	files := []string{
		// Replace these with your actual YANG model file paths
		"model.yang",
	}

	// Run the YANG-to-Go code generator
	if err := runYANGGenerator(yangModelPath, files, outputDir); err != nil {
		log.Fatalf("Error running YANG generator: %v", err)
	}

fmt.Println("Go code generation successful!")*/

/*func runYANGGenerator(yangModelPath string, yangFiles []string, outputDir string) error {
	// Set up the YANG model files for generation
	genParams := &ygot.GeneratorParameters{
		YangModelsDir: yangModelPath, // Path to YANG models
		YangFiles:     yangFiles,     // YANG files to process
		OutputDir:     outputDir,     // Output directory for generated code
	}

	// Generate Go code from the YANG files
	if err := genutil.GenerateCode(genParams); err != nil {
		return fmt.Errorf("failed to generate Go code from YANG files: %v", err)
	}

	return nil
}*/
