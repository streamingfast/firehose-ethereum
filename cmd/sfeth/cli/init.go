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
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{Use: "init", Short: "Initializes local environment", RunE: sfInitE}

func init() {
	RootCmd.AddCommand(initCmd)
}

func sfInitE(cmd *cobra.Command, args []string) (err error) {
	cmd.SilenceUsage = true
	return fmt.Errorf("disabled init command")

	//	configFile := viper.GetString("global-config-file")
	//	zlog.Debug("starting init", zap.String("config-file", configFile))
	//
	//	if !viper.GetBool("skip-checks") {
	//		checkGethVersionOrExit()
	//	}
	//
	//	runProducer, err := askProducer()
	//	if err != nil {
	//		return err
	//	}
	//
	//	var networkID, enodesList string
	//	var secondsBetweenBlocks uint32
	//	if runProducer {
	//		enodesList = "miner"
	//		secondsBetweenBlocks, err = askSecondsBetweenBlocks()
	//		if err != nil {
	//			return err
	//		}
	//
	//	} else {
	//		network, other, err := askKnownNetworks()
	//		if err != nil {
	//			return err
	//		}
	//
	//		if other {
	//			// TODO: make this `mainnet` eventually, 123 is meaningless.
	//			networkID = fmt.Sprintf("%d", DefaultNetworkID)
	//
	//			networkID, err = askNetworkID(networkID)
	//			if err != nil {
	//				return err
	//			}
	//
	//			enodesList, err = askEnodesList()
	//			if err != nil {
	//				return err
	//			}
	//
	//		} else {
	//			switch network {
	//			case "Mainnet":
	//				networkID = "1"
	//			case "Ropsten":
	//				networkID = "3"
	//			case "Rinkeby":
	//				networkID = "4"
	//			case "Görli":
	//				networkID = "5"
	//			default:
	//				return fmt.Errorf("unknown network selected %q", network)
	//			}
	//		}
	//	}
	//
	//	sfDataDir, err := sfAbsoluteDataDir()
	//	if err != nil {
	//		return err
	//	}
	//
	//	currentDir, err := os.Getwd()
	//	if err != nil {
	//		log.Println(err)
	//	}
	//
	//	chainDataFolder := viper.GetString("global-node-data-folder") // this does not exist anymore
	//	paths := bootstrap.GetPaths(currentDir, sfDataDir, chainDataFolder)
	//
	//	err = Init(runProducer, networkID, enodesList, secondsBetweenBlocks, configFile, paths)
	//	if err != nil {
	//		return err
	//	}
	//
	//	var msg = `
	//Initialization completed
	//`
	//	if runProducer {
	//		msg += `
	//The password to unlock the initial account is: 'secure'
	//`
	//		msg += `To start your environment, run:
	//
	//  sfeth start
	//`
	//	} else {
	//		msg += fmt.Sprintf(`To start your environment,
	//`)
	//		if !knownNetworks[networkID] {
	//			msg += fmt.Sprintf(`
	//*  Copy your genesis.json to %s
	//`, paths.MindreaderConfigDir)
	//		}
	//		msg += `* Run:
	//
	//  sfeth start
	//`
	//	}
	//
	//	zlog.Info(msg)
	//
	//	return nil
}

var knownNetworks = map[string]bool{
	"1": true,
	"3": true,
	"4": true,
	"5": true,
}

