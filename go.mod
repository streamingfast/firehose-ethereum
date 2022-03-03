module github.com/streamingfast/sf-ethereum

go 1.16

require (
	cloud.google.com/go/bigtable v1.2.0
	cloud.google.com/go/monitoring v1.3.0 // indirect
	cloud.google.com/go/trace v1.1.0 // indirect
	github.com/RoaringBitmap/roaring v0.9.4
	github.com/ShinyTrinkets/overseer v0.3.0
	github.com/golang/protobuf v1.5.2
	github.com/google/cel-go v0.4.1
	github.com/google/go-cmp v0.5.7
	github.com/lithammer/dedent v1.1.0
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/manifoldco/promptui v0.8.0
	github.com/mitchellh/go-testing-interface v1.14.1
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.3
	github.com/spf13/viper v1.8.1
	github.com/streamingfast/blockmeta v0.0.2-0.20220301162623-6ca488c38501
	github.com/streamingfast/bstream v0.0.2-0.20220303152911-1db2bc621583
	github.com/streamingfast/cli v0.0.4-0.20220113202443-f7bcefa38f7e
	github.com/streamingfast/dauth v0.0.0-20210812020920-1c83ba29add1
	github.com/streamingfast/dbin v0.0.0-20210809205249-73d5eca35dc5
	github.com/streamingfast/derr v0.0.0-20220301163149-de09cb18fc70
	github.com/streamingfast/dgrpc v0.0.0-20220301153539-536adf71b594
	github.com/streamingfast/dlauncher v0.0.0-20220302154156-935fc12be373
	github.com/streamingfast/dmetering v0.0.0-20220301165106-a642bb6a21bd
	github.com/streamingfast/dmetrics v0.0.0-20210811180524-8494aeb34447
	github.com/streamingfast/dstore v0.1.1-0.20220203133825-30eb2f9c5cd3
	github.com/streamingfast/eth-go v0.0.0-20210811181433-a73e599b102b
	github.com/streamingfast/firehose v0.1.1-0.20220303153047-161af394ee22
	github.com/streamingfast/jsonpb v0.0.0-20210811021341-3670f0aa02d0
	github.com/streamingfast/kvdb v0.1.1-0.20220228165126-18278ca47f93
	github.com/streamingfast/logging v0.0.0-20220222131651-12c3943aac2e
	github.com/streamingfast/merger v0.0.3-0.20220301162603-c0129b6f1ad4
	github.com/streamingfast/node-manager v0.0.2-0.20220301170656-5dbc7988e730
	github.com/streamingfast/pbgo v0.0.6-0.20220228185940-1bbaafec7d8a
	github.com/streamingfast/relayer v0.0.2-0.20220301162545-2db510359d2a
	github.com/streamingfast/sf-tools v0.0.0-20220301170200-43b1f43dde6f
	github.com/streamingfast/shutter v1.5.0
	github.com/streamingfast/snapshotter v0.0.0-20210906180247-1ec27a37764f
	github.com/stretchr/testify v1.7.1-0.20210427113832-6241f9ab9942
	github.com/tidwall/gjson v1.12.1
	go.uber.org/multierr v1.7.0
	go.uber.org/zap v1.21.0
	google.golang.org/grpc v1.44.0
	google.golang.org/protobuf v1.27.1
)

replace (
	github.com/ShinyTrinkets/overseer => github.com/streamingfast/overseer v0.2.1-0.20210326144022-ee491780e3ef
	github.com/gorilla/rpc => github.com/streamingfast/rpc v1.2.1-0.20201124195002-f9fc01524e38
	github.com/graph-gophers/graphql-go => github.com/streamingfast/graphql-go v0.0.0-20210204202750-0e485a040a3c
)
