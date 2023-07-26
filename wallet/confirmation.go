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

// RetryConfirmTransaction drops retry if txHash was created since timeout,
// otherwise sends txHash to ConfirmTransactionChannel
func (w *Wallet) RetryConfirmTransaction(txHash TxHash) {
	if time.Now().After(txHash.CreatedAt.Add(w.GetConfirmTransactionTimeout())) {
		var response *types.TxResponse
		response.TxHash = txHash.Hash
		w.runCallback(response, txHash.Items, fmt.Errorf("transaction error: transaction timed out"))
		log.Errorf("RetryConfirmTransaction timeout for %+v", txHash.Hash)
		return
	}
	time.Sleep(w.GetConfirmTransactionRetryInterval(txHash))
	txHash.RetryCount++
	w.ConfirmTransactionChannel <- txHash
}

func (w *Wallet) GetConfirmTransactionTimeout() time.Duration {
	timeout := w.ConfirmTransactionTimeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	return timeout
}

// GetConfirmTransactionRetryInterval returns exponential backoff and add ConfirmTransactionMinInterval
func (w *Wallet) GetConfirmTransactionRetryInterval(txHash TxHash) time.Duration {
	multiply := time.Duration(math.Pow(2, float64(txHash.RetryCount)))
	interval := w.ConfirmTransactionMinInterval
	if interval == 0 {
		interval = 5 * time.Second
	}
	return interval + multiply
}

func (w *Wallet) ConfirmTransactionHash(txHash TxHash) {
	grpcConn, err := api.GetGRPCConnection(w.GRPCURL, w.ClientCtx)
	if err != nil {
		go w.RetryConfirmTransaction(txHash)
		log.Error("unable to open grpcConn")
	}
	defer grpcConn.Close()

	txClient := txtypes.NewServiceClient(grpcConn)
	grpcRes, err := txClient.GetTx(context.Background(), &txtypes.GetTxRequest{Hash: txHash.Hash})
	if err != nil {
		go w.RetryConfirmTransaction(txHash)
		if !strings.Contains(err.Error(), "code = NotFound") {
			log.Errorf("ProcessTransactionHash.GetTx failed: %+v\n", err.Error())
		}
		return
	}

	response := grpcRes.TxResponse
	if response.Code == 0 {
		log.Infof("Transaction succeeded: %+v", response.TxHash)
		w.runCallback(response, txHash.Items, nil)
	} else {
		log.Errorf("Transaction failed: txHash: %+v, code: %+v, raw_log: %+v\n", response.TxHash, response.Code, response.RawLog)
		w.runCallback(response, txHash.Items, fmt.Errorf("transaction error: transaction failed"))
	}
}

// RunConfirmTransactionHash confirms the transaction has been completed in the blockchain
func (w *Wallet) RunConfirmTransactionHash() {
	for {
		select {
		case <-w.StopChannel:
			return
		case txHash := <-w.ConfirmTransactionChannel:
			go w.ConfirmTransactionHash(txHash)
		}
	}
}
