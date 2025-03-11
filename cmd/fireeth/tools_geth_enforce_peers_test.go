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
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_parseEnode(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    *Enode
		wantErr require.ErrorAssertionFunc
	}{
		{
			"default",
			"enode://a025a6c00a9b90866183d9cc1c10e5dc805de98efdefd2f9245e4322efddcdbd9f27cf0ab0d10a23a1127629a568eab3f234b7ab5f44dfd84a654ab2ddd74505@34.41.215.240:30305",
			&Enode{
				Original: "enode://a025a6c00a9b90866183d9cc1c10e5dc805de98efdefd2f9245e4322efddcdbd9f27cf0ab0d10a23a1127629a568eab3f234b7ab5f44dfd84a654ab2ddd74505@34.41.215.240:30305",
				ID:       "a025a6c00a9b90866183d9cc1c10e5dc805de98efdefd2f9245e4322efddcdbd9f27cf0ab0d10a23a1127629a568eab3f234b7ab5f44dfd84a654ab2ddd74505",
				IP:       "34.41.215.240",
				Port:     30305,
			},
			require.NoError,
		},
		{
			"no port",
			"enode://a025a6c00a9b90866183d9cc1c10e5dc805de98efdefd2f9245e4322efddcdbd9f27cf0ab0d10a23a1127629a568eab3f234b7ab5f44dfd84a654ab2ddd74505@34.41.215.240",
			&Enode{
				Original: "enode://a025a6c00a9b90866183d9cc1c10e5dc805de98efdefd2f9245e4322efddcdbd9f27cf0ab0d10a23a1127629a568eab3f234b7ab5f44dfd84a654ab2ddd74505@34.41.215.240",
				ID:       "a025a6c00a9b90866183d9cc1c10e5dc805de98efdefd2f9245e4322efddcdbd9f27cf0ab0d10a23a1127629a568eab3f234b7ab5f44dfd84a654ab2ddd74505",
				IP:       "34.41.215.240",
				Port:     30303,
			},
			require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseEnode(tt.in)
			tt.wantErr(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestEnode_String(t *testing.T) {
	defaultEnode := "enode://a025a6c00a9b90866183d9cc1c10e5dc805de98efdefd2f9245e4322efddcdbd9f27cf0ab0d10a23a1127629a568eab3f234b7ab5f44dfd84a654ab2ddd74505@34.41.215.240:30303"

	tests := []struct {
		name string
		in   *Enode
		want string
	}{
		{
			"default",
			enode(t, defaultEnode),
			defaultEnode,
		},

		{
			"id modified",
			enodeModified(t, defaultEnode, func(enode *Enode) {
				enode.ID = "modified-id"
			}),
			"enode://modified-id@34.41.215.240:30303",
		},

		{
			"ip modified",
			enodeModified(t, defaultEnode, func(enode *Enode) {
				enode.IP = "10.0.0.1"
			}),
			"enode://a025a6c00a9b90866183d9cc1c10e5dc805de98efdefd2f9245e4322efddcdbd9f27cf0ab0d10a23a1127629a568eab3f234b7ab5f44dfd84a654ab2ddd74505@10.0.0.1:30303",
		},

		{
			"port modified",
			enodeModified(t, defaultEnode, func(enode *Enode) {
				enode.Port = 40404
			}),
			"enode://a025a6c00a9b90866183d9cc1c10e5dc805de98efdefd2f9245e4322efddcdbd9f27cf0ab0d10a23a1127629a568eab3f234b7ab5f44dfd84a654ab2ddd74505@34.41.215.240:40404",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.String(); got != tt.want {
				t.Errorf("Enode.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func enode(t *testing.T, in string) *Enode {
	t.Helper()
	enode, err := parseEnode(in)
	require.NoError(t, err)
	return enode
}

func enodeModified(t *testing.T, in string, f func(*Enode)) *Enode {
	t.Helper()
	enode := enode(t, in)
	f(enode)
	return enode
}
