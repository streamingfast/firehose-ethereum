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

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/bstream/blockstream"
	"github.com/streamingfast/dlauncher/launcher"
	"github.com/streamingfast/firehose-ethereum/codec"
	nodemanager "github.com/streamingfast/firehose-ethereum/node-manager"
	"github.com/streamingfast/firehose-ethereum/node-manager/dev"
	"github.com/streamingfast/firehose-ethereum/node-manager/geth"
	"github.com/streamingfast/firehose-ethereum/node-manager/openeth"
	"github.com/streamingfast/logging"
	nodeManager "github.com/streamingfast/node-manager"
	nodeManagerApp "github.com/streamingfast/node-manager/app/node_manager2"
	"github.com/streamingfast/node-manager/metrics"
	reader "github.com/streamingfast/node-manager/mindreader"
	"github.com/streamingfast/node-manager/operator"
	pbbstream "github.com/streamingfast/pbgo/sf/bstream/v1"
	pbheadinfo "github.com/streamingfast/pbgo/sf/headinfo/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var nodeLogger, nodeTracer = logging.PackageLogger("node", "github.com/streamingfast/firehose-ethereum/node")
var nodeGethLogger, _ = logging.PackageLogger("node.geth", "github.com/streamingfast/firehose-ethereum/node/geth", DefaultLevelInfo)
var nodeOpenEthereumLogger, _ = logging.PackageLogger("node.openethereum", "github.com/streamingfast/firehose-ethereum/node/open-ethereum", DefaultLevelInfo)

var readerLogger, readerTracer = logging.PackageLogger("reader", "github.com/streamingfast/firehose-ethereum/mindreader")
var readerGethLogger, _ = logging.PackageLogger("reader.geth", "github.com/streamingfast/firehose-ethereum/mindreader/geth", DefaultLevelInfo)
var readerOpenEthereumLogger, _ = logging.PackageLogger("reader.open-ethereum", "github.com/streamingfast/firehose-ethereum/mindreader/open-ethereum", DefaultLevelInfo)

func registerNodeApp(backupModuleFactories map[string]operator.BackupModuleFactory) {
	launcher.RegisterApp(zlog, &launcher.AppDef{
		ID:            "node",
		Title:         "Ethereum Node",
		Description:   "Ethereum node with built-in operational manager",
		RegisterFlags: registerEthereumNodeFlags,
		InitFunc: func(runtime *launcher.Runtime) error {
			return nil
		},
		FactoryFunc: nodeFactoryFunc(false, backupModuleFactories)})
}

