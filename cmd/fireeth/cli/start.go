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

package cli

import (
	"context"
	"fmt"
	"github.com/streamingfast/dmetering"
	tracing "github.com/streamingfast/sf-tracing"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/derr"
	"github.com/streamingfast/dlauncher/launcher"
	_ "github.com/streamingfast/firehose-ethereum/types"
	"go.uber.org/zap"
)

var StartCmd = &cobra.Command{Use: "start", Short: "Starts Ethereum on StreamingFast services all at once", RunE: sfStartE, Args: cobra.ArbitraryArgs}

func init() {
	RootCmd.AddCommand(StartCmd)
}

func sfStartE(cmd *cobra.Command, args []string) (err error) {
	cmd.SilenceUsage = true

	dataDir := viper.GetString("global-data-dir")
	zlog.Debug("fireeth binary started", zap.String("data_dir", dataDir))

	configFile := viper.GetString("global-config-file")
	zlog.Info(fmt.Sprintf("starting with config file '%s'", configFile))

	if err := Start(cmd.Context(), dataDir, args); err != nil {
		return err
	}

	zlog.Info("goodbye")
	return nil
}

func Start(ctx context.Context, dataDir string, args []string) (err error) {
	dataDirAbs, err := filepath.Abs(dataDir)
	if err != nil {
		return fmt.Errorf("unable to setup directory structure: %w", err)
	}

	err = makeDirs([]string{dataDirAbs})
	if err != nil {
		return err
	}

	bstream.GetProtocolFirstStreamableBlock = uint64(viper.GetInt("common-first-streamable-block"))
	modules := &launcher.Runtime{
		AbsDataDir:              dataDirAbs,
		ProtocolSpecificModules: map[string]interface{}{},
	}

	blocksCacheEnabled := viper.GetBool("common-blocks-cache-enabled")
	if blocksCacheEnabled {
		bstream.GetBlockPayloadSetter = bstream.ATMCachedPayloadSetter

		cacheDir := MustReplaceDataDir(modules.AbsDataDir, viper.GetString("common-blocks-cache-dir"))
		storeUrl := MustReplaceDataDir(modules.AbsDataDir, viper.GetString("common-merged-blocks-store-url"))
		maxRecentEntryBytes := viper.GetInt("common-blocks-cache-max-recent-entry-bytes")
		maxEntryByAgeBytes := viper.GetInt("common-blocks-cache-max-entry-by-age-bytes")
		bstream.InitCache(storeUrl, cacheDir, maxRecentEntryBytes, maxEntryByAgeBytes)
	}

	if registerCommonModulesCallback != nil {
		zlog.Debug("invoking register common modules callback since it's set")
		if err := registerCommonModulesCallback(modules); err != nil {
			return fmt.Errorf("register common modules: %w", err)
		}
	}

	err = bstream.ValidateRegistry()
	if err != nil {
		return fmt.Errorf("protocol specific hooks not configured correctly: %w", err)
	}

	// FIXME: That should be a shared dependencies across `Ethereum on StreamingFast`, it will avoid the need to call `dmetering.SetDefaultMeter`
	metering, err := dmetering.New(viper.GetString("common-metering-plugin"))
	if err != nil {
		return fmt.Errorf("unable to initialize dmetering: %w", err)
	}
	dmetering.SetDefaultMeter(metering)

	launch := launcher.NewLauncher(zlog, modules)
	zlog.Debug("launcher created")
	runByDefault := func(app string) bool {
		switch app {
		case "reader-node-stdin":
			return false
		}
		return true
	}

	apps := launcher.ParseAppsFromArgs(args, runByDefault)
	if len(args) == 0 && launcher.Config["start"] != nil {
		apps = launcher.ParseAppsFromArgs(launcher.Config["start"].Args, runByDefault)
	}
	if err := setupTracing(ctx, apps); err != nil {
		return fmt.Errorf("failed to setup tracing: %w", err)
	}

	zlog.Info(fmt.Sprintf("launching applications: %s", strings.Join(apps, ",")))
	if err = launch.Launch(apps); err != nil {
		return err
	}

	printWelcomeMessage(apps)

	signalHandler := derr.SetupSignalHandler(viper.GetDuration(CommonSystemShutdownSignalDelayFlag))
	select {
	case <-signalHandler:
		zlog.Info("received termination signal, quitting")
		go launch.Close()
	case appID := <-launch.Terminating():
		if launch.Err() == nil {
			zlog.Info(fmt.Sprintf("application %s triggered a clean shutdown, quitting", appID))
		} else {
			zlog.Info(fmt.Sprintf("application %s shutdown unexpectedly, quitting", appID))
			err = launch.Err()
		}
	}

	launch.WaitForTermination()
	dmetering.WaitToFlush()

	return
}

func setupTracing(ctx context.Context, apps []string) error {
	serviceName := "fireeth"
	if len(apps) == 1 {
		serviceName = serviceName + "/" + apps[0]
	}
	return tracing.SetupOpenTelemetry(ctx, serviceName)
}

func printWelcomeMessage(apps []string) {
	hasDashboard := containsApp(apps, "dashboard")
	hasAPIProxy := containsApp(apps, "apiproxy")
	if !hasDashboard && !hasAPIProxy {
		// No welcome message to print, advanced usage
		return
	}

	zlog.Info("your instance should be ready in a few seconds")
}

func containsApp(apps []string, searchedApp string) bool {
	for _, app := range apps {
		if app == searchedApp {
			return true
		}
	}

	return false
}
