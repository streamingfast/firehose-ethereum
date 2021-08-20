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
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/streamingfast/bstream"
	"github.com/streamingfast/dgrpc"
	nodeManagerApp "github.com/streamingfast/node-manager/app/node_manager"
	nodeMindReaderApp "github.com/streamingfast/node-manager/app/node_mindreader"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/streamingfast/dlauncher/flags"
	"github.com/streamingfast/dlauncher/launcher"
	"github.com/streamingfast/logging"
	nodeManager "github.com/streamingfast/node-manager"
	"github.com/streamingfast/node-manager/metrics"
	"github.com/streamingfast/node-manager/operator"
	nodemanager "github.com/streamingfast/sf-ethereum/node-manager"
	"github.com/streamingfast/sf-ethereum/node-manager/geth"
	"github.com/streamingfast/sf-ethereum/node-manager/openeth"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func init() {
	appLogger := zap.NewNop()
	nodeLogger := zap.NewNop()

	logging.Register("github.com/streamingfast/sf-ethereum/node", &appLogger)
	logging.Register("github.com/streamingfast/sf-ethereum/node/node", &nodeLogger)

	launcher.RegisterApp(&launcher.AppDef{
		ID:          "node",
		Title:       "Ethereum Node",
		Description: "Ethereum node with built-in operational manager",
		MetricsID:   "node",
		Logger: launcher.NewLoggingDef(
			"github.com/streamingfast/sf-ethereum/node.*",
			[]zapcore.Level{zap.WarnLevel, zap.WarnLevel, zap.InfoLevel, zap.DebugLevel},
		),
		RegisterFlags: registerEthereumNodeFlags,
		InitFunc: func(runtime *launcher.Runtime) error {
			return nil
		},
		FactoryFunc: nodeFactoryFunc(false, &appLogger, &nodeLogger)})
}

