package main

import (
	"github.com/jelly/sogrep-service/cmd"
	"github.com/jelly/sogrep-service/cmd/sogrep-service/gen"
)


func main() {
	cmd.RootCmd.AddCommand(commandServe())
	cmd.RootCmd.AddCommand(gen.CommandGen())
	cmd.RootCmd.Execute()
}
