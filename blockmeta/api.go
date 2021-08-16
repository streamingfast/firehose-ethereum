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

package blockmeta

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	pbheadinfo "github.com/streamingfast/pbgo/dfuse/headinfo/v1"
	"github.com/golang/protobuf/ptypes"
	"github.com/streamingfast/blockmeta"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

var APIs []string

func init() {
	blockmeta.BlockNumToIDFromAPI = blockNumToIDFromAPI
	blockmeta.GetHeadInfoFromAPI = headInfoFromAPI
	blockmeta.GetIrrIDFromAPI = IrrIDFromAPI
}

func IrrIDFromAPI(ctx context.Context, blockNum uint64, libNum uint64) (string, error) {
	zlog.Info("getting irr id from api", zap.Uint64("block_num", blockNum), zap.Uint64("lib_num", libNum))
	if libNum == 0 {
		return blockNumToIDFromAPI(ctx, blockNum-1)
	}
	return blockNumToIDFromAPI(ctx, libNum)
}

func blockNumToIDFromAPI(ctx context.Context, blockNum uint64) (string, error) {
	zlog.Debug("getBlockForNum", zap.Uint64("block_num", blockNum))
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if len(APIs) == 0 {
		return "", nil
	}

	respChan := make(chan string)
	errChan := make(chan error)
	for _, addr := range APIs {
		go func(addr string) {
			hexBlockNum := fmt.Sprintf("0x%x", blockNum)
			zlog.Debug("calling getBlockForNum", zap.String("hex_block_num", hexBlockNum), zap.String("api_address", addr))
			blk, err := getBlockByNum(hexBlockNum, addr)
			if err != nil || blk == nil {
				select {
				case errChan <- err:
				case <-ctx.Done():
				}
				return
			}

			select {
			case respChan <- blk.Hash:
			case <-ctx.Done():
			}
		}(addr)
	}
	var errors []error
	for {
		if len(errors) == len(APIs) {
			return "", fmt.Errorf("all Ethereum API calls failed with errors: %v", errors)
		}
		select {
		case resp := <-respChan:
			return resp, nil
		case err := <-errChan:
			errors = append(errors, err)
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

func headInfoFromAPI(ctx context.Context) (*pbheadinfo.HeadInfoResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	respChan := make(chan *pbheadinfo.HeadInfoResponse)
	errChan := make(chan error)
	zlog.Debug("head info from api", zap.Int("api_address_count", len(APIs)))
	for _, addr := range APIs {
		go func(addr string) {
			block, err := getBlockByNum("latest", addr)
			if err != nil {
				select {
				case errChan <- err:
				case <-ctx.Done():
				}
				return
			}
			zlog.Debug("called api for block data", zap.String("api_address", addr), zap.Reflect("block_data", block))

			resp, err := headInfo(ctx, block)
			if err != nil {
				select {
				case errChan <- err:
				case <-ctx.Done():
				}
				return
			}

			zlog.Debug("returning head info from api", zap.String("api_address", addr), zap.Reflect("head_info", resp))
			select {
			case respChan <- resp:
			case <-ctx.Done():
			}

		}(addr)
	}
	var errors []error
	for {
		if len(errors) == len(APIs) {
			return nil, fmt.Errorf("all APIs [%d] failed with errors: %v", len(APIs), errors)
		}
		select {
		case resp := <-respChan:
			return resp, nil
		case err := <-errChan:
			errors = append(errors, err)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func headInfo(ctx context.Context, block *Block) (*pbheadinfo.HeadInfoResponse, error) {
	headTimestamp, err := ptypes.TimestampProto(block.Timestamp)
	if err != nil {
		return nil, err
	}

	libNum := uint64(0)
	if block.Number > 200 {
		libNum = block.Number - 200
	}
	libId, err := blockNumToIDFromAPI(ctx, libNum)
	if err != nil {
		return nil, err
	}
	headInfo := &pbheadinfo.HeadInfoResponse{
		LibNum:   libNum,
		LibID:    libId,
		HeadNum:  block.Number,
		HeadID:   block.Hash,
		HeadTime: headTimestamp,
	}

	return headInfo, nil
}

type Block struct {
	Number     uint64
	Hash       string
	ParentHash string
	Timestamp  time.Time
}

func getBlockByNum(num string, apiAddress string) (*Block, error) {
	rawRequest := `{"jsonrpc": "2.0", "method": "eth_getBlockByNumber", "params": ["%s",true], "id": 1}`

	request := fmt.Sprintf(rawRequest, num)
	resp, err := http.Post(apiAddress, "application/json", strings.NewReader(request))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http error [%d]", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	cleanedNum := strings.Replace(gjson.GetBytes(body, "result.number").String(), "0x", "", -1)
	blockNum, _ := strconv.ParseUint(cleanedNum, 16, 64)
	cleanedTime := strings.Replace(gjson.GetBytes(body, "result.timestamp").String(), "0x", "", -1)
	epoc, _ := strconv.ParseUint(cleanedTime, 16, 64)

	b := &Block{
		Number:     blockNum,
		Hash:       strings.TrimPrefix(gjson.GetBytes(body, "result.hash").String(), "0x"),
		ParentHash: strings.TrimPrefix(gjson.GetBytes(body, "result.parentHash").String(), "0x"),
		Timestamp:  time.Unix(int64(epoc), 0),
	}
	zlog.Debug("get block for num", zap.Reflect("block", b))
	return b, nil
}
func getBlockByID(id string, apiAddress string) (*Block, error) {
	rawRequest := `{"jsonrpc": "2.0", "method": "eth_getBlockByHash", "params": ["%s",true], "id": 1}`

	if !strings.HasPrefix(id, "0x") {
		id = "0x" + id
	}
	request := fmt.Sprintf(rawRequest, id)
	resp, err := http.Post(apiAddress, "application/json", strings.NewReader(request))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http error [%d]", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	cleanedNum := strings.Replace(gjson.GetBytes(body, "result.number").String(), "0x", "", -1)
	blockNum, _ := strconv.ParseUint(cleanedNum, 16, 64)
	cleanedTime := strings.Replace(gjson.GetBytes(body, "result.timestamp").String(), "0x", "", -1)
	epoc, _ := strconv.ParseUint(cleanedTime, 16, 64)

	b := &Block{
		Number:     blockNum,
		Hash:       strings.TrimPrefix(gjson.GetBytes(body, "result.hash").String(), "0x"),
		ParentHash: strings.TrimPrefix(gjson.GetBytes(body, "result.parentHash").String(), "0x"),
		Timestamp:  time.Unix(int64(epoc), 0),
	}
	zlog.Debug("get block for num", zap.Reflect("block", b))
	return b, nil
}
