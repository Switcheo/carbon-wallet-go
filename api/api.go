package api

import (
	"context"
	subaccounttypes "github.com/Switcheo/carbon/x/subaccount/types"
	"github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	log "github.com/sirupsen/logrus"
)

// GetAccount gets an account from
func GetAccount(targetGRPCAddress string, bech32Address string)  (account *authtypes.BaseAccount, err error)  {
	grpcConn, err := GetGRPCConnection(targetGRPCAddress)
	if err != nil {
		return nil, err
	}
	defer grpcConn.Close()

	log.Info("Getting account: ", bech32Address)

	// This creates a gRPC client to query the x/account service.
	authClient := authtypes.NewQueryClient(grpcConn)
	accountRes, err := authClient.Account(
		context.Background(),
		&authtypes.QueryAccountRequest{
			Address: bech32Address,
		},
	)
	if err != nil {
		log.Error(err)
		return
	}

	ba := authtypes.BaseAccount{}
	err = ba.Unmarshal(accountRes.Account.Value)
	if err != nil {
		log.Error(err)
		return
	}

	log.Infof("Found Account. Address: %s, AccountNumber: %d, Sequence: %d", ba.Address, ba.AccountNumber, ba.Sequence)

	return &ba, nil
}


// GetChainID of the node from tendermint grpc
func GetChainID(targetGRPCAddress string) (chainID string, err error) {
	grpcConn, err := GetGRPCConnection(targetGRPCAddress)
	if err != nil {
		return "", err
	}
	defer grpcConn.Close()
	log.Info("Getting Node Info")

	serviceClient := tmservice.NewServiceClient(grpcConn)
	nodeInfoRes, err := serviceClient.GetNodeInfo(
		context.Background(),
		&tmservice.GetNodeInfoRequest{},
	)
	if err != nil {
		log.Error(err)
		return
	}

	log.Info("node info: ", nodeInfoRes.DefaultNodeInfo)
	return nodeInfoRes.DefaultNodeInfo.Network, nil
}

// GetSubAccountPower get subaccount power
// TODO: NYI, requires power endpoint for grpc query on stargate
func GetSubAccountPower(targetGRPCAddress string, subAddress sdk.AccAddress) (*sdk.Int, error) {
	grpcConn, err := GetGRPCConnection(targetGRPCAddress)
	if err != nil {
		return nil, err
	}
	defer grpcConn.Close()

	log.Info("Getting subaccount: ", subAddress)

	// This creates a gRPC client to query the subaccount
	subaccountClient := subaccounttypes.NewQueryClient(grpcConn)
	subAccountRes, err := subaccountClient.SubAccount(
		context.Background(),
		&subaccounttypes.QueryGetSubAccountRequest{
			SubAccount: subAddress.String(),
		},
	)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	sa := subAccountRes.GetSubAccount()

	log.Info("NYI: ", sa)
	//log.Infof("Found Account. Address: %s, AccountNumber: %d, Sequence: %d", sa., ba.AccountNumber, ba.Sequence)

	return nil, nil
}
