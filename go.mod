module github.com/Switcheo/carbon-wallet-go

go 1.16

require (
	github.com/cosmos/cosmos-sdk v0.42.4
	github.com/google/uuid v1.2.0
	github.com/sirupsen/logrus v1.8.1
	google.golang.org/grpc v1.37.0
)

replace google.golang.org/grpc => google.golang.org/grpc v1.33.2

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1

replace github.com/cosmos/cosmos-sdk => github.com/Switcheo/cosmos-sdk v0.34.4-0.20210614065833-8f123154d4c8
