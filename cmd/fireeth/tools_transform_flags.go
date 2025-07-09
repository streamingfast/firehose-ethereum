package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/streamingfast/cli/sflags"
	"github.com/streamingfast/eth-go"
	pbtransform "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/transform/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/anypb"
)

func parseTransformFlags(cmd *cobra.Command, logger *zap.Logger) (transforms []*anypb.Any, err error) {
	filters, err := parseFilters(sflags.MustGetString(cmd, "call-filters"), sflags.MustGetString(cmd, "log-filters"), sflags.MustGetBool(cmd, "send-all-block-headers"))
	if err != nil {
		return nil, err
	}

	headerOnly := sflags.MustGetBool(cmd, "header-only")
	if filters != nil && headerOnly {
		return nil, fmt.Errorf("'header-only' flag is exclusive with 'call-filters', 'log-filters' and 'send-all-block-headers' choose either 'header-only' or a combination of the others")
	}

	if headerOnly {
		t, err := anypb.New(&pbtransform.HeaderOnly{})
		if err != nil {
			return nil, err
		}

		return []*anypb.Any{t}, nil
	}

	if filters != nil {
		t, err := anypb.New(filters)
		if err != nil {
			return nil, err
		}

		return []*anypb.Any{t}, nil
	}

	return
}

func parseFilters(callFilters, logFilters string, sendAllBlockHeaders bool) (*pbtransform.CombinedFilter, error) {
	mf := &pbtransform.CombinedFilter{}

	if callFilters == "" && logFilters == "" && !sendAllBlockHeaders {
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
					addr := eth.MustNewAddressLoose(a)
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

	if sendAllBlockHeaders {
		mf.SendAllBlockHeaders = true
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
