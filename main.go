package main

import "github.com/tae2089/kubectl-like/cmd"

func main() {
	// if rooCmd.Use == "" {
	// 	panic("root command not initialized")
	// }
	rooCmd := cmd.CreateRootCmd()
	if err := rooCmd.Execute(); err != nil {
		panic(err)
	}
}
