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
	"archive/tar"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/streamingfast/dstore"
	"go.uber.org/zap"
)

//GenesisBootstrapper needs to write genesis file, static node file, then run a command like 'geth init'
type GenesisBootstrapper struct {
	dataDir        string
	genesisFileURL string
	cmdArgs        []string
	nodePath       string
	//	staticNodesFilepath string
	logger *zap.Logger
}

func NewGenesisBootstrapper(dataDir string, genesisFileURL string, nodePath string, cmdArgs []string, logger *zap.Logger) *GenesisBootstrapper {
	return &GenesisBootstrapper{
		dataDir:        dataDir,
		genesisFileURL: genesisFileURL,
		nodePath:       nodePath,
		cmdArgs:        cmdArgs,
		logger:         logger,
	}
}

func downloadDstoreObject(url string, destpath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	reader, _, _, err := dstore.OpenObject(ctx, url)
	if err != nil {
		return fmt.Errorf("cannot get file from store: %w", err)
	}
	defer reader.Close()
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(destpath, data, 0644)
}

func (b *GenesisBootstrapper) Bootstrap() error {
	if b.genesisFileURL == "" || isBootstrapped(b.dataDir, b.logger) {
		return nil
	}

	genesisFilePath := filepath.Join(b.dataDir, "genesis.json")

	b.logger.Info("running bootstrap sequence", zap.String("data_dir", b.dataDir), zap.String("genesis_file_path", genesisFilePath))
	if err := os.MkdirAll(b.dataDir, 0755); err != nil {
		return fmt.Errorf("cannot create folder %s to bootstrap node: %w", b.dataDir, err)
	}

	if !fileExists(genesisFilePath) {
		b.logger.Info("fetching genesis file", zap.String("source_url", b.genesisFileURL))
		if err := downloadDstoreObject(b.genesisFileURL, genesisFilePath); err != nil {
			return err
		}
	}

	cmd := exec.Command(b.nodePath, b.cmdArgs...)
	b.logger.Info("running node init command (creating genesis block from genesis.json)", zap.Stringer("cmd", cmd))
	if output, err := runCmd(cmd); err != nil {
		return fmt.Errorf("failed to init node (output %s): %w", output, err)
	}

	return nil
}

func NewTarballBootstrapper(
	url string,
	dataDir string,
	logger *zap.Logger,
) *TarballBootstrapper {
	return &TarballBootstrapper{
		url:     url,
		dataDir: dataDir,
		logger:  logger,
	}
}

type TarballBootstrapper struct {
	url     string
	dataDir string
	logger  *zap.Logger
}

func isBootstrapped(dataDir string, logger *zap.Logger) bool {
	var foundCURRENT bool
	err := filepath.Walk(dataDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if filepath.Base(path) == "CURRENT" {
				foundCURRENT = true
				return io.EOF
			}
			return nil
		})
	if err != nil && !os.IsNotExist(err) && err != io.EOF {
		logger.Warn("error while checking for bootstrapped status", zap.Error(err))
	}

	return foundCURRENT
}

func (b *TarballBootstrapper) isBootstrapped() bool {
	return isBootstrapped(b.dataDir, b.logger)
}

func (b *TarballBootstrapper) Bootstrap() error {
	if b.isBootstrapped() {
		return nil
	}

	b.logger.Info("bootstrapping geth chain data from pre-built data", zap.String("bootstrap_data_url", b.url))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	reader, _, _, err := dstore.OpenObject(ctx, b.url, dstore.Compression("zstd"))
	if err != nil {
		return fmt.Errorf("cannot get snapshot from gstore: %w", err)
	}
	defer reader.Close()

	b.createChainData(reader)
	return nil
}

func (b *TarballBootstrapper) createChainData(reader io.Reader) error {
	err := os.MkdirAll(b.dataDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("unable to create blocks log file: %w", err)
	}

	b.logger.Info("extracting bootstrapping data into node data directory", zap.String("data_dir", b.dataDir))
	tr := tar.NewReader(reader)
	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}

			return err
		}

		path := filepath.Join(b.dataDir, header.Name)
		b.logger.Debug("about to write content of entry", zap.String("name", header.Name), zap.String("path", path), zap.Bool("is_dir", header.FileInfo().IsDir()))
		if header.FileInfo().IsDir() {
			err = os.MkdirAll(path, os.ModePerm)
			if err != nil {
				return fmt.Errorf("unable to create directory: %w", err)
			}

			continue
		}

		file, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("unable to create file: %w", err)
		}

		if _, err := io.Copy(file, tr); err != nil {
			file.Close()
			return err
		}
		file.Close()
	}
}

func dirExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
func runCmd(cmd *exec.Cmd) (string, error) {
	// This runs (and wait) the command, combines both stdout and stderr in a single stream and return everything
	out, err := cmd.CombinedOutput()
	if err == nil {
		return "", nil
	}

	return string(out), err
}
