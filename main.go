// SPDX-FileCopyrightText: 2020-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

/*
Package onos-config is the main entry point to the ONOS configuration subsystem.

It connects to devices through a Southbound gNMI interface and
gives a gNMI interface northbound for other systems to connect to, and an
Admin service through gRPC

Arguments
-allowUnvalidatedConfig <allow configuration for devices without a corresponding model plugin>

-modelPlugin (repeated) <the location of a shared object library that implements the Model Plugin interface>

-caPath <the location of a CA certificate>

-keyPath <the location of a client private key>

-certPath <the location of a client certificate>

See ../../docs/run.md for how to run the application.
*/
package main

import (
	//trees "config_service/pkg/yang_modules/trees"

	//moduleregistry "config_service/pkg/module-registry"

	moduleregistry "config-service/pkg/module-registry"

	"git.cs.kau.se/hamzchah/opencnc_kafka-exporter/logger/pkg/logger"
)

var log = logger.GetLogger()

// The main entry point
func main() {
	log.Infof("Starting config service")

	// Example usage: Pass the file path to Parse
	//trees.TreeParser("pkg/yang_modules/yang/ieee802-dot1ab-lldp.yang")

	mr := moduleregistry.NewModuleRegistry()

	mr.PrintModuleRegistry()

	//addswitch.MapConfigStructures(mr)

	// Now create your ModuleRegistry that uses the etcd client
	//mr := moduleregistry.NewModuleRegistry()

	// Now you can use mr and the etcd client
	//fmt.Println("ModuleRegistry initialized with etcd client:", mr)
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