func nodeFactoryFunc(isReader bool, backupModuleFactories map[string]operator.BackupModuleFactory) func(*launcher.Runtime) (launcher.App, error) {
	return func(runtime *launcher.Runtime) (launcher.App, error) {
		appLogger := nodeLogger
		appTracer := nodeTracer
		if isReader {
			appLogger = readerLogger
			appTracer = readerTracer
		}

		sfDataDir := runtime.AbsDataDir

		nodePath,
			networkID,
			nodeType,
			nodeDataDir,
			nodeIPCPath,
			debugDeepMind,
			logToZap,
			httpAddr,
			readinessMaxLatency,
			nodeEnforcePeers,
			bootstrapDataURL,
			backupConfigs,
			shutdownDelay,
			nodeArguments,
			err := parseCommonNodeFlags(appLogger, sfDataDir, isReader)
		if err != nil {
			return nil, err
		}

		prefix := "node"
		if isReader {
			prefix = "reader-node"
		}

		metricsAndReadinessManager := buildMetricsAndReadinessManager(prefix, readinessMaxLatency)
		nodeLogger := getSupervisedProcessLogger(isReader, nodeType)

		superviser, err := buildSuperviser(
			metricsAndReadinessManager,
			nodeType,
			nodePath,
			nodeIPCPath,
			nodeDataDir,
			nodeArguments,
			nodeEnforcePeers,
			appLogger,
			nodeLogger,
			logToZap,
			isReader,
			debugDeepMind,
		)
		if err != nil {
			return nil, err
		}

		var bootstrapper operator.Bootstrapper
		if bootstrapDataURL != "" {
			if nodeType != "geth" {
				return nil, fmt.Errorf("feature bootstrap-data-url only supported for node type geth")
			}

			switch {
			case strings.HasSuffix(bootstrapDataURL, "tar.zst") || strings.HasSuffix(bootstrapDataURL, "tar.zstd"):
				bootstrapper = geth.NewTarballBootstrapper(bootstrapDataURL, nodeDataDir, nodeLogger)
			case strings.HasSuffix(bootstrapDataURL, "json"):
				// special bootstrap case
				bootstrapArgs, err := buildNodeArguments(appLogger, sfDataDir, networkID, nodeDataDir, nodeIPCPath, "", nodeType, "bootstrap", "")
				if err != nil {
					return nil, fmt.Errorf("cannot build node bootstrap arguments")
				}
				bootstrapper = geth.NewGenesisBootstrapper(nodeDataDir, bootstrapDataURL, nodePath, bootstrapArgs, nodeLogger)
			default:
				return nil, fmt.Errorf("bootstrap-data-url should point to either an archive ending in '.tar.zstd' or a genesis file ending in '.json', not %s", bootstrapDataURL)
			}
		}

		chainOperator, err := buildChainOperator(
			superviser,
			metricsAndReadinessManager,
			shutdownDelay,
			bootstrapper,
			appLogger,
		)
		if err != nil {
			return nil, err
		}

		backupModules, backupSchedules, err := operator.ParseBackupConfigs(appLogger, backupConfigs, backupModuleFactories)
		if err != nil {
			return nil, fmt.Errorf("parsing backup configs: %w", err)
		}

		appLogger.Info("backup config",
			zap.Strings("config", backupConfigs),
			zap.Int("backup_module_count", len(backupModules)),
			zap.Int("backup_schedule_count", len(backupSchedules)),
		)

		for name, mod := range backupModules {
			appLogger.Info("registering backup module", zap.String("name", name), zap.Any("module", mod))
			err := chainOperator.RegisterBackupModule(name, mod)
			if err != nil {
				return nil, fmt.Errorf("unable to register backup module %s: %w", name, err)
			}
			appLogger.Info("backup module registered", zap.String("name", name), zap.Any("module", mod))
		}

		for _, sched := range backupSchedules {
			chainOperator.RegisterBackupSchedule(sched)
		}

		if !isReader {
			return nodeManagerApp.New(&nodeManagerApp.Config{
				HTTPAddr: httpAddr,
			}, &nodeManagerApp.Modules{
				Operator:                   chainOperator,
				MetricsAndReadinessManager: metricsAndReadinessManager,
			}, appLogger), nil
		}

		blockStreamServer := blockstream.NewUnmanagedServer(
			blockstream.ServerOptionWithLogger(appLogger),
			blockstream.ServerOptionWithBuffer(1),
		)
		gprcListenAddr := viper.GetString("reader-node-grpc-listen-addr")
		_, oneBlocksStoreURL, _ := mustGetCommonStoresURLs(runtime.AbsDataDir)
		workingDir := MustReplaceDataDir(sfDataDir, viper.GetString("reader-node-working-dir"))
		batchStartBlockNum := viper.GetUint64("reader-node-start-block-num")
		batchStopBlockNum := viper.GetUint64("reader-node-stop-block-num")
		oneBlockFileSuffix := viper.GetString("reader-node-oneblock-suffix")
		blocksChanCapacity := viper.GetInt("reader-node-blocks-chan-capacity")

		updateMetricsAndSuperviser := func(block *bstream.Block) error {
			if s, ok := superviser.(UpdatableSuperviser); ok {
				s.UpdateLastBlockSeen(block.Number)
			}
			return metricsAndReadinessManager.UpdateHeadBlock(block)
		}

		readerPlugin, err := reader.NewMindReaderPlugin(
			oneBlocksStoreURL,
			workingDir,
			func(lines chan string) (reader.ConsolerReader, error) {
				return codec.NewConsoleReader(appLogger, lines)
			},
			batchStartBlockNum,
			batchStopBlockNum,
			blocksChanCapacity,
			updateMetricsAndSuperviser,
			func(error) {
				chainOperator.Shutdown(nil)
			},
			oneBlockFileSuffix,
			blockStreamServer,
			appLogger,
			appTracer,
		)
		if err != nil {
			return nil, fmt.Errorf("new reader plugin: %w", err)
		}

		superviser.RegisterLogPlugin(readerPlugin)

		trxPoolLogPlugin := nodemanager.NewTrxPoolLogPlugin(appLogger)
		superviser.RegisterLogPlugin(trxPoolLogPlugin)

		return nodeManagerApp.New(&nodeManagerApp.Config{
			HTTPAddr: httpAddr,
			GRPCAddr: gprcListenAddr,
		}, &nodeManagerApp.Modules{
			Operator:                   chainOperator,
			MindreaderPlugin:           readerPlugin,
			MetricsAndReadinessManager: metricsAndReadinessManager,
			RegisterGRPCService: func(server grpc.ServiceRegistrar) error {
				pbheadinfo.RegisterHeadInfoServer(server, blockStreamServer)
				pbbstream.RegisterBlockStreamServer(server, blockStreamServer)

				trxPoolLogPlugin.RegisterServices(server)

				return nil
			},
		}, appLogger), nil
	}
}

