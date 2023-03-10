package tools

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/firehose-ethereum/types"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	sftools "github.com/streamingfast/sf-tools"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func init() {
	Cmd.AddCommand(DownloadFromFirehoseCmd)
	DownloadFromFirehoseCmd.Flags().StringP("api-token-env-var", "a", "FIREHOSE_API_TOKEN", "Look for a JWT in this environment variable to authenticate against endpoint")
	DownloadFromFirehoseCmd.Flags().BoolP("plaintext", "p", false, "Use plaintext connection to firehose")
	DownloadFromFirehoseCmd.Flags().BoolP("insecure", "k", false, "Skip SSL certificate validation when connecting to firehose")
	DownloadFromFirehoseCmd.Flags().Bool("fix-ordinals", false, "Decode the eth blocks to fix the ordinals in the receipt logs")
}

var DownloadFromFirehoseCmd = &cobra.Command{
	Use:   "download-from-firehose <endpoint> <start> <stop> <destination>",
	Short: "download blocks from firehose and save them to merged-blocks",
	Args:  cobra.ExactArgs(4),
	RunE:  downloadFromFirehoseE,
	Example: ExamplePrefixed("fireeth tools download-from-firehose", `
		api.streamingfast.io 1000 2000 ./outputdir
	`),
}

func downloadFromFirehoseE(cmd *cobra.Command, args []string) error {
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
	destFolder := args[3]

	apiTokenEnvVar := mustGetString(cmd, "api-token-env-var")
	apiToken := os.Getenv(apiTokenEnvVar)

	plaintext := mustGetBool(cmd, "plaintext")
	insecure := mustGetBool(cmd, "insecure")
	var fixerFunc func(*bstream.Block) (*bstream.Block, error)
	if mustGetBool(cmd, "fix-ordinals") {
		fixerFunc = func(in *bstream.Block) (*bstream.Block, error) {
			block := in.ToProtocol().(*pbeth.Block)
			return types.BlockFromProto(block, in.LibNum)
		}
	}

	return sftools.DownloadFirehoseBlocks(
		ctx,
		endpoint,
		apiToken,
		insecure,
		plaintext,
		start,
		stop,
		destFolder,
		decodeAnyPB,
		fixerFunc,
		zlog,
	)
}

func decodeAnyPB(in *anypb.Any) (*bstream.Block, error) {
	block := &pbeth.Block{}
	if err := anypb.UnmarshalTo(in, block, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("unmarshal anypb: %w", err)
	}

	// We are downloading only final blocks from the Firehose connection which means the LIB for them
	// can be set to themself (althought we use `- 1` to ensure problem would occur if codde don't like
	// `LIBNum == self.BlockNum`).
	return types.BlockFromProto(block, block.Number-1)
}