func nodeFactoryFunc(isMindreader bool, appLogger, nodeLogger **zap.Logger) func(*launcher.Runtime) (launcher.App, error) {
	return func(runtime *launcher.Runtime) (launcher.App, error) {
		sfDataDir := runtime.AbsDataDir

		nodePath,
			blockmetaAddr,
			networkID,
			nodeType,
			nodeDataDir,
			nodeIPCPath,
			debugDeepMind,
			logToZap,
			managerAPIAddress,
			readinessMaxLatency,
			nodeEnforcePeers,
			bootstrapDataURL,
			backupConfigs,
			shutdownDelay,
			nodeArguments,
			err := parseCommonNodeFlags(sfDataDir, isMindreader)
		if err != nil {
			return nil, err
		}

		prefix := "node"
		if isMindreader {
			prefix = "mindreader-node"
		}
		metricsAndReadinessManager := buildMetricsAndReadinessManager(prefix, readinessMaxLatency)

		superviser, err := buildSuperviser(
			metricsAndReadinessManager,
			nodeType,
			networkID,
			nodePath,
			nodeIPCPath,
			nodeDataDir,
			nodeArguments,
			nodeEnforcePeers,

			*appLogger, *nodeLogger, logToZap, debugDeepMind,
		)
		if err != nil {
			return nil, err
		}

		tracker := runtime.Tracker.Clone()
		tracker.AddGetter(bstream.NetworkLIBTarget, bstream.NetworkLIBBlockRefGetter(blockmetaAddr))

		var bootstrapper operator.Bootstrapper
		if bootstrapDataURL != "" {
			if nodeType != "geth" && nodeType != "lachesis" {
				return nil, fmt.Errorf("feature bootstrap-data-url only supported for node type geth or lachesis")
			}

			switch {
			case strings.HasSuffix(bootstrapDataURL, "tar.zst") || strings.HasSuffix(bootstrapDataURL, "tar.zstd"):
				bootstrapper = geth.NewTarballBootstrapper(bootstrapDataURL, nodeDataDir, *nodeLogger)
			case strings.HasSuffix(bootstrapDataURL, "json"):
				// special bootstrap case
				bootstrapArgs, err := buildNodeArguments(networkID, nodeDataDir, nodeIPCPath, "", nodeType, "bootstrap")
				if err != nil {
					return nil, fmt.Errorf("cannot build node bootstrap arguments")
				}
				bootstrapper = geth.NewGenesisBootstrapper(nodeDataDir, bootstrapDataURL, nodePath, bootstrapArgs, *nodeLogger)
			default:
				return nil, fmt.Errorf("bootstrap-data-url should point to either an archive ending in '.tar.zstd' or a genesis file ending in '.json', not %s", bootstrapDataURL)
			}

		}

		chainOperator, err := buildChainOperator(
			superviser,
			metricsAndReadinessManager,
			shutdownDelay,
			bootstrapper,
			*appLogger,
		)
		if err != nil {
			return nil, err
		}

		backupModules, backupSchedules, err := parseBackupConfigs(backupConfigs)
		if err != nil {
			return nil, fmt.Errorf("parsing backup configs: %w", err)
		}
		zlog.Info("backup config", zap.Any("config", backupConfigs), zap.Int("backup module num", len(backupModules)), zap.Int("backup schedule num", len(backupSchedules)))

		for name, mod := range backupModules {
			zlog.Info("registering backup module", zap.String("name", name), zap.Any("module", mod))
			err := chainOperator.RegisterBackupModule(name, mod)
			if err != nil {
				return nil, fmt.Errorf("unable to register backup module %s: %w", name, err)
			}
			zlog.Info("backup module registered", zap.String("name", name), zap.Any("module", mod))
		}

		for _, sched := range backupSchedules {
			chainOperator.RegisterBackupSchedule(sched)
		}

		if !isMindreader {
			return nodeManagerApp.New(&nodeManagerApp.Config{
				ManagerAPIAddress: managerAPIAddress,
			}, &nodeManagerApp.Modules{
				Operator:                   chainOperator,
				MetricsAndReadinessManager: metricsAndReadinessManager,
			}, *appLogger), nil
		} else {
			GRPCAddr := viper.GetString("mindreader-node-grpc-listen-addr")
			oneBlockStoreURL := MustReplaceDataDir(sfDataDir, viper.GetString("common-oneblock-store-url"))
			mergedBlockStoreURL := MustReplaceDataDir(sfDataDir, viper.GetString("common-blocks-store-url"))
			workingDir := MustReplaceDataDir(sfDataDir, viper.GetString("mindreader-node-working-dir"))
			mergeAndStoreDirectly := viper.GetBool("mindreader-node-merge-and-store-directly")
			mergeThresholdBlockAge := viper.GetDuration("mindreader-node-merge-threshold-block-age")
			batchStartBlockNum := viper.GetUint64("mindreader-node-start-block-num")
			batchStopBlockNum := viper.GetUint64("mindreader-node-stop-block-num")
			failOnNonContiguousBlock := false //FIXME ?
			waitTimeForUploadOnShutdown := viper.GetDuration("mindreader-node-wait-upload-complete-on-shutdown")
			oneBlockFileSuffix := viper.GetString("mindreader-node-oneblock-suffix")
			blocksChanCapacity := viper.GetInt("mindreader-node-blocks-chan-capacity")
			gs := dgrpc.NewServer(dgrpc.WithLogger(*appLogger))

			mindreaderPlugin, err := getMindreaderLogPlugin(
				oneBlockStoreURL,
				mergedBlockStoreURL,
				workingDir,
				mergeAndStoreDirectly,
				mergeThresholdBlockAge,
				batchStartBlockNum,
				batchStopBlockNum,
				failOnNonContiguousBlock,
				waitTimeForUploadOnShutdown,
				oneBlockFileSuffix,
				blocksChanCapacity,
				chainOperator.Shutdown,
				metricsAndReadinessManager,
				tracker,
				gs,
				*appLogger,
			)
			if err != nil {
				return nil, err
			}

			superviser.RegisterLogPlugin(mindreaderPlugin)

			trxPoolLogPlugin := nodemanager.NewTrxPoolLogPlugin(*appLogger)
			superviser.RegisterLogPlugin(trxPoolLogPlugin)
			trxPoolLogPlugin.RegisterServices(gs)

			return nodeMindReaderApp.New(&nodeMindReaderApp.Config{
				ManagerAPIAddress: managerAPIAddress,
				GRPCAddr:          GRPCAddr,
			}, &nodeMindReaderApp.Modules{
				Operator:                   chainOperator,
				MetricsAndReadinessManager: metricsAndReadinessManager,
				GrpcServer:                 gs,
			}, *appLogger), nil

		}
	}
}

type nodeArgsByRole map[string]string

