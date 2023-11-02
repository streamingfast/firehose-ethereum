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

package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bobg/go-generics/v2/slices"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/cli"
	. "github.com/streamingfast/cli"
	"github.com/streamingfast/cli/sflags"
	firecore "github.com/streamingfast/firehose-core"
	"github.com/streamingfast/logging"
	"github.com/streamingfast/shutter"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

func registerGethEnforcePeersCmd(parent *cobra.Command, binary string, logger *zap.Logger, tracer logging.Tracer) {
	registerGroup(parent,
		Group("geth", "Geth tools around peers management and some maintenance tasks",
			Command(createGethEnforcePeersE(logger, tracer),
				"enforce-peers",
				"Enforce provided peers to be connected to the node, this tools is meant to run as a sidecar to a Geth node",
				Flags(func(flags *pflag.FlagSet) {
					flags.BoolP("once", "o", false, "Run as a unit operation and avoid launching a long running process")
					flags.String("ipc-file-path", "", "Path to the IPC file to connect to")
					flags.StringArray("ensure-peers", nil, "List of peers to ensure connection to")
				}),
				ExamplePrefixed(fmt.Sprintf("%s tools geth enforce-peers", binary), `
					# Run as a long running process
					--ipc-file-path=/var/data/geth/ipc --ensure-peers=<hostname>[:<port>]

					# Run only once instead of launching a long running process
					--ipc-file-path=/var/data/geth/ipc --ensure-peers=<hostname>[:<port>] --once
				`),
			),
		),
	)
}

func createGethEnforcePeersE(logger *zap.Logger, tracer logging.Tracer) firecore.CommandExecutor {
	return func(cmd *cobra.Command, _ []string) error {
		node := &GethNode{
			ipcFilePath: sflags.MustGetString(cmd, "ipc-file-path"),
			logger:      logger,
		}

		cli.Ensure(node.ipcFilePath != "", "--ipc-file-path is required")

		monitor := &GethMonitor{
			Shutter: shutter.New(),
			node:    node,
			logger:  logger.Named("monitor"),
			tracer:  tracer,
		}

		peersEnforcer := &GethPeersEnforcer{
			Shutter:              shutter.New(),
			node:                 node,
			wantedPeersHostnames: sflags.MustGetStringArray(cmd, "ensure-peers"),
			wantedPeers:          map[string]*Enode{},
			logger:               logger.Named("enforcer"),
		}

		cli.Ensure(len(peersEnforcer.wantedPeersHostnames) > 0, "--ensure-peers is required")

		if sflags.MustGetBool(cmd, "once") {
			return runOnce(node, monitor, peersEnforcer)
		}

		return runDaemon(cmd.Context(), logger, monitor, peersEnforcer)
	}
}

func runOnce(node *GethNode, monitor *GethMonitor, enforcer *GethPeersEnforcer) error {
	if err := monitor.runOnce(); err != nil {
		return fmt.Errorf("cannot run monitor once: %w", err)
	}

	if err := enforcer.runOnce(); err != nil {
		return fmt.Errorf("cannot run peers enforcer once: %w", err)
	}

	// The second run of the monitor is to ensure that connected peers were recorded
	if err := monitor.runOnce(); err != nil {
		return fmt.Errorf("cannot run monitor once: %w", err)
	}

	fmt.Printf("Connected peers (%d)\n", len(node.connectedPeers))
	for _, peer := range node.connectedPeers {
		peerEnode, err := parseEnode(peer)
		if err != nil {
			fmt.Println("- " + peer)
			continue
		}

		suffix := ""
		if _, found := enforcer.wantedPeers[peerEnode.ID]; found {
			suffix = " (wanted)"
		}

		fmt.Println("- " + peer + suffix)
	}

	return nil
}

func runDaemon(ctx context.Context, logger *zap.Logger, monitor *GethMonitor, enforcer *GethPeersEnforcer) error {
	app := cli.NewApplication(ctx)

	logger.Info("starting Geth monitor")
	app.SuperviseAndStart(monitor)

	logger.Info("starting Geth peers enforcer")
	app.SuperviseAndStart(enforcer)

	logger.Info("waiting for termination signal")
	app.WaitForTermination(logger, 0, 0)

	return nil
}

var enodeRegexp = regexp.MustCompile(`enode://([a-f0-9]*)@.*$`)

type GethNode struct {
	ipcFilePath string
	logger      *zap.Logger

	connectedPeers []string
	enodeStr       string
	peerMutex      sync.RWMutex

	lastBlock      *bstream.Block
	lastBlockMutex sync.RWMutex
}

func (s *GethNode) sendGethCommand(cmd string) (string, error) {
	c, err := net.Dial("unix", s.ipcFilePath)
	if err != nil {
		return "", err
	}
	defer c.Close()

	_, err = c.Write([]byte(cmd))
	if err != nil {
		return "", err
	}

	resp, err := readString(c)
	return resp, err
}

