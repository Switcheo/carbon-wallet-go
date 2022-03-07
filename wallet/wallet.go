package wallet

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Switcheo/carbon-wallet-go/constants"
	"github.com/Switcheo/carbon-wallet-go/utils"
	"github.com/cosmos/cosmos-sdk/client"
	clienttx "github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cmcryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtxtypes "github.com/cosmos/cosmos-sdk/x/auth/tx"

	"github.com/Switcheo/carbon-wallet-go/api"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// Config
const (
	DefaultGas         = 1000000000000
	BroadcastModeAsync = BroadcastMode("async")
	BroadcastModeSync  = BroadcastMode("sync")
	BroadcastModeBlock = BroadcastMode("block")
)

// BroadcastMode - async, sync and block are only supported
type BroadcastMode string

// SubmitMsgResponse submit message response
type SubmitMsgResponse struct {
	ID       string
	Response *sdktypes.TxResponse
	Error    error
}

// MsgQueueItem message queue item
type MsgQueueItem struct {
	ID    string
	Msg   sdktypes.Msg
	Async bool
}

// Wallet - used to submit tx
type Wallet struct {
	AccountNumber    uint64
	ChainID          string
	PrivKey          cmcryptotypes.PrivKey
	PubKey           cmcryptotypes.PubKey
	Bech32Addr       string
	MainPrefix       string
	DefaultGas       uint64
	GRPCURL          string
	MsgFlushInterval time.Duration
	MsgQueue         chan MsgQueueItem
	ResponseChannel  chan SubmitMsgResponse
	StopChannel      chan int
}

// AccAddress -
func (w *Wallet) AccAddress() sdktypes.AccAddress {
	return sdktypes.AccAddress(w.PubKey.Address())
}

// CreateAndSignTx - creates a tx and signs it to be broadcasted
// Everytime this is called, it will also automatically do a grpc call to get the latest acc sequence for signature
func (w *Wallet) CreateAndSignTx(msgs []sdktypes.Msg) (tx authsigning.Tx, err error) {
	txConfig := GetTxConfig()
	txBuilder := txConfig.NewTxBuilder()

	// Get acc to get latest sequence and account number
	acc, err := api.GetAccount(w.GRPCURL, w.Bech32Addr)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	// Set messages
	err = txBuilder.SetMsgs(msgs...)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	// Set other tx details
	var feeCoins types.Coins = make([]types.Coin, 1)
	feeAmount := utils.MustDecShiftInt(types.NewDec(int64(len(txBuilder.GetTx().GetMsgs()))), 8)
	feeCoins[0] = types.Coin{
		Denom:  constants.MainDenom,
		Amount: feeAmount,
	}
	txBuilder.SetFeeAmount(feeCoins)
	txBuilder.SetGasLimit(feeAmount.Uint64())

	// Adapted from: https://docs.cosmos.network/master/run-node/txs.html#broadcasting-a-transaction-3

	// First round: we gather all the signer infos. We use the "set empty
	// signature" hack to do that.
	sigV2 := signingtypes.SignatureV2{
		PubKey: w.PrivKey.PubKey(),
		Data: &signingtypes.SingleSignatureData{
			SignMode:  txConfig.SignModeHandler().DefaultMode(),
			Signature: nil,
		},
		Sequence: acc.GetSequence(),
	}

	err = txBuilder.SetSignatures(sigV2)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	// Second round: all signer infos are set, so each signer can sign.
	signerData := authsigning.SignerData{
		ChainID:       w.ChainID,
		AccountNumber: acc.GetAccountNumber(),
		Sequence:      acc.GetSequence(),
	}
	sigV2, err = clienttx.SignWithPrivKey(
		txConfig.SignModeHandler().DefaultMode(), signerData,
		txBuilder, w.PrivKey, txConfig, acc.GetSequence())
	if err != nil {
		return nil, err
	}

	err = txBuilder.SetSignatures(sigV2)
	if err != nil {
		return nil, err
	}

	return txBuilder.GetTx(), nil
}

