package main

import (
	"github.com/spf13/cobra"
	"github.com/streamingfast/cli"
	firecore "github.com/streamingfast/firehose-core"
)

var blockEncoder = firecore.NewBlockEncoder()

func examplePrefixed(prefix, examples string) string {
	return string(cli.ExamplePrefixed(prefix, examples))
}

func registerGroup(parent *cobra.Command, group cli.CommandOption) {
	group.Apply(parent)
}
