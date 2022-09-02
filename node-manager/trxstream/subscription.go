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

package trxstream

import (
	"sync"

	pbtrxstream "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/trxstream/v1"
	"go.uber.org/zap"
)

func newSubscription(chanSize int, logger *zap.Logger) (out *subscription) {
	return &subscription{
		incomingTrx: make(chan *pbtrxstream.Transaction, chanSize),
		logger:      logger,
	}
}

type subscription struct {
	incomingTrx chan *pbtrxstream.Transaction
	closed      bool
	quitOnce    sync.Once
	logger      *zap.Logger
}

func (s *subscription) Push(trx *pbtrxstream.Transaction) {
	if len(s.incomingTrx) == cap(s.incomingTrx) {
		s.quitOnce.Do(func() {
			s.logger.Info("reach max buffer size for subscription, closing channel")
			s.closed = true
			close(s.incomingTrx)
		})
		return
	}

	if s.closed {
		s.logger.Warn("received trx in a close subscription")
		return
	}

	s.logger.Debug("subscription writing accepted block", zap.Int("capacity", cap(s.incomingTrx)))
	s.incomingTrx <- trx
}