//func Init(runMiner bool, networkID string, enodesList string, secondsBetweenBlocks uint32, configFile string, paths *bootstrap.Paths) (err error) {
//	toRun := []string{"all", "-peering-node", "-mindreader-node-stdin", "-tokenmeta", "-evm-executor"}
//	if !runMiner {
//		toRun = append(toRun, "-miner-node")
//	}
//
//	apps := launcher.ParseAppsFromArgs(toRun, func(s string) bool {
//		return true
//	})
//	conf := make(map[string]*launcher.DfuseCommandConfig)
//	conf["start"] = &launcher.DfuseCommandConfig{
//		Args: apps,
//		Flags: map[string]string{
//			"common-network-id":                      networkID,
//			"miner-node-extra-arguments":             "--mine --nodiscover --allow-insecure-unlock --unlock=0x62d9ad344366c268c3062764bbf67d318ec2c8fd --password=/dev/null",
//			"mindreader-node-bootstrap-static-nodes": enodesList,
//			"peering-node-extra-arguments":           "--mind=false --miner.threads=0",
//		},
//	}
//
//	if runMiner {
//		if err := os.MkdirAll(paths.MinerConfigDir, 0755); err != nil {
//			return fmt.Errorf("mkdir miner config dir: %w", err)
//		}
//
//		if err := os.MkdirAll(paths.MinerCfgKeystoreDir, 0755); err != nil {
//			return fmt.Errorf("mkdir miner keystore dir: %w", err)
//		}
//
//		zlog.Info(fmt.Sprintf("writing: %s", paths.MinerCfgNodekeyFilePath_))
//		if err = ioutil.WriteFile(paths.MinerCfgNodekeyFilePath, []byte(bootstrap.NodekeyFile), 0600); err != nil {
//			return fmt.Errorf("writing %s file: %w", paths.MinerCfgGenesisFilePath, err)
//		}
//
//		zlog.Info(fmt.Sprintf("writing: %s", paths.MinerCfgGenesisFilePath))
//		if err = ioutil.WriteFile(paths.MinerCfgGenesisFilePath, []byte(fmt.Sprintf(bootstrap.GenesisDotJson, secondsBetweenBlocks)), 0600); err != nil {
//			return fmt.Errorf("writing %s file: %w", paths.MinerCfgGenesisFilePath, err)
//		}
//
//		keystoreFilePath := filepath.Join(paths.MinerCfgKeystoreDir, "miner-private-key-62d9ad344366c268c3062764bbf67d318ec2c8fd")
//		zlog.Info(fmt.Sprintf("writing: %s", keystoreFilePath))
//		if err = ioutil.WriteFile(keystoreFilePath, []byte(bootstrap.KeystoreJSON), 0600); err != nil {
//			return fmt.Errorf("writing %s file: %w", keystoreFilePath, err)
//		}
//
//		zlog.Info("mining account created")
//	}
//
//	if err := os.MkdirAll(paths.MindreaderConfigDir, 0755); err != nil {
//		return fmt.Errorf("mkdir producer: %s", err)
//	}
//
//	if runMiner {
//		zlog.Info(fmt.Sprintf("writing: %s", paths.MindreaderCfgStaticNodesFilePath))
//		if err = ioutil.WriteFile(paths.MindreaderCfgStaticNodesFilePath, []byte(bootstrap.StaticNodesDotJSON), 0600); err != nil {
//			return fmt.Errorf("writing %s file: %s", paths.MinerCfgGenesisFilePath, err)
//		}
//
//		zlog.Info(fmt.Sprintf("writing: %s", paths.MindreaderCfgGenesisFilePath))
//		if err = ioutil.WriteFile(paths.MindreaderCfgGenesisFilePath, []byte(fmt.Sprintf(bootstrap.GenesisDotJson, secondsBetweenBlocks)), 0600); err != nil {
//			return fmt.Errorf("writing %s file: %s", paths.MindreaderCfgGenesisFilePath, err)
//		}
//	}
//
//	configBytes, err := yaml.Marshal(conf)
//	if err != nil {
//		return err
//	}
//
//	zlog.Info(fmt.Sprintf("writing config '%s'", strings.TrimPrefix(configFile, "./")))
//	if err = ioutil.WriteFile(configFile, configBytes, 0644); err != nil {
//		return fmt.Errorf("writing config file %s: %w", configFile, err)
//	}
//
//	return nil
//}