func (s *GethNode) setEnodeStr(enodeStr string) error {
	ipAddr := getIPAddress()
	if ipAddr == "" {
		return fmt.Errorf("cannot find local IP address")
	}

	s.peerMutex.Lock()
	defer s.peerMutex.Unlock()
	fixedEnodeStr := enodeRegexp.ReplaceAllString(enodeStr, fmt.Sprintf(`enode://${1}@%s:30303`, ipAddr))
	if fixedEnodeStr != "" && s.enodeStr != fixedEnodeStr {
		s.enodeStr = fixedEnodeStr
	}
	return nil
}

type GethPeersEnforcer struct {
	*shutter.Shutter

	node                 *GethNode
	wantedPeersHostnames []string
	wantedPeers          map[string]*Enode
	logger               *zap.Logger
}

// EnsurePeersByDNS periodically checks IP addresses on the given FQDNs,
// calls /v1/server_id on port 8080 (or other if specified in hostname) and adds them as peers
// wantedPeersHostnames can point to the headless service name in k8s
func (s *GethPeersEnforcer) Run() {
	for {
		select {
		case <-s.Terminating():
			s.logger.Info("geth peers enforced terminated")
			return
		case <-time.After(10 * time.Second):
		}

		if err := s.runOnce(); err != nil {
			s.logger.Warn("enforce peer run failed", zap.Error(err))
		}
	}
}

func (s *GethPeersEnforcer) runOnce() error {
	if len(s.node.enodeStr) < 20 {
		s.logger.Info("wrong enode string will retry", zap.String("enode", s.node.enodeStr))
		return nil
	}

	for _, hostname := range s.wantedPeersHostnames {
		enodes := s.getEnodesFromPeers(hostname)

		enodesString := slices.Map(enodes, func(e *Enode) string { return e.String() })
		s.logger.Debug("got enode", zap.String("hostname", hostname), zap.Strings("enodes", enodesString))

		for _, enode := range enodes {
			s.wantedPeers[enode.ID] = enode
		}
	}

	for _, enode := range s.wantedPeers {
		if err := s.addPeer(enode); err != nil {
			return fmt.Errorf("cannot add peer: %w", err)
		}
	}

	return nil
}

// AddPeer sends a command through IPC socket to connect geth to the given peer
func (s *GethPeersEnforcer) addPeer(peer *Enode) error {
	if s.mustIgnorePeer(peer.Full) {
		return nil
	}

	resp, err := s.node.sendGethCommand(fmt.Sprintf(`{"jsonrpc":"2.0","method":"admin_addPeer","params":["%s"],"id":1}`, peer))
	if err != nil {
		return err
	}

	if !gjson.Get(resp, "result").Bool() {
		return fmt.Errorf("result not true, got '%s'", resp)
	}

	return nil
}

func (s *GethPeersEnforcer) mustIgnorePeer(peer string) bool {
	s.node.peerMutex.RLock()
	defer s.node.peerMutex.RUnlock()

	if strings.Contains(peer, s.node.enodeStr[0:19]) {
		s.logger.Debug("peer is ourself due to same enode id, ignoring", zap.String("peer", peer), zap.String("ourself", s.node.enodeStr))
		return true
	}

	for _, peerPrefix := range s.node.connectedPeers {
		if strings.Contains(peer, peerPrefix) {
			s.logger.Debug("peer already connected", zap.String("peer", peer), zap.String("peerPrefix", peerPrefix))
			return true
		}
	}

	return false
}

type Enode struct {
	Full string
	ID   string
	IP   string
}

func (e *Enode) String() string {
	return e.Full
}

func parseEnode(in string) (*Enode, error) {
	// enode://e4d5433fd9e84930cd38028f2fdb1ca8d55bdb7b6a749da57e8aa7fc5d3c146c44c0d129a14cecc7b0f13bb98700bf392dd4fd7c31bf2fe26038d4ba8f5a8e32@127.0.0.1:30303
	if !strings.HasPrefix(in, "enode://") {
		return nil, fmt.Errorf("invalid enode string, must start with 'enode://': %s", in)
	}

	raw := strings.TrimPrefix(in, "enode://")
	id, ip, ok := strings.Cut(raw, "@")
	if !ok {
		return nil, fmt.Errorf("invalid enode string, must have '@' separator: %s", in)
	}

	return &Enode{
		Full: in,
		ID:   id,
		IP:   ip,
	}, nil
}

func (s *GethPeersEnforcer) getEnodesFromPeers(hostname string) []*Enode {
	port := "8545"
	if splitted := strings.Split(hostname, ":"); len(splitted) == 2 {
		port = splitted[1]
		hostname = splitted[0]
	}

	s.logger.Debug("getting enodes from peers", zap.String("hostname", hostname), zap.String("port", port))

	ips, err := net.LookupIP(hostname)
	if err != nil {
		s.logger.Warn("cannot get IP for hostname", zap.Error(err), zap.String("hostname", hostname))
		return nil
	}

	var enodes []*Enode
	for _, ip := range ips {
		endpoint := fmt.Sprintf("http://%s:%s", ip, port)
		body := `{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}`

		enodeAddr, err := httpPost(endpoint, body)
		if err != nil {
			s.logger.Warn("error getting enode string from RPC call", zap.String("endpoint", endpoint), zap.String("content", body), zap.Error(err))
			continue
		}

		enodeStr := gjson.Get(enodeAddr, "result.enode").String()

		enode, err := parseEnode(enodeStr)
		if err != nil {
			s.logger.Warn("got invalid enode string from IP", zap.String("enode", enodeStr), zap.Error(err))
			continue
		}

		enodes = append(enodes, enode)
	}
	return enodes
}

