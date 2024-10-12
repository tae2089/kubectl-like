package main

import "github.com/tae2089/kubectl-like/cmd"

func main() {
	rooCmd := cmd.CreateRootCmd()
	if err := rooCmd.Execute(); err != nil {
		panic(err)
	}
}
