package tools

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/streamingfast/eth-go"
	pbtransform "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/transform/v1"
	sftools "github.com/streamingfast/sf-tools"
	"google.golang.org/protobuf/types/known/anypb"
)

func init() {
	firehoseClientCmd := sftools.GetFirehoseClientCmd(zlog, tracer, transformsSetter)
	firehoseClientCmd.Flags().String("call-filters", "", "call filters (format: '[address1[+address2[+...]]]:[eventsig1[+eventsig2[+...]]]")
	firehoseClientCmd.Flags().String("log-filters", "", "log filters (format: '[address1[+address2[+...]]]:[eventsig1[+eventsig2[+...]]]")
	Cmd.AddCommand(firehoseClientCmd)
}

var transformsSetter = func(cmd *cobra.Command) (transforms []*anypb.Any, err error) {
	filters, err := parseFilters(mustGetString(cmd, "call-filters"), mustGetString(cmd, "log-filters"))
	if err != nil {
		return nil, err
	}

	if filters != nil {
		t, err := anypb.New(filters)
		if err != nil {
			return nil, err
		}
		transforms = append(transforms, t)
	}
	return
}

func parseFilters(callFilters, logFilters string) (*pbtransform.CombinedFilter, error) {
	mf := &pbtransform.CombinedFilter{}

	if callFilters == "" && logFilters == "" {
		return nil, nil
	}
	if callFilters != "" {
		for _, filter := range strings.Split(callFilters, ",") {
			if filter == "" {
				continue
			}
			parts := strings.Split(filter, ":")
			if len(parts) != 2 {
				return nil, fmt.Errorf("option --call-filters must be of type address_hash+address_hash+address_hash:event_sig_hash+event_sig_hash (repeated, separated by comma)")
			}
			var addrs []eth.Address
			for _, a := range strings.Split(parts[0], "+") {
				if a != "" {
					addr := eth.MustNewAddress(a)
					addrs = append(addrs, addr)
				}
			}
			var sigs []eth.Hash
			for _, s := range strings.Split(parts[1], "+") {
				if s != "" {
					sig := eth.MustNewHash(s)
					sigs = append(sigs, sig)
				}
			}

			mf.CallFilters = append(mf.CallFilters, basicCallToFilter(addrs, sigs))
		}
	}

	if logFilters != "" {
		for _, filter := range strings.Split(logFilters, ",") {
			if filter == "" {
				continue
			}
			parts := strings.Split(filter, ":")
			if len(parts) != 2 {
				return nil, fmt.Errorf("option --log-filters must be of type address_hash+address_hash+address_hash:event_sig_hash+event_sig_hash (repeated, separated by comma)")
			}
			var addrs []eth.Address
			for _, a := range strings.Split(parts[0], "+") {
				if a != "" {
					addr := eth.MustNewAddress(a)
					addrs = append(addrs, addr)
				}
			}
			var sigs []eth.Hash
			for _, s := range strings.Split(parts[1], "+") {
				if s != "" {
					sig := eth.MustNewHash(s)
					sigs = append(sigs, sig)
				}
			}

			mf.LogFilters = append(mf.LogFilters, basicLogFilter(addrs, sigs))
		}
	}

	return mf, nil
}

func basicCallToFilter(addrs []eth.Address, sigs []eth.Hash) *pbtransform.CallToFilter {
	var addrBytes [][]byte
	var sigsBytes [][]byte

	for _, addr := range addrs {
		b := addr.Bytes()
		addrBytes = append(addrBytes, b)
	}

	for _, sig := range sigs {
		b := sig.Bytes()
		sigsBytes = append(sigsBytes, b)
	}

	return &pbtransform.CallToFilter{
		Addresses:  addrBytes,
		Signatures: sigsBytes,
	}
}

func basicLogFilter(addrs []eth.Address, sigs []eth.Hash) *pbtransform.LogFilter {
	var addrBytes [][]byte
	var sigsBytes [][]byte

	for _, addr := range addrs {
		b := addr.Bytes()
		addrBytes = append(addrBytes, b)
	}

	for _, sig := range sigs {
		b := sig.Bytes()
		sigsBytes = append(sigsBytes, b)
	}

	return &pbtransform.LogFilter{
		Addresses:       addrBytes,
		EventSignatures: sigsBytes,
	}
}
