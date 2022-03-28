package tools

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/streamingfast/bstream"
	dfuse "github.com/streamingfast/client-go"
	"github.com/streamingfast/dgrpc"
	"github.com/streamingfast/dstore"
	pbfirehose "github.com/streamingfast/pbgo/sf/firehose/v1"
	"github.com/streamingfast/sf-ethereum/codec"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/credentials/oauth"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func init() {
	Cmd.AddCommand(DownloadFromFirehoseCmd)
	//CheckCmd.PersistentFlags().StringP("range", "r", "", "Block range to use for the check")
	DownloadFromFirehoseCmd.Flags().StringP("api-key-env-var", "a", "STREAMINGFAST_API_KEY", "Look for an API key in this environment variable")
	DownloadFromFirehoseCmd.Flags().BoolP("plaintext", "p", false, "Use plaintext connection to firehose")
	DownloadFromFirehoseCmd.Flags().BoolP("insecure", "k", false, "Skip SSL certificate validation when connecting to firehose")
}

var DownloadFromFirehoseCmd = &cobra.Command{
	Use:     "download-from-firehose",
	Short:   "download block files from firehose",
	Args:    cobra.ExactArgs(4),
	RunE:    downloadFromFirehoseE,
	Example: "sfeth tools download-from-firehose api.streamingfast.io 1000 2000 ./outputdir",
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

	apiKeyEnvVar := viper.GetString("api-key-env-var")
	apiKey := os.Getenv(apiKeyEnvVar)

	plaintext := viper.GetBool("plaintext")
	insecure := viper.GetBool("insecure")

	return streamBlocks(ctx, endpoint, apiKey, insecure, plaintext, start, stop, destFolder)
}

var retryDelay = 5 * time.Second

type CallOptionsGetter func(context.Context) ([]grpc.CallOption, error)

func nilCallOptionsGetter(context.Context) ([]grpc.CallOption, error) {
	return nil, nil
}

func NewFirehoseClient(endpoint, apiKey string, useInsecureTSLConnection, usePlainTextConnection bool) (pbfirehose.StreamClient, CallOptionsGetter, error) {
	var skipAuth bool
	var clientOptions []dfuse.ClientOption
	if apiKey == "" {
		clientOptions = []dfuse.ClientOption{dfuse.WithoutAuthentication()}
		skipAuth = true
	}

	client, err := dfuse.NewClient(endpoint, apiKey, clientOptions...)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create streamingfast client")
	}

	if useInsecureTSLConnection && usePlainTextConnection {
		return nil, nil, fmt.Errorf("option --insecure and --plaintext are mutually exclusive, they cannot be both specified at the same time")
	}

	var dialOptions []grpc.DialOption
	switch {
	case usePlainTextConnection:
		zlog.Debug("Configuring transport to use a plain text connection")
		dialOptions = []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	case useInsecureTSLConnection:
		zlog.Debug("Configuring transport to use an insecure TLS connection (skips certificate verification)")
		dialOptions = []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}))}
	}

	conn, err := dgrpc.NewExternalClient(endpoint, dialOptions...)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create external gRPC client")
	}

	getCallOpts := nilCallOptionsGetter
	if !skipAuth {
		getCallOpts = func(ctx context.Context) ([]grpc.CallOption, error) {
			tokenInfo, err := client.GetAPITokenInfo(ctx)
			if err != nil {
				return nil, fmt.Errorf("unable to retrieve StreamingFast API token: %w", err)
			}
			credentials := oauth.NewOauthAccess(&oauth2.Token{AccessToken: tokenInfo.Token, TokenType: "Bearer"})
			return []grpc.CallOption{grpc.PerRPCCredentials(credentials)}, nil
		}
	}

	return pbfirehose.NewStreamClient(conn), getCallOpts, err
}

func newMergedBlocksWriter(store dstore.Store) *mergedBlocksWriter {
	return &mergedBlocksWriter{
		store:         store,
		writerFactory: bstream.GetBlockWriterFactory,
	}
}

type mergedBlocksWriter struct {
	store         dstore.Store
	lowBlockNum   uint64
	blocks        []*bstream.Block
	writerFactory bstream.BlockWriterFactory
}

func (w *mergedBlocksWriter) process(blk *bstream.Block) error {
	if blk.Number%100 == 0 && w.lowBlockNum == 0 {
		if w.lowBlockNum == 0 { // initial block
			w.lowBlockNum = blk.Number
			w.blocks = append(w.blocks, blk)
			return nil
		}
	}

	if blk.Number == w.lowBlockNum+99 {
		w.blocks = append(w.blocks, blk)

		if err := w.writeBundle(); err != nil {
			return err
		}
		return nil
	}
	w.blocks = append(w.blocks, blk)

	return nil
}

func filename(num uint64) string {
	return fmt.Sprintf("%010d", num)
}

func (w *mergedBlocksWriter) writeBundle() error {
	file := filename(w.lowBlockNum)
	zlog.Info("writing merged file to store (suffix: .dbin.zst)", zap.String("filename", file), zap.Uint64("lowBlockNum", w.lowBlockNum))

	if len(w.blocks) == 0 {
		return fmt.Errorf("no blocks to write to bundle")
	}
	writeDone := make(chan struct{})

	pr, pw := io.Pipe()
	defer func() {
		pw.Close()
		<-writeDone
	}()

	go func() {
		err := w.store.WriteObject(context.Background(), file, pr)
		if err != nil {
			zlog.Error("writing to store", zap.Error(err))
		}
		w.lowBlockNum += 100
		w.blocks = nil
		close(writeDone)
	}()

	blockWriter, err := w.writerFactory.New(pw)
	if err != nil {
		return err
	}

	for _, blk := range w.blocks {
		if err := blockWriter.Write(blk); err != nil {
			return err
		}
	}

	return err
}

func streamBlocks(ctx context.Context, endpoint string, apiKey string, insecure, plaintext bool, startBlock, stopBlock uint64, destFolder string) error {
	firehoseClient, getGRPCOpts, err := NewFirehoseClient(endpoint, apiKey, insecure, plaintext)
	if err != nil {
		return err
	}

	store, err := dstore.NewDBinStore(destFolder)
	if err != nil {
		return err
	}
	mergeWriter := newMergedBlocksWriter(store)

	for {
		forkSteps := []pbfirehose.ForkStep{pbfirehose.ForkStep_STEP_IRREVERSIBLE}

		request := &pbfirehose.Request{
			StartBlockNum: int64(startBlock),
			StopBlockNum:  stopBlock,
			ForkSteps:     forkSteps,
		}

		grpcCallOpts, err := getGRPCOpts(ctx)
		if err != nil {
			return fmt.Errorf("unable to get grpc options: %w", err)
		}
		stream, err := firehoseClient.Blocks(context.Background(), request, grpcCallOpts...)
		if err != nil {
			return fmt.Errorf("unable to start blocks stream: %w", err)
		}

		for {
			response, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					return nil
				}

				zlog.Error("Stream encountered a remote error, going to retry",
					zap.Duration("retry_delay", retryDelay),
					zap.Error(err),
				)
				<-time.After(retryDelay)
				break
			}

			block := &pbcodec.Block{}
			if err := anypb.UnmarshalTo(response.Block, block, proto.UnmarshalOptions{}); err != nil {
				return fmt.Errorf("unmarshal anypb: %w", err)
			}

			blk, err := codec.BlockFromProto(block)
			if err != nil {
				return fmt.Errorf("bstream block from proto: %w", err)
			}

			if err := mergeWriter.process(blk); err != nil {
				return fmt.Errorf("write to blockwriter: %w", err)
			}

		}

	}

}
