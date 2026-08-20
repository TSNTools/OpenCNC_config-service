// go run . -test <type> [-configId <id>]
//
// Examples:
//   go run . -test qbv-relym
//   go run . -test qbv-relym-multi
//   go run . -test qbv-bandr-multi
//   go run . -test qbv-tttech
//   go run . -test vlan-tttech
//   go run . -test priority-tttech
//   go run . -test apply-config -configId 7e14f5f9-6d6b-413c-b4d4-2d2c1a86141b

package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	testType := flag.String("test", "apply-config", "Type of test to run: apply-config, qbv-relym, qbv-relym-multi, qbv-bandr-multi, qbv-tttech, vlan-tttech, priority-tttech, netconf-protocol, all")
	configID := flag.String("configId", "2f009d9e-3efe-4e59-a750-bac2f30e4d5a", "Configuration ID (used with apply-config)")
	flag.StringVar(configID, "id", "2f009d9e-3efe-4e59-a750-bac2f30e4d5a", "Short flag for configuration ID")
	flag.Parse()

	switch *testType {
	case "apply-config":
		fmt.Printf("=== Running ApplyConfigurationById Test (ConfigID: %s) ===\n", *configID)
		TestApplyConfigByIdWithRunningConfig(*configID)

	case "qbv-relym":
		fmt.Println("=== Running RELY-TSN4 Single-Port QBV Test ===")
		TestQbvPlugin_relym()

	case "qbv-relym-multi":
		fmt.Println("=== Running RELY-TSN4 Multi-Port QBV Test ===")
		TestQbvPlugin_relym_multiPort()

	case "qbv-bandr-multi", "qbv-br2018":
		fmt.Println("=== Running B&R (2018) Multi-Port QBV Test ===")
		TestQbvPlugin_br2018_multiPort()

	case "qbv-tttech":
		fmt.Println("=== Running TTTech EVB QBV Test ===")
		TestQbvPlugin_tttech()

	case "vlan-tttech":
		fmt.Println("=== Running TTTech EVB VLAN Test ===")
		TestVlanPlugin_tttech()

	case "priority-tttech":
		fmt.Println("=== Running TTTech EVB Priority PCP Test ===")
		TestPriorityPlugin_tttech()

	case "netconf-protocol":
		fmt.Println("=== Running Netconf Protocol Backend Test ===")
		TestNetconfProtocol()

	case "all":
		fmt.Println("=== Running All Available Plugin Tests ===")
		fmt.Println("\n[1/7] QBV RELY-TSN4 Test:")
		TestQbvPlugin_relym()
		fmt.Println("\n[2/7] QBV RELY-TSN4 Multi-Port Test:")
		TestQbvPlugin_relym_multiPort()
		fmt.Println("\n[3/7] QBV B&R Multi-Port Test:")
		TestQbvPlugin_br2018_multiPort()
		fmt.Println("\n[4/7] QBV TTTech Test:")
		TestQbvPlugin_tttech()
		fmt.Println("\n[5/7] VLAN TTTech Test:")
		TestVlanPlugin_tttech()
		fmt.Println("\n[6/7] Priority TTTech Test:")
		TestPriorityPlugin_tttech()
		fmt.Println("\n[7/7] ApplyConfigurationById Test:")
		TestApplyConfigByIdWithRunningConfig(*configID)

	default:
		fmt.Printf("Unknown test type: %s\n\n", *testType)
		fmt.Println("Available test types:")
		fmt.Println("  - apply-config       : Test ApplyConfigurationById via engine & running config")
		fmt.Println("  - qbv-relym          : Test QBV on RELY-TSN4 switch (single port)")
		fmt.Println("  - qbv-relym-multi    : Test QBV on RELY-TSN4 switch (multi port)")
		fmt.Println("  - qbv-bandr-multi    : Test QBV on B&R 2018 switch (multi port)")
		fmt.Println("  - qbv-tttech         : Test QBV on TTTech EVB switch")
		fmt.Println("  - vlan-tttech        : Test VLAN on TTTech EVB switch")
		fmt.Println("  - priority-tttech    : Test Priority PCP mapping on TTTech EVB switch")
		fmt.Println("  - netconf-protocol   : Test Netconf Protocol Backend")
		fmt.Println("  - all                : Run all test suites")
		os.Exit(1)
	}
}