// BroadcastTx - broadcasts a tx via grpc
func (w *Wallet) BroadcastTx(tx authsigning.Tx, mode BroadcastMode) (txResp *sdktypes.TxResponse, err error) {
	switch mode {
	case BroadcastModeAsync:
	case BroadcastModeSync:
	case BroadcastModeBlock:
	default:
		err = fmt.Errorf("invalid broadcast mode: %s", mode)
		return
	}

	txConfig := GetTxConfig()
	txBytes, err := txConfig.TxEncoder()(tx)
	if err != nil {
		return nil, err
	}

	// Broadcast the tx via gRPC. We create a new client for the Protobuf Tx
	// service.
	grpcConn, err := api.GetGRPCConnection(w.GRPCURL)
	if err != nil {
		return nil, err
	}
	defer grpcConn.Close()

	txClient := txtypes.NewServiceClient(grpcConn)

	log.Info("Broadcasting Txn with messages: ", tx.GetMsgs())

	// We then call the BroadcastTx method on this client.
	grpcRes, err := txClient.BroadcastTx(
		context.Background(),
		&txtypes.BroadcastTxRequest{
			Mode:    txtypes.BroadcastMode_BROADCAST_MODE_SYNC,
			TxBytes: txBytes, // Proto-binary of the signed transaction, see previous step.
		},
	)
	if err != nil {
		log.Info(err)
		return nil, err
	}

	return grpcRes.TxResponse, nil
}

// SubmitMsg - submits a sdk.Msg to for broadcasting
func (w *Wallet) SubmitMsg(msg sdktypes.Msg) (*sdktypes.TxResponse, error) {
	id := uuid.New().String()
	item := MsgQueueItem{
		ID:    id,
		Msg:   msg,
		Async: false,
	}
	w.MsgQueue <- item
	for {
		msgResponse := <-w.ResponseChannel
		if strings.EqualFold(msgResponse.ID, id) {
			return msgResponse.Response, msgResponse.Error
		}
	}
}

// SubmitMsgAsync non-blocking submit
func (w *Wallet) SubmitMsgAsync(msg sdktypes.Msg) {
	id := uuid.New().String()
	item := MsgQueueItem{
		ID:    id,
		Msg:   msg,
		Async: true,
	}
	w.MsgQueue <- item
}

// ProcessMsgQueue process the msg queue
func (w *Wallet) ProcessMsgQueue() {
	items := []MsgQueueItem{}
	msgs := []sdktypes.Msg{}

	for {
		select {
		case item := <-w.MsgQueue:
			items = append(items, item)
			msgs = append(msgs, item.Msg)
			continue
		default:
		}
		break
	}

	if len(items) == 0 {
		return
	}

	tx, err := w.CreateAndSignTx(msgs)
	if err != nil {
		for _, item := range items {
			w.EnqueueMsgResponse(item, &sdktypes.TxResponse{}, err)
		}
		return
	}

	response, err := w.BroadcastTx(tx, "block")
	if err != nil || response.Code != 0 {
		errFull := fmt.Errorf("submit msg failed: %d, %v", response.Code, response.RawLog)
		if err != nil {
			errFull = fmt.Errorf("%v, %v", errFull, err.Error())
		}
		for _, item := range items {
			w.EnqueueMsgResponse(item, response, errFull)
		}
		return
	}

	for _, item := range items {
		w.EnqueueMsgResponse(item, response, nil)
	}
}

// EnqueueMsgResponse enqueue msg response
func (w *Wallet) EnqueueMsgResponse(item MsgQueueItem, response *sdktypes.TxResponse, err error) {
	if item.Async {
		return
	}

	msgResponse := SubmitMsgResponse{
		ID:       item.ID,
		Response: response,
		Error:    err,
	}
	w.ResponseChannel <- msgResponse
}

// RunProcessMsgQueue process msg queue in intervals
func (w *Wallet) RunProcessMsgQueue() {
	for {
		select {
		case <-w.StopChannel:
			return
		default:
			interval := w.MsgFlushInterval
			if interval == 0 {
				interval = 100 * time.Millisecond
			}
			time.Sleep(interval)
			w.ProcessMsgQueue()
		}
	}
}

// Disconnect disconnect wallet
func (w *Wallet) Disconnect() {
	w.StopChannel <- 1
}

func GetTxConfig() client.TxConfig {
	// Choose codec: Amino or Protobuf. Here, we use Protobuf
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	protoCodec := codec.NewProtoCodec(interfaceRegistry)
	return authtxtypes.NewTxConfig(protoCodec, []signingtypes.SignMode{signingtypes.SignMode_SIGN_MODE_DIRECT})
}
