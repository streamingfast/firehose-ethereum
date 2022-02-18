module github.com/streamingfast/sf-ethereum

go 1.15

require (
	cloud.google.com/go/bigtable v1.2.0
	github.com/RoaringBitmap/roaring v0.9.4 // indirect
	github.com/ShinyTrinkets/overseer v0.3.0
	github.com/coreos/etcd v3.3.13+incompatible // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dfuse-io/dbin v0.0.0-20200406215642-ec7f22e794eb // indirect
	github.com/dfuse-io/dgrpc v0.0.0-20210810041652-d033fee35ae0 // indirect
	github.com/dfuse-io/dmetrics v0.0.0-20200406214800-499fc7b320ab // indirect
	github.com/dfuse-io/dstore v0.1.0 // indirect
	github.com/dfuse-io/jsonpb v0.0.0-20200406211248-c5cf83f0e0c0 // indirect
	github.com/dfuse-io/pbgo v0.0.6-0.20210108215028-712d6889e94a // indirect
	github.com/golang/protobuf v1.5.2
	github.com/google/cel-go v0.4.1
	github.com/google/go-cmp v0.5.6
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/lithammer/dedent v1.1.0
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/manifoldco/promptui v0.8.0
	github.com/mitchellh/go-testing-interface v1.14.1
	github.com/pkg/errors v0.9.1
	github.com/rs/xid v1.2.1 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/spf13/viper v1.8.1
	github.com/streamingfast/blockmeta v0.0.2-0.20220120224650-4174f3af6303
	github.com/streamingfast/bstream v0.0.2-0.20220218160526-8411195fe367
	github.com/streamingfast/cli v0.0.3-0.20210811201236-5c00ec55462d
	github.com/streamingfast/dauth v0.0.0-20210812020920-1c83ba29add1
	github.com/streamingfast/dbin v0.0.0-20210809205249-73d5eca35dc5
	github.com/streamingfast/derr v0.0.0-20210811180100-9138d738bcec
	github.com/streamingfast/dgrpc v0.0.0-20211210152421-f8cec68e0383
	github.com/streamingfast/dlauncher v0.0.0-20211210162313-cf4aa5fc4878
	github.com/streamingfast/dmetering v0.0.0-20210812002943-aa53fa1ce172
	github.com/streamingfast/dmetrics v0.0.0-20210811180524-8494aeb34447
	github.com/streamingfast/dstore v0.1.1-0.20211028233549-6fa17808533b
	github.com/streamingfast/eth-go v0.0.0-20210811181433-a73e599b102b
	github.com/streamingfast/firehose v0.1.1-0.20220211203454-a8faca3705ca
	github.com/streamingfast/jsonpb v0.0.0-20210811021341-3670f0aa02d0
	github.com/streamingfast/kvdb v0.0.2-0.20210811194032-09bf862bd2e3
	github.com/streamingfast/logging v0.0.0-20220214194203-5c70c134fea8
	github.com/streamingfast/merger v0.0.3-0.20220201165645-625f805412f0
	github.com/streamingfast/node-manager v0.0.2-0.20211207181611-383f06886a4c
	github.com/streamingfast/pbgo v0.0.6-0.20220104194237-6534a2f6320b
	github.com/streamingfast/relayer v0.0.2-0.20220120224524-84b9578c9323
	github.com/streamingfast/sf-tools v0.0.0-20220201214246-766d25d2d43b
	github.com/streamingfast/shutter v1.5.0
	github.com/streamingfast/snapshotter v0.0.0-20210906180247-1ec27a37764f
	github.com/stretchr/testify v1.7.1-0.20210427113832-6241f9ab9942
	github.com/tidwall/gjson v1.12.1
	go.uber.org/multierr v1.6.0
	go.uber.org/zap v1.19.1
	google.golang.org/grpc v1.39.1
	google.golang.org/protobuf v1.27.1
)

replace (
	github.com/ShinyTrinkets/overseer => github.com/dfuse-io/overseer v0.2.1-0.20210326144022-ee491780e3ef
	github.com/gorilla/rpc => github.com/streamingfast/rpc v1.2.1-0.20201124195002-f9fc01524e38
	github.com/graph-gophers/graphql-go => github.com/streamingfast/graphql-go v0.0.0-20210204202750-0e485a040a3c
	github.com/ugorji/go/codec => github.com/ugorji/go v1.1.2
)
