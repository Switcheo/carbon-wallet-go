package carbonwalletgo

import (
	"github.com/cosmos/cosmos-sdk/client"
	"golang.org/x/time/rate"
	"os"
	"path"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	cmcryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	"github.com/cosmos/cosmos-sdk/version"

	"github.com/Switcheo/carbon-wallet-go/api"
	"github.com/Switcheo/carbon-wallet-go/wallet"
)

type WalletConfig struct {
	// Tx timeout height before the transaction is not considered valid by the blockchain
	// set to 0 if you do not want tx to timeout
	TxTimeoutHeight int64
	// Update Block Height throttle duration
	UpdateBlockHeightLimit time.Duration
	// The time to wait between sending out the messages in the async message queue
	// as a single txn.
	MsgFlushInterval time.Duration
	// Message queue buffer length for async messages - if messages have not been flushed
	// (through MsgFlushInterval) before the buffer is full, new msgs will block.
	MsgQueueLength int64
	// Response channel length for sync messages - if channel is not read before the
	// buffer is full, new responses will block.
	ResponseChannelLength int64
	// ConfirmTransaction channel length for sync messages - if channel is not read before the
	// buffer is full, new responses will block.
	ConfirmTransactionChannelLength int64
}

func DefaultWalletConfig() *WalletConfig {
	return &WalletConfig{
		TxTimeoutHeight:                 30, // MainNet ~1min
		UpdateBlockHeightLimit:          5 * time.Second,
		MsgFlushInterval:                100 * time.Millisecond,
		MsgQueueLength:                  1000,
		ResponseChannelLength:           100,
		ConfirmTransactionChannelLength: 100,
	}
}

// ConnectCliWallet connect to a cli wallet
func ConnectCliWallet(targetGRPCAddress string, label string, password string, mainPrefix string, config *WalletConfig, clientCtx client.Context) (wallet wallet.Wallet) {
	for {
		chainID, err := api.GetChainID(targetGRPCAddress, clientCtx)
		if err != nil {
			log.Warnln(label, ": could not get chain id, will try again in a while", err.Error())
			time.Sleep(time.Second * 3) // polling interval
			continue
		}

		privKey, err := getPrivKeyFromCLI(label, password)
		if err != nil {
			log.Warnln(label, ": could not get private key, will try again in a while", err.Error())
			time.Sleep(time.Second * 3) // polling interval
			continue
		}

		wallet, err = ConnectWallet(targetGRPCAddress, privKey, label, chainID, mainPrefix, config, clientCtx)
		if err != nil {
			log.Warnln(label, ": could not connect to wallet, will try again in a while", err.Error())
			time.Sleep(time.Second * 3) // polling interval
			continue
		}

		break
	}

	return wallet
}

// ConnectWallet - inits wallet
func ConnectWallet(targetGRPCAddress string, privKey cmcryptotypes.PrivKey, label string, chainID string, mainPrefix string, config *WalletConfig, clientCtx client.Context) (w wallet.Wallet, err error) {
	pubKey := privKey.PubKey()
	bech32Addr, err := bech32.ConvertAndEncode(mainPrefix, pubKey.Address())
	if err != nil {
		return
	}
	account, err := api.GetAccount(targetGRPCAddress, bech32Addr, clientCtx)
	if err != nil {
		if strings.Contains(err.Error(), "connect: connection refused") {
			log.Info("connection refused, retrying...")
			time.Sleep(100 * time.Millisecond)
			return ConnectWallet(targetGRPCAddress, privKey, label, chainID, mainPrefix, config, clientCtx)
		}
		return
	}

	if account.AccountNumber == 0 {
		log.Info("account not yet setup, will retry in a while...")
		time.Sleep(1000 * time.Millisecond)
		return ConnectWallet(targetGRPCAddress, privKey, label, chainID, mainPrefix, config, clientCtx)
	}

	if config == nil {
		config = DefaultWalletConfig()
	}

	w = wallet.Wallet{
		AccountNumber:             account.AccountNumber,
		AccountSequence:           account.Sequence,
		ChainID:                   chainID,
		PrivKey:                   privKey,
		PubKey:                    pubKey,
		Bech32Addr:                bech32Addr,
		MainPrefix:                mainPrefix,
		DefaultGas:                wallet.DefaultGas,
		TxTimeoutHeight:           config.TxTimeoutHeight,
		UpdateBlockHeightLimiter:  rate.NewLimiter(rate.Every(config.UpdateBlockHeightLimit), 1),
		GRPCURL:                   targetGRPCAddress,
		MsgFlushInterval:          config.MsgFlushInterval,
		MsgQueue:                  make(chan wallet.MsgQueueItem, config.MsgQueueLength),
		ResponseChannel:           make(chan wallet.SubmitMsgResponse, config.ResponseChannelLength),
		StopChannel:               make(chan int, 3),
		ConfirmTransactionChannel: make(chan wallet.TxItems, config.ConfirmTransactionChannelLength),
		ClientCtx:                 clientCtx,
	}

	w.UpdateBlockHeight()

	go w.RunProcessMsgQueue()
	go w.RunConfirmTransactionHash()

	return
}

// getPrivKeyFromCLI -
func getPrivKeyFromCLI(name, passphrase string) (cryptotypes.PrivKey, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	os.Stdin.Close() // force passphrase input from io.Reader
	kb, err := keyring.New(version.Name, "file", path.Join(home, ".carbon"), strings.NewReader(passphrase+"\n"), getCodec())
	if err != nil {
		return nil, err
	}
	armor, err := kb.ExportPrivKeyArmor(name, passphrase)
	if err != nil {
		return nil, err
	}
	privKey, _, err := crypto.UnarmorDecryptPrivKey(armor, passphrase)
	if err != nil {
		return nil, err
	}
	return privKey, nil
}

func getCodec() codec.Codec {
	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	return codec.NewProtoCodec(registry)
}