func isGenesisBootstrapper(bootstrapDataURL string) bool {
	return strings.HasSuffix(bootstrapDataURL, "json")
}

func getSupervisedProcessLogger(isReader bool, nodeType string) *zap.Logger {
	switch nodeType {
	case "geth", "dev":
		if isReader {
			return readerGethLogger
		} else {
			return nodeGethLogger
		}
	case "openethereum":
		if isReader {
			return readerOpenEthereumLogger
		} else {
			return nodeOpenEthereumLogger
		}
	default:
		panic(fmt.Errorf("unknown node type %q, only knows about %q, %q and %q", nodeType, "geth", "openethereum", "dev"))
	}
}

type nodeArgsByRole map[string]string

var nodeArgsByTypeAndRole = map[string]nodeArgsByRole{
	"geth": {
		"dev-miner": "--networkid={network-id} --datadir={node-data-dir} --ipcpath={node-ipc-path} --port=" + NodeP2PPort + " --http --http.api=eth,net,web3,personal --http.port=" + NodeRPCPort + " --http.addr=0.0.0.0 --http.vhosts=* --mine --nodiscover --allow-insecure-unlock --password=/dev/null --miner.etherbase=" + devMinerAddress + " --unlock=" + devMinerAddress,
		"peering":   "--networkid={network-id} --datadir={node-data-dir} --ipcpath={node-ipc-path} --port=30304 --http --http.api=eth,net,web3 --http.port=8546 --http.addr=0.0.0.0 --http.vhosts=* --firehose-block-progress",
		"reader":    "--networkid={network-id} --datadir={node-data-dir} --ipcpath={node-ipc-path} --port=" + ReaderNodeP2PPort + " --http --http.api=eth,net,web3 --http.port=" + ReaderNodeRPCPort + " --http.addr=0.0.0.0 --http.vhosts=* --firehose-enabled",
		"bootstrap": "--networkid={network-id} --datadir={node-data-dir} --maxpeers 10 init {node-data-dir}/genesis.json",
	},
	"openethereum": {
		"peering": "--network-id={network-id} --ipc-path={node-ipc-path} --base-path={node-data-dir} --port=" + NodeP2PPort + " --jsonrpc-port=" + NodeRPCPort + " --jsonrpc-apis=web3,net,eth,parity,parity,parity_pubsub,parity_accounts,parity_set --firehose-block-progress",
		//"dev-miner": ...
		"reader": "--network-id={network-id} --ipc-path={node-ipc-path} --base-path={node-data-dir} --port=" + ReaderNodeP2PPort + " --jsonrpc-port=" + ReaderNodeRPCPort + " --jsonrpc-apis=web3,net,eth,parity,parity,parity_pubsub,parity_accounts,parity_set --firehose-enabled --no-warp",
	},
	"dev": {
		"reader": "tools poll-rpc-blocks http://localhost:8545 0",
	},
}

