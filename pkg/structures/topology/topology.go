package topology

import "fmt"

// Method to print the topology
func (topo *Topology) Print() {
	if topo == nil {
		fmt.Println("Topology is empty.")
		return
	}
	fmt.Println("Nodes:")

	for _, node := range topo.Nodes {
		fmt.Printf("	Type: %s\n", node.Type)
		fmt.Printf("	Name: %s\n", node.Name)
		if node.Type == "bridges" {
			fmt.Printf("	Processing delay: %d\n", node.Bridge.ProcessingDelay)
		}

		fmt.Println("	Ports:")
		for _, port := range node.Ports {
			fmt.Printf("		ID: %s\n", port.Id)
			fmt.Printf("		Name: %s\n", port.Name)
			fmt.Printf("		Speed: %v\n", port.PortSpeed)
			fmt.Printf("		Number of queues: %v\n", port.NumberOfQueues)
			fmt.Println()
		}
		fmt.Println()
	}

	fmt.Println("Links:")
	for _, link := range topo.Links {
		fmt.Printf("	ID: %s\n", link.Id)
		fmt.Printf("	Source: %s\n", link.Source)
		fmt.Printf("	Target: %s\n", link.Target)
		fmt.Printf("	Propagation delay: %d\n", link.PropogationDelay)
		fmt.Printf("	Bandwidth: %d\n", link.Bandwidth)
		fmt.Println()
	}
}
