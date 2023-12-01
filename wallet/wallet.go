package wallet

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/time/rate"

	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/x/tx/signing"
	"github.com/Switcheo/carbon-wallet-go/constants"
	"github.com/Switcheo/carbon-wallet-go/utils"
	"github.com/cosmos/cosmos-sdk/client"
	clienttx "github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cmcryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtxtypes "github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/cosmos/gogoproto/proto"

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
	ID       string
	Msg      sdktypes.Msg
	Async    bool
	Callback func(*sdktypes.TxResponse, sdktypes.Msg, error)
}

type TxItems struct {
	Hash       string
	Items      []MsgQueueItem
	CreatedAt  time.Time
	RetryCount uint
}

// Wallet - used to submit tx
type Wallet struct {
	AccountNumber                 uint64
	AccountSequence               uint64
	ChainID                       string
	PrivKey                       cmcryptotypes.PrivKey
	PubKey                        cmcryptotypes.PubKey
	Bech32Addr                    string
	MainPrefix                    string
	DefaultGas                    uint64
	TxTimeoutHeight               int64
	CurrentBlockHeight            int64
	UpdateBlockHeightLimiter      *rate.Limiter
	GRPCURL                       string
	MsgFlushInterval              time.Duration
	MsgQueue                      chan MsgQueueItem
	ResponseChannel               chan SubmitMsgResponse
	StopChannel                   chan int
	ConfirmTransactionChannel     chan TxItems
	ConfirmTransactionMinInterval time.Duration
	ConfirmTransactionTimeout     time.Duration
	ClientCtx                     client.Context
}

// AccAddress -
func (w *Wallet) AccAddress() sdktypes.AccAddress {
	return sdktypes.AccAddress(w.PubKey.Address())
}

func (w *Wallet) IncrementAccountSequence() {
	w.AccountSequence++
}

// CreateAndSignTx - creates a tx and signs it to be broadcasted
// Everytime this is called, the nonce will be incremented.
// If there is a nonce error after broadcast, it'll refetch the nonce.
func (w *Wallet) CreateAndSignTx(msgs []sdktypes.Msg) (tx authsigning.Tx, err error) {
	txConfig := GetTxConfig()
	txBuilder := txConfig.NewTxBuilder()

	accountSequence := w.AccountSequence
	w.IncrementAccountSequence()

	// Set messages
	err = txBuilder.SetMsgs(msgs...)
	if err != nil {
		log.Error("setmsg err", err)
		return nil, err
	}

	// Set other tx details
	var feeCoins types.Coins = make([]types.Coin, 1)
	feeAmount := utils.MustDecShiftInt(sdkmath.LegacyNewDec(int64(len(txBuilder.GetTx().GetMsgs()))), 8)
	feeCoins[0] = types.Coin{
		Denom:  constants.MainDenom,
		Amount: feeAmount,
	}
	txBuilder.SetFeeAmount(feeCoins)
	txBuilder.SetGasLimit(feeAmount.Uint64())

	if w.TxTimeoutHeight != 0 {
		timeoutHeight := w.GetCurrentBlockHeight() + w.TxTimeoutHeight
		txBuilder.SetTimeoutHeight(uint64(timeoutHeight))
	}

	// Adapted from: https://docs.cosmos.network/master/run-node/txs.html#broadcasting-a-transaction-3

	// First round: we gather all the signer infos. We use the "set empty
	// signature" hack to do that.
	sigV2 := signingtypes.SignatureV2{
		PubKey: w.PrivKey.PubKey(),
		Data: &signingtypes.SingleSignatureData{
			SignMode:  signingtypes.SignMode_SIGN_MODE_DIRECT,
			Signature: nil,
		},
		Sequence: accountSequence,
	}

	err = txBuilder.SetSignatures(sigV2)
	if err != nil {
		log.Error("setsig err", err)
		return nil, err
	}

	// Second round: all signer infos are set, so each signer can sign.
	signerData := authsigning.SignerData{
		ChainID:       w.ChainID,
		AccountNumber: w.AccountNumber,
		Sequence:      accountSequence,
		PubKey:        w.PrivKey.PubKey(),
	}
	sigV2, err = clienttx.SignWithPrivKey(
		context.Background(),
		signingtypes.SignMode_SIGN_MODE_DIRECT, signerData,
		txBuilder, w.PrivKey, txConfig, accountSequence)
	if err != nil {
		log.Error("sign err", err)
		return nil, err
	}

	err = txBuilder.SetSignatures(sigV2)
	if err != nil {
		log.Error("setsig err", err)
		return nil, err
	}

	return txBuilder.GetTx(), nil
}

