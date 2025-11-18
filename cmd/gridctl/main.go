package main

import "github.com/terraconstructs/grid/cmd/gridctl/cmd"

func main() {
	// Set version information from build-time variables
	cmd.SetVersion(version, commit, date, builtBy)
	cmd.Execute()
}