func registerEthereumNodeFlags(cmd *cobra.Command) error {
	registerCommonNodeFlags(cmd, false)
	cmd.Flags().String("node-role", "peering", "Sets default node arguments. accepted values: peering, dev-miner. See `fireeth help nodes` for more info")
	return nil
}

// flags common to reader and regular node
func registerCommonNodeFlags(cmd *cobra.Command, isReader bool) {
	prefix := "node-"
	managerAPIAddr := NodeManagerAPIAddr
	if isReader {
		prefix = "reader-node-"
		managerAPIAddr = ReaderNodeManagerAPIAddr
	}

	cmd.Flags().String(prefix+"path", "geth", "command that will be launched by the node manager (ignored on type 'dev')")
	cmd.Flags().String(prefix+"type", "dev", "one of: ['dev', 'geth','openethereum']")
	cmd.Flags().String(prefix+"arguments", "", "If not empty, overrides the list of default node arguments (computed from node type and role). Start with '+' to append to default args instead of replacing. You can use the {public-ip} token, that will be matched against space-separated hostname:IP pairs in PUBLIC_IPS env var, taking hostname from HOSTNAME env var.")
	cmd.Flags().String(prefix+"data-dir", "{sf-data-dir}/{node-role}/data", "Directory for node data ({node-role} is either reader, peering or dev-miner)")
	cmd.Flags().String(prefix+"ipc-path", "{sf-data-dir}/{node-role}/ipc", "IPC path cannot be more than 64chars on geth")

	cmd.Flags().String(prefix+"manager-api-addr", managerAPIAddr, "Ethereum node manager API address")
	cmd.Flags().Duration(prefix+"readiness-max-latency", 30*time.Second, "Determine the maximum head block latency at which the instance will be determined healthy. Some chains have more regular block production than others.")

	cmd.Flags().String(prefix+"bootstrap-data-url", "", "URL (file or gs) to either a genesis.json file or a .tar.zst archive to decompress in the datadir. Only used when bootstrapping (no prior data)")
	cmd.Flags().String(prefix+"enforce-peers", "", "Comma-separated list of operator nodes that will be queried for an 'enode' value and added as a peer")

	cmd.Flags().StringSlice(prefix+"backups", []string{}, "Repeatable, space-separated key=values definitions for backups. Example: 'type=gke-pvc-snapshot prefix= tag=v1 freq-blocks=1000 freq-time= project=myproj archive=true'")

	cmd.Flags().Bool(prefix+"log-to-zap", true, "Enable all node logs to transit into node's logger directly, when false, prints node logs directly to stdout")
	cmd.Flags().Bool(prefix+"debug-firehose-logs", false, "[DEV] Prints firehose instrumentation logs to standard output, should be use for debugging purposes only")
}

func parseCommonNodeFlags(appLogger *zap.Logger, sfDataDir string, isReader bool) (
	nodePath string,
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
	if isReader {
		prefix = "reader-node-"
		nodeRole = "reader"
	}

	nodePath = viper.GetString(prefix + "path")
	networkID = fmt.Sprintf("%d", viper.GetUint32("common-network-id"))
	nodeType = viper.GetString(prefix + "type")
	nodeDataDir = replaceNodeRole(nodeRole,
		MustReplaceDataDir(sfDataDir, viper.GetString(prefix+"data-dir")))
	nodeIPCPath = replaceNodeRole(nodeRole,
		MustReplaceDataDir(sfDataDir, viper.GetString(prefix+"ipc-path")))
	debugDeepMind = viper.GetBool(prefix + "debug-firehose-logs")
	logToZap = viper.GetBool(prefix + "log-to-zap")
	managerAPIAddress = viper.GetString(prefix + "manager-api-addr")
	readinessMaxLatency = viper.GetDuration(prefix + "readiness-max-latency")
	nodeEnforcePeers = viper.GetString(prefix + "enforce-peers")
	bootstrapDataURL = viper.GetString(prefix + "bootstrap-data-url")
	backupConfigs = viper.GetStringSlice(prefix + "backups")
	shutdownDelay = viper.GetDuration(CommonSystemShutdownSignalDelayFlag) // we reuse this global value

	nodeArguments, err = buildNodeArguments(
		appLogger,
		sfDataDir,
		networkID,
		nodeDataDir,
		nodeIPCPath,
		viper.GetString(prefix+"arguments"),
		nodeType,
		nodeRole,
		bootstrapDataURL,
	)

	return
}

