// Copyright 2021 dfuse Platform Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tools

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"cloud.google.com/go/bigtable"
	"github.com/golang/protobuf/proto"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/streamingfast/jsonpb"
	"github.com/streamingfast/kvdb"
	_ "github.com/streamingfast/kvdb/store/badger"
	_ "github.com/streamingfast/kvdb/store/bigkv"
	_ "github.com/streamingfast/kvdb/store/tikv"
	pbcodec "github.com/streamingfast/sf-ethereum/pb/sf/ethereum/codec/v1"
	trxdb "github.com/streamingfast/sf-ethereum/trxdb"
	"go.uber.org/multierr"
)

var dbCmd = &cobra.Command{Use: "db", Short: "Read from Ethereum database and manage it"}
var dbBlkCmd = &cobra.Command{Use: "blk", Short: "Read a block", RunE: dbReadBlockE, Args: cobra.ExactArgs(1)}
var dbTrxCmd = &cobra.Command{Use: "trx", Short: "Reads a transaction", RunE: dbReadTrxE, Args: cobra.ExactArgs(1)}

var dbRollbackCmd = &cobra.Command{Use: "rollback", Short: "Rollback information in the database"}
var dbRollbackIrrCmd = &cobra.Command{Use: "irreversible", Short: "Rollback irreversibility information in the database from a given irreversible block down to a limit block num", RunE: dbRollbackIrrE, Args: cobra.ExactArgs(1)}

var chainDiscriminator = func(blockID string) bool {
	return true
}

func init() {
	Cmd.AddCommand(dbCmd)
	dbCmd.AddCommand(dbBlkCmd)
	dbCmd.AddCommand(dbTrxCmd)

	dbCmd.AddCommand(dbRollbackCmd)
	dbRollbackCmd.AddCommand(dbRollbackIrrCmd)

	dbCmd.PersistentFlags().String("dsn", "badger:///sf-data/kvdb/kvdb_badger.db", "KVStore DSN")
}

func readBlockFromDB(ctx context.Context, ref string, db trxdb.DB) (blocks []*pbcodec.BlockWithRefs, err error) {
	if blockNum, err := strconv.ParseUint(ref, 10, 64); err == nil {
		dbBlocks, err := db.GetBlockByNum(ctx, blockNum)
		if err != nil {
			return nil, fmt.Errorf("failed to get block: %w", err)
		}
		blocks = append(blocks, dbBlocks...)
	} else {
		blockHash, err := hex.DecodeString(ref)
		if err != nil {
			return nil, fmt.Errorf("invalid block id %q: %w", ref, err)
		}

		dbBlock, err := db.GetBlock(ctx, blockHash)
		if err != nil {
			return nil, fmt.Errorf("failed to get block: %w", err)
		}
		blocks = append(blocks, dbBlock)
	}

	return blocks, nil
}

func dbReadBlockE(cmd *cobra.Command, args []string) (err error) {
	db, err := trxdb.New(viper.GetString("dsn"), trxdb.WithLogger(zlog))
	if err != nil {
		return fmt.Errorf("failed to setup db: %w", err)
	}

	blocks, err := readBlockFromDB(cmd.Context(), args[0], db)
	for _, blk := range blocks {
		printEntity(blk)

	}

	return nil
}

func dbRollbackIrrE(cmd *cobra.Command, args []string) (err error) {

	lowBlockNum, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("cannot parse value for lowBlockNum (%s): %w", args[0], err)
	}

	db, err := trxdb.New(viper.GetString("dsn"), trxdb.WithLogger(zlog))
	if err != nil {
		return fmt.Errorf("failed to setup db: %w", err)
	}

	lib, err := db.GetLastWrittenIrreversibleBlockRef(cmd.Context())
	if err != nil {
		return fmt.Errorf("no lastWrittenIrrBlock found: %w", err)
	}

	for i := lib.Num(); i > lowBlockNum; i-- {
		dbBlocks, err := db.GetBlockByNum(cmd.Context(), i)
		if err != nil {
			return fmt.Errorf("failed to get block: %w", err)
		}
		for _, b := range dbBlocks {
			err := db.RevertNowIrreversibleBlock(cmd.Context(), b.Block)
			if err != nil {
				return err
			}
		}

		if i%10 == 0 {
			fmt.Printf("processing %d, going back to %d (%d done, %d remaining)\n", i, lowBlockNum, lib.Num()-i, i-lowBlockNum)
		}
		fmt.Printf("DONE! New LastWrittenIrreversibleBlock is now: %d\n", i)

	}
	return nil
}

func dbReadTrxE(cmd *cobra.Command, args []string) (err error) {
	db, err := trxdb.New(viper.GetString("dsn"), trxdb.WithLogger(zlog))
	if err != nil {
		return err
	}
	trxID := args[0]
	trxHash, err := hex.DecodeString(trxID)
	if err != nil {
		return fmt.Errorf("invalid transaction id %q: %w", trxID, err)
	}

	traces, err := db.GetTransaction(cmd.Context(), trxHash)
	if err == kvdb.ErrNotFound {
		return fmt.Errorf("Transaction %q not found", trxID)
	}
	if err != nil {
		return fmt.Errorf("Failed to get transaction: %w", err)
	}

	for _, trace := range traces {
		printEntity(trace)
	}

	return nil
}

func printEntity(pb proto.Message) (err error) {
	cnt, err := jsonpb.MarshalIndentToString(pb, "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(cnt))
	return nil
}

/// Helpers for old dfuse stack information, should be deleted once Ethereum is all migrated to new code

func blockKey(hash string, num uint64) string {
	return fmt.Sprintf("blkh:%s:%016x", hash, ^num)
}

func explodeBlockHashKey(key string) (hash string, num uint64, err error) {
	parts := strings.Split(key, ":")
	revNum, err := strconv.ParseUint(parts[2], 16, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid block number %q: %w", parts[1], err)
	}

	return parts[1], ^revNum, nil
}

func blockNumKey(hash string, num uint64) string {
	return fmt.Sprintf("blkn:%016x:%s", ^num, hash)
}

func explodeBlockNumKey(key string) (hash string, num uint64, err error) {
	parts := strings.Split(key, ":")
	revNum, err := strconv.ParseUint(parts[1], 16, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid block number %q: %w", parts[1], err)
	}

	return parts[2], ^revNum, nil
}

func readValue(tag string, item bigtable.ReadItem, pb proto.Message) error {
	err := proto.Unmarshal(item.Value, pb)
	if err != nil {
		return fmt.Errorf("unable to read %q: %w", tag, err)
	}

	return nil
}

func flushMutations(ctx context.Context, table *bigtable.Table, keys []string, mutations []*bigtable.Mutation) error {
	fmt.Printf("Flushing mutations on %d keys\n", len(keys))
	errs, err := table.ApplyBulk(ctx, keys, mutations)
	if err != nil {
		return fmt.Errorf("unable to applied bulk changes: %w", err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("unable to applied all bulk changes: %w", multierr.Combine(errs...))
	}

	return nil
}
