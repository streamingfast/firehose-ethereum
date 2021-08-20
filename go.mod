module github.com/streamingfast/sf-ethereum

go 1.15

require (
	cloud.google.com/go/bigtable v1.2.0
	github.com/ShinyTrinkets/overseer v0.3.0
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869 // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/golang/protobuf v1.5.2
	github.com/google/cel-go v0.4.1
	github.com/google/go-cmp v0.5.6
	github.com/lithammer/dedent v1.1.0
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/lytics/lifecycle v0.0.0-20130117214539-7b4c4028d422 // indirect
	github.com/manifoldco/promptui v0.7.0
	github.com/mitchellh/go-testing-interface v1.14.1
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.7.0
	github.com/streamingfast/blockmeta v0.0.2-0.20210811194956-90dc4202afda
	github.com/streamingfast/bstream v0.0.2-0.20210819232303-b30fbeee04ff
	github.com/streamingfast/dauth v0.0.0-20210811181149-e8fd545948cc
	github.com/streamingfast/dbin v0.0.0-20210809205249-73d5eca35dc5
	github.com/streamingfast/derr v0.0.0-20210811180100-9138d738bcec
	github.com/streamingfast/dgrpc v0.0.0-20210811180351-8646818518b2
	github.com/streamingfast/dhammer v0.0.0-20210811180702-456c4cf0a840
	github.com/streamingfast/dlauncher v0.0.0-20210811194929-f06e488e63da
	github.com/streamingfast/dmetering v0.0.0-20210812002943-aa53fa1ce172
	github.com/streamingfast/dmetrics v0.0.0-20210811180524-8494aeb34447
	github.com/streamingfast/dstore v0.1.1-0.20210811180812-4db13e99cc22
	github.com/streamingfast/eth-go v0.0.0-20210811181433-a73e599b102b
	github.com/streamingfast/firehose v0.1.1-0.20210811195158-d4b116b4b447
	github.com/streamingfast/jsonpb v0.0.0-20210811021341-3670f0aa02d0
	github.com/streamingfast/kvdb v0.0.2-0.20210811194032-09bf862bd2e3
	github.com/streamingfast/logging v0.0.0-20210811175431-f3b44b61606a
	github.com/streamingfast/merger v0.0.3-0.20210811195536-1011c89f0a67
	github.com/streamingfast/node-manager v0.0.2-0.20210820155058-c5162e259ac0
	github.com/streamingfast/pbgo v0.0.6-0.20210812023556-e996f9c4fb86
	github.com/streamingfast/relayer v0.0.2-0.20210811200014-6e0e8bc2814f
	github.com/streamingfast/shutter v1.5.0
	github.com/streamingfast/snapshotter v1.0.3-0.20210811193818-c9e9d4e206e3
	github.com/stretchr/testify v1.7.1-0.20210427113832-6241f9ab9942
	github.com/tidwall/gjson v1.6.1
	go.uber.org/multierr v1.6.0
	go.uber.org/zap v1.17.0
	google.golang.org/grpc v1.39.1
	google.golang.org/protobuf v1.27.1
)

replace github.com/ugorji/go/codec => github.com/ugorji/go v1.1.2

replace github.com/graph-gophers/graphql-go => github.com/streamingfast/graphql-go v0.0.0-20210204202750-0e485a040a3c

replace github.com/ShinyTrinkets/overseer => github.com/dfuse-io/overseer v0.2.1-0.20210326144022-ee491780e3ef

replace github.com/gorilla/rpc => github.com/dfuse-io/rpc v1.2.1-0.20201124195002-f9fc01524e38
