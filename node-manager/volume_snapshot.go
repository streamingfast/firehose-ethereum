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

package nodemanager

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/streamingfast/snapshotter"
	"go.uber.org/zap"
)

type GKEPVCSnapshotter struct {
	tag       string
	project   string
	namespace string
	pod       string
	prefix    string
}

var gkeExampleConfigString = "type=gke-pvc-snapshot tag=v1 namespace=default project=mygcpproject prefix=datadir"

func gkeCheckMissing(conf map[string]string, param string) error {
	if conf[param] == "" {
		return fmt.Errorf("backup module gke-pvc-snapshot missing value for %s. Example: %s", param, gkeExampleConfigString)
	}
	return nil
}

func NewGKEPVCSnapshotter(conf map[string]string) (*GKEPVCSnapshotter, error) {
	for _, label := range []string{"tag", "project", "namespace", "prefix"} {
		if err := gkeCheckMissing(conf, label); err != nil {
			return nil, err
		}
	}
	return &GKEPVCSnapshotter{
		tag:       conf["tag"],
		project:   conf["project"],
		namespace: conf["namespace"],
		pod:       os.Getenv("HOSTNAME"),
		prefix:    conf["prefix"],
	}, nil
}

func (s *GKEPVCSnapshotter) RequiresStop() bool {
	return true
}

func (s *GKEPVCSnapshotter) Backup(lastSeenBlockNum uint32) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	snapshotName := snapshotter.GenerateName(s.namespace, s.tag, lastSeenBlockNum)
	return snapshotName, snapshotter.TakeSnapshot(ctx, snapshotName, s.project, s.namespace, s.pod, s.prefix)

}

func (s *Superviser) TakeVolumeSnapshot(volumeSnapshotTag, project, namespace, pod, prefix string, lastSeenBlockNum uint64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	snapshotName := snapshotter.GenerateName(namespace, volumeSnapshotTag, uint32(lastSeenBlockNum))
	s.Logger.Info("starting snapshot", zap.String("name", snapshotName))
	return snapshotter.TakeSnapshot(ctx, snapshotName, project, namespace, pod, prefix)
}