func buildNodeArguments(appLogger *zap.Logger, dataDir, networkID, nodeDataDir, nodeIPCPath, providedArgs, nodeType, nodeRole, bootstrapDataURL string) ([]string, error) {
	zlog.Info("building node arguments", zap.String("node-type", nodeType), zap.String("node-role", nodeRole))
	typeRoles, ok := nodeArgsByTypeAndRole[nodeType]
	if !ok {
		return nil, fmt.Errorf("invalid node type: %s", nodeType)
	}

	args, ok := typeRoles[nodeRole]
	if !ok {
		return nil, fmt.Errorf("invalid node role: %s for type %s", nodeRole, nodeType)
	}

	// This sets `--firehose-genesis-file` if the node role is of type reader
	// (for which case we are sure that Firehose patch is supported) and if the bootstrap data
	// url is a `genesis.json` file.
	if nodeRole == "reader" && isGenesisBootstrapper(bootstrapDataURL) {
		args += fmt.Sprintf(" --firehose-genesis-file=%s", bootstrapDataURL)
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
	args = strings.Replace(args, "{data-dir}", dataDir, -1)

	if strings.Contains(args, "{public-ip}") {
		var foundPublicIP string
		hostname := os.Getenv("HOSTNAME")
		publicIPs := os.Getenv("PUBLIC_IPS") // format is PUBLIC_IPS="reader-v3-1:1.2.3.4 backup-node:5.6.7.8"
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
			appLogger.Warn("cannot find public IP in environment variable PUBLIC_IPS (format: 'HOSTNAME:a.b.c.d HOSTNAME2:e.f.g.h'), using 127.0.0.1 as fallback", zap.String("hostname", hostname))
			foundPublicIP = "127.0.0.1"
		}
		args = strings.Replace(args, "{public-ip}", foundPublicIP, -1)
	}

	return strings.Fields(args), nil
}

type UpdatableSuperviser interface {
	UpdateLastBlockSeen(blockNum uint64)
}

func buildSuperviser(
	metricsAndReadinessManager *nodeManager.MetricsAndReadinessManager,
	nodeType string,
	nodePath string,
	nodeIPCPath string,
	nodeDataDir string,
	nodeArguments []string,
	enforcedPeers string,
	appLogger,
	supervisedProcessLogger *zap.Logger,
	logToZap,
	deepMind,
	debugDeepMind bool,
) (nodeManager.ChainSuperviser, error) {

	switch nodeType {
	case "dev":
		superviser, err := dev.NewSuperviser(
			nodePath,
			nodeArguments,
			metricsAndReadinessManager.UpdateHeadBlock,
			appLogger,
			supervisedProcessLogger,
		)
		if err != nil {
			return nil, fmt.Errorf("unable to create chain superviser: %w", err)
		}

		return superviser, nil

	case "geth":
		superviser, err := geth.NewGethSuperviser(
			nodePath,
			nodeDataDir,
			nodeIPCPath,
			nodeArguments,
			deepMind,
			debugDeepMind,
			metricsAndReadinessManager.UpdateHeadBlock,
			enforcedPeers,
			logToZap,
			appLogger,
			supervisedProcessLogger,
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
			deepMind,
			debugDeepMind,
			metricsAndReadinessManager.UpdateHeadBlock,
			enforcedPeers,
			logToZap,
			appLogger, nodeGethLogger,
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
	appReadiness := metrics.NewAppReadiness(name)

	metricsAndReadinessManager := nodeManager.NewMetricsAndReadinessManager(
		headBlockTimeDrift,
		headBlockNumber,
		appReadiness,
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

func replaceNodeRole(nodeRole, in string) string {
	return strings.Replace(in, "{node-role}", nodeRole, -1)
}
