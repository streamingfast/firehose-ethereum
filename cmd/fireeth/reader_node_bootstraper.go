package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/streamingfast/cli"
	"github.com/streamingfast/cli/sflags"
	"github.com/streamingfast/dstore"
	firecore "github.com/streamingfast/firehose-core"
	"github.com/streamingfast/firehose-core/node-manager/operator"
	"go.uber.org/zap"
)

func newReaderNodeBootstrapper(_ context.Context, logger *zap.Logger, cmd *cobra.Command, resolvedNodeArguments []string, resolver firecore.ReaderNodeArgumentResolver) (operator.Bootstrapper, error) {
	bootstrapDataURL := sflags.MustGetString(cmd, "reader-node-bootstrap-data-url")
	if bootstrapDataURL == "" {
		return nil, nil
	}

	switch {
	case strings.HasSuffix(bootstrapDataURL, "json"):
		var args []string
		if dataDirArgument := findDataDirArgument(resolvedNodeArguments); dataDirArgument != "" {
			args = append(args, dataDirArgument)
		}

		return NewGenesisBootstrapper(resolver("{node-data-dir}"), bootstrapDataURL, sflags.MustGetString(cmd, "reader-node-path"), append(args, "init"), logger), nil

	default:
		// There is a default handler for some base cases, so we need to return nil so the default handler can take over
		// and handle the case(s).
		return nil, nil
	}
}

func findDataDirArgument(resolvedNodeArguments []string) string {
	for i, arg := range resolvedNodeArguments {
		if strings.HasPrefix(arg, "--datadir") {
			// If the argument is in 2 parts (e.g. [--datadir, <value>]), we try to re-combine them
			if arg == "--datadir" {
				if len(resolvedNodeArguments) > i+1 {
					return "--datadir=" + resolvedNodeArguments[i+1]
				}

				// The arguments are invalid, we'll let the node fail later on
				return arg
			}

			return arg
		}
	}

	return ""
}

// GenesisBootstrapper needs to write genesis file, static node file, then run a command like 'geth init'
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
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	return os.WriteFile(destpath, data, 0644)
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

	if !cli.FileExists(genesisFilePath) {
		b.logger.Info("fetching genesis file", zap.String("source_url", b.genesisFileURL))
		if err := downloadDstoreObject(b.genesisFileURL, genesisFilePath); err != nil {
			return err
		}
	}

	cmd := exec.Command(b.nodePath, append(b.cmdArgs, genesisFilePath)...)
	b.logger.Info("running node init command (creating genesis block from genesis.json)", zap.Stringer("cmd", cmd))
	if output, err := runCmd(cmd); err != nil {
		return fmt.Errorf("failed to init node (output %s): %w", output, err)
	}

	return nil
}

func runCmd(cmd *exec.Cmd) (string, error) {
	// This runs (and wait) the command, combines both stdout and stderr in a single stream and return everything
	out, err := cmd.CombinedOutput()
	if err == nil {
		return "", nil
	}

	return string(out), err
}

func isBootstrapped(dataDir string, logger *zap.Logger) bool {
	var foundFile bool
	err := filepath.Walk(dataDir,
		func(_ string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}

			// As soon as there is a file, we assume it's bootstrapped
			foundFile = true
			return io.EOF
		})
	if err != nil && !os.IsNotExist(err) && err != io.EOF {
		logger.Warn("error while checking for bootstrapped status", zap.Error(err))
	}

	return foundFile
}
