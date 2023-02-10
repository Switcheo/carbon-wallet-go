module github.com/Switcheo/carbon-wallet-go

go 1.16

require (
	github.com/cosmos/cosmos-sdk v0.46.8
	github.com/google/uuid v1.2.0
	github.com/sirupsen/logrus v1.8.1
	golang.org/x/time v0.0.0-20220722155302-e5dcc9cfc0b9
	google.golang.org/grpc v1.42.0
)

replace google.golang.org/grpc => google.golang.org/grpc v1.33.2

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1

replace github.com/cosmos/cosmos-sdk => github.com/Switcheo/cosmos-sdk v0.46.9-0.20230217064727-b167703114d1