var nodeArgsByTypeAndRole = map[string]nodeArgsByRole{
	"geth": {
		"dev-miner":  "--networkid={network-id} --datadir={node-data-dir} --ipcpath={node-ipc-path} --port=" + NodeP2PPort + " --rpc --rpcapi=admin,debug,eth,net,web3,personal --rpcport=" + NodeRPCPort + " --rpcaddr=0.0.0.0 --rpcvhosts=* --nousb --mine --nodiscover --allow-insecure-unlock --password=/dev/null --miner.etherbase=" + devMinerAddress + " --unlock=" + devMinerAddress,
		"peering":    "--networkid={network-id} --datadir={node-data-dir} --ipcpath={node-ipc-path} --port=30304 --rpc --rpcapi=admin,debug,eth,net,web3 --rpcport=8546 --rpcaddr=0.0.0.0 --rpcvhosts=* --nousb --firehose-deep-mind-block-progress",
		"mindreader": "--networkid={network-id} --datadir={node-data-dir} --ipcpath={node-ipc-path} --port=" + MindreaderNodeP2PPort + " --rpc --rpcapi=admin,debug,eth,net,web3 --rpcport=" + MindreaderNodeRPCPort + " --rpcaddr=0.0.0.0 --rpcvhosts=* --nousb --firehose-deep-mind",
		"bootstrap":  "--networkid={network-id} --datadir={node-data-dir} --maxpeers 10 init {node-data-dir}/genesis.json",
	},
	"lachesis": {
		"peering":    "--networkid={network-id} --ipcpath={node-ipc-path} --datadir={node-data-dir} --port=30304 --rpc --rpcapi=admin,debug,eth,net,web3 --rpcport=8546 --rpcaddr=0.0.0.0 --rpcvhosts=* --nousb --firehose-deep-mind-block-progress --config /config/config.toml",
		"mindreader": "--networkid={network-id} --ipcpath={node-ipc-path} --datadir={node-data-dir} --port=" + MindreaderNodeP2PPort + " --rpc --rpcapi personal,eth,net,web3,debug,admin --rpcport " + MindreaderNodeRPCPort + " --rpcaddr 0.0.0.0 --rpcvhosts * --nousb --firehose-deep-mind --config /config/config.toml",
		"bootstrap":  "--networkid={network-id} --datadir={node-data-dir} --maxpeers 10 init {node-data-dir}/genesis.json",
	},
	"openethereum": {
		"peering": "--network-id={network-id} --ipc-path={node-ipc-path} --base-path={node-data-dir} --port=" + NodeP2PPort + " --jsonrpc-port=" + NodeRPCPort + " --jsonrpc-apis=debug,web3,net,eth,parity,parity,parity_pubsub,parity_accounts,parity_set --firehose-deep-mind-block-progress",
		//"dev-miner": ...
		"mindreader": "--network-id={network-id} --ipc-path={node-ipc-path} --base-path={node-data-dir} --port=" + MindreaderNodeP2PPort + " --jsonrpc-port=" + MindreaderNodeRPCPort + " --jsonrpc-apis=debug,web3,net,eth,parity,parity,parity_pubsub,parity_accounts,parity_set --firehose-deep-mind --no-warp",
	},
}

func registerEthereumNodeFlags(cmd *cobra.Command) error {
	registerCommonNodeFlags(cmd, false)
	cmd.Flags().String("node-role", "peering", "Sets default node arguments. accepted values: peering, dev-miner. See `sfeth help nodes` for more info")
	return nil
}

// flags common to mindreader and regular node
func registerCommonNodeFlags(cmd *cobra.Command, isMindreader bool) {
	prefix := "node-"
	managerAPIAddr := NodeManagerAPIAddr
	defaultEnforcedPeers := ""
	if isMindreader {
		prefix = "mindreader-node-"
		managerAPIAddr = MindreaderNodeManagerAPIAddr
		defaultEnforcedPeers = "localhost" + NodeManagerAPIAddr
	}

	cmd.Flags().String(prefix+"path", "geth", "command that will be launched by the node manager")
	cmd.Flags().String(prefix+"type", "geth", "one of: ['geth','lachesis','openethereum']")
	cmd.Flags().String(prefix+"arguments", "", "If not empty, overrides the list of default node arguments (computed from node type and role). Start with '+' to append to default args instead of replacing. You can use the {public-ip} token, that will be matched against space-separated hostname:IP pairs in PUBLIC_IPS env var, taking hostname from HOSTNAME env var.")
	cmd.Flags().String(prefix+"data-dir", "{sf-data-dir}/{node-role}/data", "Directory for node data ({node-role} is either mindreader, peering or dev-miner)")
	cmd.Flags().String(prefix+"ipc-path", "{sf-data-dir}/{node-role}/ipc", "IPC path cannot be more than 64chars on geth and lachesis")

	cmd.Flags().String(prefix+"manager-api-addr", managerAPIAddr, "Ethereum node manager API address")
	cmd.Flags().Duration(prefix+"readiness-max-latency", 30*time.Second, "Determine the maximum head block latency at which the instance will be determined healthy. Some chains have more regular block production than others.")

	cmd.Flags().String(prefix+"bootstrap-data-url", "", "URL (file or gs) to either a genesis.json file or a .tar.zst archive to decompress in the datadir. Only used when bootstrapping (no prior data)")
	cmd.Flags().String(prefix+"enforce-peers", defaultEnforcedPeers, "Comma-separated list of operator nodes that will be queried for an 'enode' value and added as a peer")

	cmd.Flags().StringSlice(prefix+"backups", []string{}, "Repeatable, space-separated key=values definitions for backups. Example: 'type=gke-pvc-snapshot prefix= tag=v1 freq-blocks=1000 freq-time= project=myproj'")

	cmd.Flags().Bool(prefix+"log-to-zap", true, "Enable all node logs to transit into node's logger directly, when false, prints node logs directly to stdout")
	cmd.Flags().Bool(prefix+"debug-deep-mind", false, "[DEV] Prints deep mind instrumentation logs to standard output, should be use for debugging purposes only")
}

