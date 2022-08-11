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

package geth

import (
	"fmt"
	"time"

	"github.com/streamingfast/bstream"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// Monitor periodically checks the head block num and block time, as well as the enode string (server ID)
func (s *Superviser) Monitor() {
	started := time.Now()
	for {
		time.Sleep(2 * time.Second)

		if !s.IsRunning() {
			continue
		}
		resp, err := s.sendGethCommand(`{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}`)
		if err != nil {
			s.Logger.Warn("geth Monitor cannot get info from IPC socket", zap.Error(err))
			if time.Since(started) < time.Minute {
				continue
			}
		} else {
			s.setEnodeStr(gjson.Get(resp, "result.enode").String())
		}

		resp, err = s.sendGethCommand(`{"jsonrpc":"2.0","method":"admin_peers","params":[],"id":1}`)
		if err != nil {
			s.Logger.Warn("geth Monitor cannot get peers from IPC socket", zap.Error(err))
		} else {
			connectedPeers := []string{}
			for _, peer := range gjson.Get(resp, "result").Array() {
				connectedPeers = append(connectedPeers, peer.Get("enode").String())
			}
			s.infoMutex.Lock()
			s.connectedPeers = connectedPeers
			s.infoMutex.Unlock()
		}

		if s.headBlockUpdateFunc != nil {
			resp, err = s.sendGethCommand(`{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`)
			if err != nil {
				s.Logger.Warn("geth Monitor cannot get blocknumber from IPC socket", zap.Error(err))
				continue
			}
			lastBlock := gjson.Get(resp, "result")
			lastBlockNum := hex2uint(lastBlock.String())
			if lastBlockNum == 0 {
				continue
			}

			resp, err = s.sendGethCommand(fmt.Sprintf(`{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["%s", true],"id":1}`, lastBlock))
			if err != nil {
				s.Logger.Warn("geth Monitor cannot get block by number", zap.Error(err))
				continue
			}
			timestamp := time.Unix(hex2int(gjson.Get(resp, "result.timestamp").String()), 0)
			hash := hex2string(gjson.Get(resp, "result.hash").String())

			s.headBlockUpdateFunc(&bstream.Block{
				Id:        hash,
				Number:    uint64(lastBlockNum),
				Timestamp: timestamp,
			})
		}

	}
}
