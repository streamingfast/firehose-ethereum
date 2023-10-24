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

package dev

import (
	"regexp"
	"strings"

	"github.com/ShinyTrinkets/overseer"
	nodemanager "github.com/streamingfast/firehose-ethereum/node-manager"
	nodeManager "github.com/streamingfast/node-manager"
	"github.com/streamingfast/node-manager/superviser"
	"go.uber.org/zap"
)

var enodeRegexp = regexp.MustCompile(`enode://([a-f0-9]*)@.*$`)

type Superviser struct {
	*nodemanager.Superviser
	command             string
	headBlockUpdateFunc nodeManager.HeadBlockUpdater
	lastBlockSeen       uint64
}

func (s *Superviser) GetName() string {
	return "geth"
}

func NewSuperviser(
	binary string,
	arguments []string,
	headBlockUpdateFunc nodeManager.HeadBlockUpdater,
	appLogger *zap.Logger,
	nodeLogger *zap.Logger,
) (*Superviser, error) {
	// Ensure process manager line buffer is large enough (50 MiB) for our Deep Mind instrumentation outputting lot's of text.
	overseer.DEFAULT_LINE_BUFFER_SIZE = 50 * 1024 * 1024

	gethSuperviser := &Superviser{
		Superviser: &nodemanager.Superviser{
			Superviser: superviser.New(appLogger, binary, arguments),
			Logger:     appLogger,
		},
		command:             binary + " " + strings.Join(arguments, " "),
		headBlockUpdateFunc: headBlockUpdateFunc,
	}

	gethSuperviser.RegisterLogPlugin(nodemanager.NewGethToZapLogPlugin(false, nodeLogger))

	return gethSuperviser, nil
}

func (s *Superviser) GetCommand() string {
	return s.command
}

func (s *Superviser) LastSeenBlockNum() uint64 {
	return s.lastBlockSeen
}

func (s *Superviser) ServerID() (string, error) {
	return "dev", nil
}

//
//func (s *Superviser) ServerID() (string, error) {
//	return "dev", nil
//}
//
//func (s *Superviser) UpdateLastBlockSeen(blockNum uint64) {
//	s.lastBlockSeen = blockNum
//}

//func (s *Superviser) lastBlockSeenLogPlugin(line string) {
//	switch {
//	case strings.HasPrefix(line, "DMLOG FINALIZE_BLOCK"):
//		line = strings.TrimSpace(strings.TrimPrefix(line, "DMLOG FINALIZE_BLOCK"))
//	case strings.HasPrefix(line, "FIRE FINALIZE_BLOCK"):
//		line = strings.TrimSpace(strings.TrimPrefix(line, "FIRE FINALIZE_BLOCK"))
//	default:
//		return
//	}
//
//	blockNum, err := strconv.ParseUint(line, 10, 64)
//	if err != nil {
//		s.Logger.Error("unable to extract last block num", zap.String("line", line), zap.Error(err))
//		return
//	}
//
//	//metrics.SetHeadBlockNumber(blockNum)
//	s.lastBlockSeen = blockNum
//}
