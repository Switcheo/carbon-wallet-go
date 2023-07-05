module github.com/Switcheo/carbon-wallet-go

go 1.16

require (
	github.com/cosmos/cosmos-sdk v0.46.11
	github.com/google/uuid v1.3.0
	github.com/sirupsen/logrus v1.9.0
	golang.org/x/time v0.1.0
	google.golang.org/grpc v1.53.0
)

replace (
	github.com/cosmos/cosmos-sdk => github.com/Switcheo/cosmos-sdk v0.46.13-0.20230417100423-b613c525eea9
	github.com/cosmos/iavl => github.com/Switcheo/iavl v0.19.6-0.20230417100210-36cd2d2d01cf
	github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1
	github.com/tendermint/tendermint => github.com/Switcheo/cometbft v0.34.28-0.20230417131808-019923f54f6b
)