// UpdateBlockHeight updates the block height using rate limiter to update CurrentBlockHeight.
// CurrentBlockHeight is used to calculate tx.TimeoutHeight
func (w *Wallet) UpdateBlockHeight() {
	if !w.UpdateBlockHeightLimiter.Allow() {
		return
	}

	blockHeight, err := api.GetLatestBlockHeight(w.GRPCURL, w.ClientCtx)
	if err != nil {
		panic("unable to get latest block height of chain")
	}
	w.CurrentBlockHeight = blockHeight
}

// GetCurrentBlockHeight calls UpdateBlockHeight before returning CurrentBlockHeight
func (w *Wallet) GetCurrentBlockHeight() int64 {
	w.UpdateBlockHeight()
	return w.CurrentBlockHeight
}

// BroadcastTx - broadcasts a tx via grpc
func (w *Wallet) BroadcastTx(tx authsigning.Tx, mode BroadcastMode, items []MsgQueueItem) (txResp *sdktypes.TxResponse, err error) {
	switch mode {
	case BroadcastModeAsync:
	case BroadcastModeSync:
	default:
		err = fmt.Errorf("invalid broadcast mode: %s", mode)
		return
	}

	txConfig := GetTxConfig()
	txBytes, err := txConfig.TxEncoder()(tx)
	if err != nil {
		log.Error("encoding err", err)
		return nil, err
	}

	// Broadcast the tx via gRPC. We create a new client for the Protobuf Tx
	// service.
	grpcConn, err := api.GetGRPCConnection(w.GRPCURL, w.ClientCtx)
	if err != nil {
		log.Error("grpc error", err)
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
		log.Error(err)
		return nil, err
	}

	if grpcRes.TxResponse.Code != 0 {
		err = fmt.Errorf("Broadcast failed with code: %+v, raw_log: %+v\n", grpcRes.TxResponse.Code, grpcRes.TxResponse.RawLog)
		log.Error(err)

		// handle account nonce mismatch error
		if grpcRes.TxResponse.Code == 32 { // 32 is nonce error
			acc, err := api.GetAccount(w.GRPCURL, w.Bech32Addr, w.ClientCtx)
			if err != nil {
				err = fmt.Errorf("Unable to refetch account sequence: %+v\n", err)
				log.Error(err)
				return grpcRes.TxResponse, err
			}
			w.AccountSequence = acc.Sequence
		}
		return grpcRes.TxResponse, err
	}

	txHash := grpcRes.TxResponse.TxHash
	log.Info("Broadcasted tx hash: ", txHash)
	w.ConfirmTransactionChannel <- TxItems{Hash: grpcRes.TxResponse.TxHash, CreatedAt: time.Now(), RetryCount: 0, Items: items}

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
func (w *Wallet) SubmitMsgAsync(msg sdktypes.Msg, callback func(*sdktypes.TxResponse, sdktypes.Msg, error)) {
	id := uuid.New().String()
	item := MsgQueueItem{
		ID:       id,
		Msg:      msg,
		Async:    true,
		Callback: callback,
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
		log.Error("create ang sign tx err", err)
		for _, item := range items {
			w.EnqueueMsgResponse(item, &sdktypes.TxResponse{}, err)
		}
		return
	}

	var responseErr error
	response, err := w.BroadcastTx(tx, BroadcastModeSync, items)
	if err != nil {
		responseErr = err
	}
	if response != nil && response.Code != 0 {
		responseErr = fmt.Errorf("submit msg failed: err: %v, code: %d, raw_log: %v\n", err, response.Code, response.RawLog)
	}

	for _, item := range items {
		w.EnqueueMsgResponse(item, response, responseErr)
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
	interfaceRegistry, _ := codectypes.NewInterfaceRegistryWithOptions(
		// todo: should panic here?
		codectypes.InterfaceRegistryOptions{
			ProtoFiles: proto.HybridResolver,
			SigningOptions: signing.Options{
				AddressCodec: address.Bech32Codec{
					Bech32Prefix: "tswth",
				},
				ValidatorAddressCodec: address.Bech32Codec{
					Bech32Prefix: "tswthvaloper",
				},
			},
		},
	)
	protoCodec := codec.NewProtoCodec(interfaceRegistry)
	return authtxtypes.NewTxConfig(protoCodec, []signingtypes.SignMode{signingtypes.SignMode_SIGN_MODE_DIRECT})
}
