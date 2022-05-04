package tools

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/streamingfast/eth-go"
	"github.com/streamingfast/firehose/client"
	"github.com/streamingfast/jsonpb"
	pbfirehose "github.com/streamingfast/pbgo/sf/firehose/v1"
	pbtransform "github.com/streamingfast/sf-ethereum/types/pb/sf/ethereum/transform/v1"
	"google.golang.org/protobuf/types/known/anypb"
)

var retryDelay = time.Second

func init() {
	Cmd.AddCommand(FirehoseClientCmd)
	FirehoseClientCmd.Flags().StringP("api-token-env-var", "a", "FIREHOSE_API_TOKEN", "Look for a JWT in this environment variable to authenticate against endpoint")
	FirehoseClientCmd.Flags().String("call-filters", "", "call filters (format: '[address1[+address2[+...]]]:[eventsig1[+eventsig2[+...]]]")
	FirehoseClientCmd.Flags().String("log-filters", "", "log filters (format: '[address1[+address2[+...]]]:[eventsig1[+eventsig2[+...]]]")
	FirehoseClientCmd.Flags().BoolP("plaintext", "p", false, "Use plaintext connection to firehose")
	FirehoseClientCmd.Flags().BoolP("insecure", "k", false, "Skip SSL certificate validation when connecting to firehose")
}

var FirehoseClientCmd = &cobra.Command{
	Use:     "firehose-client",
	Short:   "print firehose block stream as JSON",
	Args:    cobra.ExactArgs(3),
	RunE:    firehoseClientE,
	Example: "sfeth tools firehose-client api.streamingfast.io 1000 2000",
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

func firehoseClientE(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	endpoint := args[0]
	start, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		return fmt.Errorf("parsing start block num: %w", err)
	}
	stop, err := strconv.ParseUint(args[2], 10, 64)
	if err != nil {
		return fmt.Errorf("parsing stop block num: %w", err)
	}
	apiTokenEnvVar := mustGetString(cmd, "api-token-env-var")
	jwt := os.Getenv(apiTokenEnvVar)

	plaintext := mustGetBool(cmd, "plaintext")
	insecure := mustGetBool(cmd, "insecure")

	firehoseClient, grpcCallOpts, err := client.NewFirehoseClient(endpoint, jwt, insecure, plaintext)
	if err != nil {
		return err
	}

	forkSteps := []pbfirehose.ForkStep{pbfirehose.ForkStep_STEP_NEW}

	filters, err := parseFilters(mustGetString(cmd, "call-filters"), mustGetString(cmd, "log-filters"))
	if err != nil {
		return err
	}

	var transforms []*anypb.Any
	if filters != nil {
		t, err := anypb.New(filters)
		if err != nil {
			return err
		}
		transforms = append(transforms, t)
	}

	request := &pbfirehose.Request{
		StartBlockNum: int64(start),
		StopBlockNum:  stop,
		ForkSteps:     forkSteps,
		Transforms:    transforms,
	}

	stream, err := firehoseClient.Blocks(ctx, request, grpcCallOpts...)
	if err != nil {
		return fmt.Errorf("unable to start blocks stream: %w", err)
	}

	for {
		response, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}

			return fmt.Errorf("stream error while receiving: %w", err)
		}

		line, err := jsonpb.MarshalToString(response)
		if err != nil {
			return fmt.Errorf("unable to marshal block %s to JSON", response)
		}

		fmt.Println(line)
	}

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