func askSecondsBetweenBlocks() (uint32, error) {
	validate := func(input string) error {
		v, err := strconv.ParseUint(input, 10, 32)
		if err != nil {
			return fmt.Errorf("cannot parse uint32: %w", err)
		}
		if v < 1 {
			return fmt.Errorf("cannot set lower than 1")
		}
		return nil
	}

	prompt := promptui.Prompt{
		Label:    "Enter desired number of seconds between blocks (minimum: 1)",
		Validate: validate,
		Default:  "1",
	}

	result, err := prompt.Run()
	time.Sleep(100 * time.Millisecond) // ensure prompt is ok
	if err != nil {
		return 0, err
	}

	res, err := strconv.ParseUint(result, 10, 32)
	return uint32(res), err
}

func askNetworkID(defaultNetworkID string) (string, error) {

	validate := func(input string) error {
		_, err := strconv.ParseUint(input, 10, 64)
		if err != nil {
			return fmt.Errorf("cannot parse network_id: %w", err)
		}
		return nil
	}

	prompt := promptui.Prompt{
		Label:    "Enter network ID",
		Validate: validate,
		Default:  defaultNetworkID,
	}

	result, err := prompt.Run()
	time.Sleep(100 * time.Millisecond) // ensure prompt is ok
	if err != nil {
		return "", err
	}

	return result, nil
}

func askEnodesList() (string, error) {
	var enodes []string
	for {
		enode, err := askEnode(len(enodes) == 0)
		if err != nil {
			return "", fmt.Errorf("failed to request enodes: %w", err)
		}
		if enode == "" {
			break
		}
		enodes = append(enodes, enode)
	}

	return strings.Join(enodes, ","), nil

}

func askEnode(first bool) (string, error) {
	validate := func(input string) error {
		if !first && input == "" {
			return nil
		}
		if !strings.HasPrefix(input, "enode://") || len(input) < (8+128) || !strings.Contains(input, "@") || !strings.Contains(input[len(input)-6:], ":") {
			return errors.New("Invalid enode URL (should start with enode://, followed by 128 hexadecimal characters, then '@hostname:port')")
		}
		if strings.Contains(input, "l") {
			return errors.New("Invalid enode URL (cannot contain a comma ',')")
		}
		return nil
	}

	label := "First enode to connect (ex: enode://123be...@127.0.0.1:1234)"
	if !first {
		label = "Add another enode? (leave blank to skip)"
	}

	prompt := promptui.Prompt{
		Label:    label,
		Validate: validate,
	}

	result, err := prompt.Run()
	time.Sleep(100 * time.Millisecond) // ensure prompt is ok
	if err != nil {
		return "", err
	}

	return result, nil
}

func askProducer() (bool, error) {
	zlog.Info(`ethereum on StreamingFast - can run a local test node configured for local mining`)
	zlog.Info(`alternatively, Ethereum on StreamingFast can connect to an already existing network`)

	prompt := promptui.Prompt{
		Label:     "Do you want to setup a local miner",
		IsConfirm: true,
	}

	result, err := prompt.Run()
	time.Sleep(100 * time.Millisecond) // ensure prompt is ok
	if err != nil && err != promptui.ErrAbort {
		return false, fmt.Errorf("unable to ask if we run producing node: %w", err)
	}

	fmt.Println("")
	return strings.ToLower(result) == "y", nil
}

func askKnownNetworks() (string, bool, error) {
	zlog.Info(`ethereum on StreamingFast can connect you to a known network directly`)
	zlog.Info(`select 'Other' if you want to specify the network peers and ID manually`)

	prompt := promptui.Select{
		Label: "Choose known network",
		Items: []interface{}{
			"Mainnet",
			"Ropsten",
			"Rinkeby",
			"Görli",
			"Other",
		},
	}

	index, value, err := prompt.Run()
	time.Sleep(100 * time.Millisecond) // ensure prompt is ok
	if err != nil && err != promptui.ErrAbort {
		return "", false, fmt.Errorf("prompt ui failed: %w", err)
	}

	fmt.Println("")
	return value, index == 4, nil
}

func runCmd(cmd *exec.Cmd) (error, string) {
	// This runs (and wait) the command, combines both stdout and stderr in a single stream and return everything
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil, ""
	}

	return err, string(out)
}
