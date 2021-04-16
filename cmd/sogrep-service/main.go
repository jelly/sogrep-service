package main

import (
	"github.com/jelly/sogrep-service/cmd"
)


func main() {
	cmd.RootCmd.AddCommand(commandServe())
	cmd.RootCmd.Execute()
}
