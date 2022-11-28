package tools

import (
	sftools "github.com/streamingfast/sf-tools"
)

func init() {
	prometheusExporterCmd := sftools.GetFirehosePrometheusExporterCmd(zlog, tracer, transformsSetter)
	prometheusExporterCmd.Flags().String("call-filters", "", "call filters (format: '[address1[+address2[+...]]]:[eventsig1[+eventsig2[+...]]]")
	prometheusExporterCmd.Flags().String("log-filters", "", "log filters (format: '[address1[+address2[+...]]]:[eventsig1[+eventsig2[+...]]]")
	prometheusExporterCmd.Flags().Bool("send-all-block-headers", false, "ask for all the blocks to be sent (header-only if there is no match)")
	Cmd.AddCommand(prometheusExporterCmd)
}