func parseCommonNodeFlags(sfDataDir string, isMindreader bool) (
	nodePath string,
	blockmetaAddr string,
	networkID string,
	nodeType string,
	nodeDataDir string,
	nodeIPCPath string,
	debugDeepMind bool,
	logToZap bool,
	managerAPIAddress string,
	readinessMaxLatency time.Duration,
	nodeEnforcePeers string,
	bootstrapDataURL string,
	backupConfigs []string,
	shutdownDelay time.Duration,
	nodeArguments []string,
	err error,
) {
	prefix := "node-"
	nodeRole := viper.GetString("node-role")
	if isMindreader {
		prefix = "mindreader-node-"
		nodeRole = "mindreader"
	}

	nodePath = viper.GetString(prefix + "path")
	blockmetaAddr = viper.GetString("common-blockmeta-addr")
	networkID = fmt.Sprintf("%d", viper.GetUint32("common-network-id"))
	nodeType = viper.GetString(prefix + "type")
	nodeDataDir = replaceNodeRole(nodeRole,
		MustReplaceDataDir(sfDataDir, viper.GetString(prefix+"data-dir")))
	nodeIPCPath = replaceNodeRole(nodeRole,
		MustReplaceDataDir(sfDataDir, viper.GetString(prefix+"ipc-path")))
	debugDeepMind = viper.GetBool(prefix + "debug-deep-mind")
	logToZap = viper.GetBool(prefix + "log-to-zap")
	managerAPIAddress = viper.GetString(prefix + "manager-api-addr")
	readinessMaxLatency = viper.GetDuration(prefix + "readiness-max-latency")
	nodeEnforcePeers = viper.GetString(prefix + "enforce-peers")
	bootstrapDataURL = viper.GetString(prefix + "bootstrap-data-url")
	backupConfigs = viper.GetStringSlice(prefix + "backups")
	shutdownDelay = viper.GetDuration("common-system-shutdown-signal-delay") // we reuse this global value

	nodeArguments, err = buildNodeArguments(
		networkID,
		nodeDataDir,
		nodeIPCPath,
		viper.GetString(prefix+"arguments"),
		nodeType,
		nodeRole,
	)

	return
}

func buildNodeArguments(networkID, nodeDataDir, nodeIPCPath, providedArgs, nodeType, nodeRole string) ([]string, error) {
	typeRoles, ok := nodeArgsByTypeAndRole[nodeType]
	if !ok {
		return nil, fmt.Errorf("invalid node type: %s", nodeType)
	}

	args, ok := typeRoles[nodeRole]
	if !ok {
		return nil, fmt.Errorf("invalid node role: %s for type %s", nodeRole, nodeType)
	}

	if providedArgs != "" {
		if strings.HasPrefix(providedArgs, "+") {
			args += " " + strings.TrimLeft(providedArgs, "+")
		} else {
			args = providedArgs // discard info provided by node type / role
		}
	}

	args = strings.Replace(args, "{node-data-dir}", nodeDataDir, -1)
	args = strings.Replace(args, "{network-id}", networkID, -1)
	args = strings.Replace(args, "{node-ipc-path}", nodeIPCPath, -1)

	if strings.Contains(args, "{public-ip}") {
		var foundPublicIP string
		hostname := os.Getenv("HOSTNAME")
		publicIPs := os.Getenv("PUBLIC_IPS") // format is PUBLIC_IPS="mindreader-v3-1:1.2.3.4 backup-node:5.6.7.8"
		for _, pairStr := range strings.Fields(publicIPs) {
			pair := strings.Split(pairStr, ":")
			if len(pair) != 2 {
				continue
			}
			if pair[0] == hostname {
				foundPublicIP = pair[1]
			}
		}

		if foundPublicIP == "" {
			zlog.Warn("cannot find public IP in environment variable PUBLIC_IPS (format: 'HOSTNAME:a.b.c.d HOSTNAME2:e.f.g.h'), using 127.0.0.1 as fallback", zap.String("hostname", hostname))
			foundPublicIP = "127.0.0.1"
		}
		args = strings.Replace(args, "{public-ip}", foundPublicIP, -1)
	}

	return strings.Fields(args), nil
}

