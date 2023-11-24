package main

import (
	"github.com/spf13/cobra"
	firecore "github.com/streamingfast/firehose-core"
	"github.com/streamingfast/logging"

	"go.uber.org/zap"
)

func newPollerCmd(logger *zap.Logger, tracer logging.Tracer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "poller",
		Short: "poll blocks from different sources",
		Args:  cobra.ExactArgs(2),
		RunE:  pollerRunE(logger, tracer),
	}

	return cmd
}

func pollerRunE(logger *zap.Logger, tracer logging.Tracer) firecore.CommandExecutor {
	return func(cmd *cobra.Command, args []string) (err error) {
		//ctx := cmd.Context()
		//blockpoller := blockpoller.New(
		return nil
	}
}
