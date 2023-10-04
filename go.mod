module github.com/Switcheo/carbon-wallet-go

go 1.16

require (
	github.com/cometbft/cometbft v0.37.2
	github.com/cosmos/cosmos-sdk v0.47.0-rc2.0.20230220103612-f094a0c33410
	github.com/google/uuid v1.3.0
	github.com/sirupsen/logrus v1.9.0
	golang.org/x/time v0.3.0
	google.golang.org/grpc v1.56.2
)

replace (
	github.com/cometbft/cometbft => github.com/Switcheo/cometbft v0.37.1-0.20230915094332-ebac4866fd2c
	github.com/cosmos/cosmos-sdk => github.com/Switcheo/cosmos-sdk v0.47.5-0.20231002062613-dc05d2771e83
	github.com/cosmos/iavl => github.com/Switcheo/iavl v0.20.1-0.20230907092650-a292b5c6f6ae
	github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1
)
