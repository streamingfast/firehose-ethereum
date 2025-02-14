package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
	"github.com/streamingfast/cli"
	fcjson "github.com/streamingfast/firehose-core/json"
	fcproto "github.com/streamingfast/firehose-core/proto"
	pbeth "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	"google.golang.org/protobuf/encoding/protojson"
)

func main() {
	protoRegistry, err := fcproto.NewRegistry(nil)
	cli.NoError(err, "failed to create proto registry")

	jsonMarshaller := fcjson.NewMarshaller(protoRegistry, fcjson.WithBytesEncoding("hex"))
	jsonEncodeOptions := []json.Options{jsontext.WithIndent("  ")}

	files, err := filepath.Glob("./comparisons/*.new.json")
	cli.NoError(err, "failed to find files")

	for _, file := range files {
		data, err := os.ReadFile(file)
		cli.NoError(err, "failed to read file")

		var freeFormBlock map[string]any
		err = json.Unmarshal(data, &freeFormBlock)
		cli.NoError(err, "failed to decode block")

		delete(freeFormBlock, "@type")

		data, err = json.Marshal(freeFormBlock)
		cli.NoError(err, "failed to encode block")

		block := &pbeth.Block{}
		err = protojson.Unmarshal(data, block)
		cli.NoError(err, "failed to decode block")

		showBlockHeader := false

		for _, tx := range block.TransactionTraces {
			for _, call := range tx.Calls {
				var sameOrdinal bool
				var firstOrdinal uint64
				if len(call.AccountCreations) > 0 {
					sameOrdinal = len(call.AccountCreations) > 1
					firstOrdinal = call.AccountCreations[0].Ordinal
				}

				for _, accountCreation := range call.AccountCreations {
					if accountCreation.Ordinal != firstOrdinal {
						sameOrdinal = false
					}
				}

				if sameOrdinal {
					if !showBlockHeader {
						showBlockHeader = true
						fmt.Printf("Block %s\n", hex.EncodeToString(block.Hash))
						fmt.Println()
					}

					fmt.Printf("Tx %s (Type %s)\n", hex.EncodeToString(tx.Hash), tx.Type.String())

					out, err := jsonMarshaller.MarshalToString(call.AccountCreations, jsonEncodeOptions...)
					cli.NoError(err, "failed to encode account creations")
					fmt.Println(out)
					fmt.Println()
				}
			}
		}

		if showBlockHeader {
			fmt.Println("=============================================================")
		}
	}

	fmt.Println("completed")
}