type GethMonitor struct {
	*shutter.Shutter
	node *GethNode

	logger *zap.Logger
	tracer logging.Tracer
}

// Monitor periodically checks the head block num and block time, as well as the enode string (server ID)
func (s *GethMonitor) Run() {
	var lastLog *time.Time

	for {
		select {
		case <-s.Terminating():
			s.logger.Info("geth monitor terminated")
			return
		case <-time.After(2 * time.Second):
		}

		if err := s.runOnce(); err != nil {
			s.logger.Warn("geth monitor run failed", zap.Error(err))
		}

		if lastLog == nil || time.Since(*lastLog) > time.Minute {
			if s.node.lastBlock == nil {
				s.logger.Info("geth monitor never seen last block yet")
			} else {
				s.logger.Info("geth monitor last block seen", zap.Stringer("block", s.node.lastBlock.AsRef()))
			}

			now := time.Now()
			lastLog = &now
		}
	}
}

func (s *GethMonitor) runOnce() error {
	resp, err := s.node.sendGethCommand(`{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}`)
	if err != nil {
		return fmt.Errorf("cannot get info from IPC socket: %w", err)
	}

	enodeStr := gjson.Get(resp, "result.enode").String()

	fields := []zap.Field{zap.String("enode", enodeStr)}
	if s.tracer.Enabled() {
		fields = append(fields, zap.Reflect("resp", resp))
	}

	s.logger.Debug("got node info", fields...)
	s.node.setEnodeStr(enodeStr)

	resp, err = s.node.sendGethCommand(`{"jsonrpc":"2.0","method":"admin_peers","params":[],"id":1}`)
	if err != nil {
		return fmt.Errorf("cannot get peers from IPC socket: %w", err)
	}

	connectedPeers := []string{}
	for _, peer := range gjson.Get(resp, "result").Array() {
		connectedPeers = append(connectedPeers, peer.Get("enode").String())
	}
	s.node.peerMutex.Lock()
	s.node.connectedPeers = connectedPeers
	s.node.peerMutex.Unlock()

	resp, err = s.node.sendGethCommand(`{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`)
	if err != nil {
		return fmt.Errorf("cannot get blocknumber from IPC socket: %w", err)
	}

	lastBlock := gjson.Get(resp, "result")
	lastBlockNum := hex2uint(lastBlock.String())
	if lastBlockNum == 0 {
		return fmt.Errorf("last block is 0, skipping")
	}

	resp, err = s.node.sendGethCommand(fmt.Sprintf(`{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["%s", true],"id":1}`, lastBlock))
	if err != nil {
		return fmt.Errorf("cannot get block by number: %w", err)
	}

	timestamp := time.Unix(hex2int(gjson.Get(resp, "result.timestamp").String()), 0)
	hash := hex2string(gjson.Get(resp, "result.hash").String())

	s.node.lastBlockMutex.Lock()
	s.node.lastBlock = &bstream.Block{
		Id:        hash,
		Number:    uint64(lastBlockNum),
		Timestamp: timestamp,
	}
	s.node.lastBlockMutex.Unlock()

	return nil
}

// cannot use ReadAll on an IPC socket
func readString(r io.Reader) (string, error) {
	br := bufio.NewReader(r)
	var line string
	for {
		l, err := br.ReadString('\n')
		if len(l) > 0 {
			line += l
		}
		switch err {
		case bufio.ErrBufferFull:
			continue
		case io.EOF, nil:
			return line, nil
		default:
			return "", err
		}
	}
}

func getIPAddress() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip.IsGlobalUnicast() {
				return ip.String()
			}
		}
	}
	return ""
}

func httpPost(addr string, out string) (string, error) {
	resp, err := http.Post(addr, "application/json", strings.NewReader(out))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func hex2int(hexStr string) int64 {
	// remove 0x suffix if found in the input string
	cleaned := strings.Replace(hexStr, "0x", "", -1)

	// base 16 for hexadecimal
	result, _ := strconv.ParseInt(cleaned, 16, 64)
	return result
}

func hex2uint(hexStr string) uint64 {
	// remove 0x suffix if found in the input string
	cleaned := strings.Replace(hexStr, "0x", "", -1)

	// base 16 for hexadecimal
	result, _ := strconv.ParseUint(cleaned, 16, 64)
	return result
}

func hex2string(hexStr string) string {
	// remove 0x suffix if found in the input string
	return strings.Replace(hexStr, "0x", "", -1)
}
