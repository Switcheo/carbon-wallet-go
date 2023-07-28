package wallet

import (
	"context"
	"fmt"
	"github.com/Switcheo/carbon-wallet-go/api"
	"github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	log "github.com/sirupsen/logrus"
	"math"
	"strings"
	"time"
)

func (w *Wallet) runCallback(response *types.TxResponse, items []MsgQueueItem, err error) {
	for _, item := range items {
		if item.Callback != nil {
			item.Callback(response, item.Msg, err)
		}
	}
}

// RetryConfirmTransaction drops retry if txItems was created since timeout,
// otherwise sends txItems to ConfirmTransactionChannel
func (w *Wallet) RetryConfirmTransaction(txItems TxItems) {
	if time.Now().After(txItems.CreatedAt.Add(w.GetConfirmTransactionTimeout())) {
		var response *types.TxResponse
		response.TxHash = txItems.Hash
		w.runCallback(response, txItems.Items, fmt.Errorf("transaction error: transaction timed out"))
		log.Errorf("RetryConfirmTransaction timeout for %+v", txItems.Hash)
		return
	}
	time.Sleep(w.GetConfirmTransactionRetryInterval(txItems))
	txItems.RetryCount++
	w.ConfirmTransactionChannel <- txItems
}

func (w *Wallet) GetConfirmTransactionTimeout() time.Duration {
	timeout := w.ConfirmTransactionTimeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	return timeout
}

// GetConfirmTransactionRetryInterval returns exponential backoff and add ConfirmTransactionMinInterval
func (w *Wallet) GetConfirmTransactionRetryInterval(txItems TxItems) time.Duration {
	multiply := time.Duration(math.Pow(2, float64(txItems.RetryCount)))
	interval := w.ConfirmTransactionMinInterval
	if interval == 0 {
		interval = 5 * time.Second
	}
	return interval + multiply
}

func (w *Wallet) ConfirmTransactionHash(txItems TxItems) {
	grpcConn, err := api.GetGRPCConnection(w.GRPCURL, w.ClientCtx)
	if err != nil {
		go w.RetryConfirmTransaction(txItems)
		log.Error("unable to open grpcConn")
	}
	defer grpcConn.Close()

	txClient := txtypes.NewServiceClient(grpcConn)
	grpcRes, err := txClient.GetTx(context.Background(), &txtypes.GetTxRequest{Hash: txItems.Hash})
	if err != nil {
		go w.RetryConfirmTransaction(txItems)
		if !strings.Contains(err.Error(), "code = NotFound") {
			log.Errorf("ProcessTransactionHash.GetTx failed: %+v\n", err.Error())
		}
		return
	}

	response := grpcRes.TxResponse
	if response.Code == 0 {
		log.Infof("Transaction succeeded: %+v", response.TxHash)
		w.runCallback(response, txItems.Items, nil)
	} else {
		log.Errorf("Transaction failed: txHash: %+v, code: %+v, raw_log: %+v\n", response.TxHash, response.Code, response.RawLog)
		w.runCallback(response, txItems.Items, fmt.Errorf("transaction error: transaction failed"))
	}
}

// RunConfirmTransactionHash confirms the transaction has been completed in the blockchain
func (w *Wallet) RunConfirmTransactionHash() {
	for {
		select {
		case <-w.StopChannel:
			return
		case txItems := <-w.ConfirmTransactionChannel:
			go w.ConfirmTransactionHash(txItems)
		}
	}
}
