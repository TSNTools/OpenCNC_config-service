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
	trees "config_service/pkg/yang_modules/trees"

	"git.cs.kau.se/hamzchah/opencnc_kafka-exporter/logger/pkg/logger"
)

var log = logger.GetLogger()

// The main entry point
func main() {
	log.Infof("Starting config service")

	// Example usage: Pass the file path to Parse
	trees.TreeParser("./yang_modules/yang/ieee802-dot1ab-lldp.yang")
}
