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

package trxdb_loader

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/streamingfast/bstream"
	"github.com/streamingfast/dmetrics"
	"github.com/streamingfast/dstore"
	"github.com/streamingfast/sf-ethereum/trxdb"
	trxdbloader "github.com/streamingfast/sf-ethereum/trxdb-loader"
	"github.com/streamingfast/sf-ethereum/trxdb-loader/metrics"
	"github.com/streamingfast/shutter"
	"go.uber.org/zap"
)

type Config struct {
	ProcessingType        string // The actual processing type to perform, either `live`, `batch` or `patch`
	BlockStoreURL         string // GS path to read batch files from
	BlockStreamAddr       string // [LIVE] Address of grpc endpoint
	KvdbDsn               string // Storage connection string
	BatchSize             uint64 // DB batch size
	StartBlockNum         uint64 // [BATCH] Block number where we start processing
	StopBlockNum          uint64 // [BATCH] Block number where we stop processing
	AllowLiveOnEmptyTable bool   // [LIVE] force pipeline creation if live request and table is empty
	HTTPListenAddr        string //  http listen address for /healthz endpoint
}

type App struct {
	*shutter.Shutter
	Config *Config
}

func New(config *Config) *App {
	return &App{
		Shutter: shutter.New(),
		Config:  config,
	}
}

func (a *App) Run() error {
	zlog.Info("launching trxdb-loader", zap.Reflect("config", a.Config))

	dmetrics.Register(metrics.Metricset)

	switch a.Config.ProcessingType {
	case "live", "batch", "patch":
	default:
		return fmt.Errorf("unknown processing-type value %q", a.Config.ProcessingType)
	}

	blocksStore, err := dstore.NewDBinStore(a.Config.BlockStoreURL)
	if err != nil {
		return fmt.Errorf("setting up archive store: %w", err)
	}
	var loader trxdbloader.Loader

	db, err := trxdb.New(a.Config.KvdbDsn)
	if err != nil {
		return fmt.Errorf("unable to create trxdb: %w", err)
	}
	// FIXME: make sure we call CLOSE() at the end!
	//defer db.Close()

	l := trxdbloader.NewBigtableLoader(a.Config.BlockStreamAddr, blocksStore, a.Config.BatchSize, db, 2)

	loader = l

	healthzHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !loader.Healthy() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}

		w.Write([]byte("ready\n"))
	})

	errorLogger, err := zap.NewStdLogAt(zlog, zap.ErrorLevel)
	if err != nil {
		return fmt.Errorf("unable to create error logger: %w", err)
	}

	httpSrv := &http.Server{
		Addr:         a.Config.HTTPListenAddr,
		Handler:      healthzHandler,
		ReadTimeout:  10 * time.Millisecond,
		WriteTimeout: 10 * time.Millisecond,
		ErrorLog:     errorLogger,
	}
	zlog.Info("starting webserver", zap.String("http_addr", a.Config.HTTPListenAddr))
	go httpSrv.ListenAndServe()

	switch a.Config.ProcessingType {
	case "live":
		err := loader.BuildPipelineLive(a.Config.AllowLiveOnEmptyTable)
		if err != nil {
			return err
		}
	case "batch":
		loader.StopBeforeBlock(uint64(a.Config.StopBlockNum))
		loader.BuildPipelineBatch(uint64(a.Config.StartBlockNum), bstream.DumbStartBlockResolver(201))
	case "patch":
		loader.StopBeforeBlock(uint64(a.Config.StopBlockNum))
		loader.BuildPipelinePatch(uint64(a.Config.StartBlockNum), bstream.DumbStartBlockResolver(201))
	}

	a.OnTerminating(loader.Shutdown)
	loader.OnTerminated(a.Shutdown)

	go loader.Launch()
	return nil
}

func (a *App) IsReady() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	url := fmt.Sprintf("http://%s/healthz", a.Config.HTTPListenAddr)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		zlog.Warn("is ready request building error", zap.Error(err))
		return false
	}
	client := http.DefaultClient
	res, err := client.Do(req)
	if err != nil {
		zlog.Debug("is ready request execution error", zap.Error(err))
		return false
	}

	if res.StatusCode == 200 {
		return true
	}
	return false
}
