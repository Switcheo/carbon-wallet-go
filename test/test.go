package main

import (
	"context"
	"fmt"
	carbonwalletgo "github.com/Switcheo/carbon-wallet-go"
	oracletypes "github.com/Switcheo/carbon/x/oracle/types"

	log "github.com/sirupsen/logrus"

	"github.com/Switcheo/carbon-wallet-go/api"
)

func main() {
	// Test a query to oracles
	grpcAddress := "127.0.0.1:9090"

	grpcConn, _ := api.GetGRPCConnection(grpcAddress)
	defer grpcConn.Close()

	// This creates a gRPC client to query the x/bank service.
	oracleClient := oracletypes.NewQueryClient(grpcConn)
	oracleRes, err := oracleClient.OracleAll(
		context.Background(),
		&oracletypes.QueryAllOracleRequest{
			//Pagination: &sdkquerytypes.PageRequest{}
		},
	)
	if err != nil {
		log.Error(err)
		return
	}

	fmt.Println(oracleRes.GetOracle())

	api.GetChainID(grpcAddress)

	password := "qweqweqwe"
	wallet := carbonwalletgo.ConnectCliWallet(grpcAddress, "oraclewallet", password, "tswth")

	// Test sending a create oracle msg
	msg1 := oracletypes.NewMsgCreateOracle(
		wallet.Bech32Addr,
		"SIDXBTC",
		"The Switcheo TradeHub SIDXBTC Index tracks the Bitcoin spot price every second. The index is a composite built from multiple sources.",
		67,
		100,
		"SecuredByValidators",
		"median",
		1,
		"{\"output_id\":\"btc_usd\",\"templates\":[{\"template_id\":\"response_handler\",\"adapters\":[{\"adapter\":\"unresponsive_handler\",\"config\":{\"strategy\":\"use_last\",\"grace_duration\":\"120\"}},{\"adapter\":\"unchanged_handler\",\"config\":{\"strategy\":\"nullify\",\"threshold_duration\":\"900\"}},{\"adapter\":\"median_filter\",\"config\":{\"strategy\":\"nullify\",\"value_ids\":\"[\\\"bitstamp_btc_usd\\\", \\\"kraken_btc_usd\\\", \\\"coinbase_btc_usd\\\"]\",\"tolerance_percentage\":\"10\"}}]}],\"jobs\":[{\"output_id\":\"bitstamp_btc_usd\",\"adapters\":[{\"adapter\":\"fetcher\",\"config\":{\"url\":\"https://www.bitstamp.net/api/v2/ticker/btcusd\",\"path\":\"last\",\"timeout\":\"10\"}},{\"template\":\"response_handler\"}]},{\"output_id\":\"kraken_btc_usd\",\"adapters\":[{\"adapter\":\"fetcher\",\"config\":{\"url\":\"https://api.kraken.com/0/public/Ticker?pair=XXBTZUSD\",\"path\":\"result.XXBTZUSD.c.0\",\"timeout\":\"10\"}},{\"template\":\"response_handler\"}]},{\"output_id\":\"coinbase_btc_usd\",\"adapters\":[{\"adapter\":\"fetcher\",\"config\":{\"url\":\"https://api.coinbase.com/v2/prices/spot?currency=USD\",\"path\":\"data.amount\",\"timeout\":\"10\"}},{\"template\":\"response_handler\"}]},{\"output_id\":\"btc_usd\",\"adapter\":\"weighted_average\",\"config\":{\"value_ids\":\"[\\\"bitstamp_btc_usd\\\", \\\"kraken_btc_usd\\\", \\\"coinbase_btc_usd\\\"]\",\"weights\":\"[0.25, 0.25, 0.5]\"}},{\"output_id\":\"btc_usd\",\"input_id\":\"btc_usd\",\"adapter\":\"float_handler\",\"config\":{\"operation\":\"round\",\"precision\":\"2\"}}]}",
	)

	resp, err := wallet.SubmitMsg(msg1)
	if err != nil {
		log.Error(err)
		return
	}
	log.Info(resp)
}