func buildSuperviser(
	metricsAndReadinessManager *nodeManager.MetricsAndReadinessManager,
	nodeType string,
	networkID string,
	nodePath string,
	nodeIPCPath string,
	nodeDataDir string,
	nodeArguments []string,
	enforcedPeers string,

	appLogger, gethLogger *zap.Logger,
	logToZap, debugDeepMind bool,
) (nodeManager.ChainSuperviser, error) {

	switch nodeType {
	case "geth", "lachesis":

		superviser, err := geth.NewGethSuperviser(
			nodePath,
			nodeDataDir,
			nodeIPCPath,
			nodeArguments,
			debugDeepMind,
			metricsAndReadinessManager.UpdateHeadBlock,
			enforcedPeers,
			logToZap,
			appLogger, gethLogger,
		)
		if err != nil {
			return nil, fmt.Errorf("unable to create chain superviser: %w", err)
		}

		return superviser, nil
	case "openethereum":
		superviser, err := openeth.NewSuperviser(
			nodePath,
			nodeDataDir,
			nodeIPCPath,
			nodeArguments,
			debugDeepMind,
			metricsAndReadinessManager.UpdateHeadBlock,
			enforcedPeers,
			logToZap,
			appLogger, gethLogger,
		)
		if err != nil {
			return nil, fmt.Errorf("unable to create chain superviser: %w", err)
		}

		return superviser, nil
	default:
		return nil, fmt.Errorf("unsupported node type: %s", nodeType)
	}
}

func buildMetricsAndReadinessManager(name string, maxLatency time.Duration) *nodeManager.MetricsAndReadinessManager {
	headBlockTimeDrift := metrics.NewHeadBlockTimeDrift(name)
	headBlockNumber := metrics.NewHeadBlockNumber(name)

	metricsAndReadinessManager := nodeManager.NewMetricsAndReadinessManager(
		headBlockTimeDrift,
		headBlockNumber,
		maxLatency,
	)
	return metricsAndReadinessManager
}

func buildChainOperator(
	superviser nodeManager.ChainSuperviser,
	metricsAndReadinessManager *nodeManager.MetricsAndReadinessManager,
	shutdownDelay time.Duration,
	bootstrapper operator.Bootstrapper,
	appLogger *zap.Logger,
) (*operator.Operator, error) {

	o, err := operator.New(
		appLogger,
		superviser,
		metricsAndReadinessManager,
		&operator.Options{
			ShutdownDelay:              shutdownDelay,
			EnableSupervisorMonitoring: true,
			Bootstrapper:               bootstrapper,
		})

	if err != nil {
		return nil, fmt.Errorf("unable to create chain operator: %w", err)
	}
	return o, nil
}

func parseBackupConfigs(backupConfigs []string) (mods map[string]operator.BackupModule, scheds []*operator.BackupSchedule, err error) {
	mods = make(map[string]operator.BackupModule)
	for _, confStr := range backupConfigs {
		conf, err := flags.ParseKVConfigString(confStr)
		if err != nil {
			return nil, nil, err
		}
		t := conf["type"]
		switch t {
		case "gke-pvc-snapshot":
			mod, err := nodemanager.NewGKEPVCSnapshotter(conf)
			if err != nil {
				return nil, nil, err
			}
			mods[t] = mod
		default:
			return nil, nil, fmt.Errorf("unknown backup module type: %s", t)
		}

		if conf["freq-blocks"] != "" || conf["freq-time"] != "" {
			newSched, err := operator.NewBackupSchedule(conf["freq-blocks"], conf["freq-time"], conf["required-hostname"], t)
			if err != nil {
				return nil, nil, fmt.Errorf("error setting up backup schedule for %s, %w", t, err)
			}

			scheds = append(scheds, newSched)
		}

	}
	return
}
