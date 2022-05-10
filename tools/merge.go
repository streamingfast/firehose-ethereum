package tools

import (
	sftools "github.com/streamingfast/sf-tools"
)

func init() {
	Cmd.AddCommand(sftools.GetMergeCmd(zlog, tracer))
}
