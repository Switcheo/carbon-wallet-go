package carbonwalletgo

import (
	"os"
	"path"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/version"

	"github.com/Switcheo/carbon-wallet-go/api"
	"github.com/Switcheo/carbon-wallet-go/wallet"
	cmcryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	log "github.com/sirupsen/logrus"

	"github.com/cosmos/cosmos-sdk/crypto"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

type WalletConfig struct {
	// The time to wait between sending out the messages in the async message queue
	// as a single txn.
	MsgFlushInterval time.Duration
	// Message queue buffer length for async messages - if messages have not been flushed
	// (through MsgFlushInterval) before the buffer is full, new msgs will block.
	MsgQueueLength int64
	// Response channel length for sync messages - if channel is not read before the
	// buffer is full, new responses will block.
	ResponseChannelLength int64
}

func NewWalletConfig(msgFlushInterval time.Duration, msgQueueLength int64, responseChannelLength int64) *WalletConfig {
	return &WalletConfig{
		MsgFlushInterval:      msgFlushInterval,
		MsgQueueLength:        msgQueueLength,
		ResponseChannelLength: responseChannelLength,
	}
}

func DefaultWalletConfig() *WalletConfig {
	return &WalletConfig{
		MsgFlushInterval:      100 * time.Millisecond,
		MsgQueueLength:        10,
		ResponseChannelLength: 100,
	}
}

// ConnectCliWallet connect to a cli wallet
func ConnectCliWallet(targetGRPCAddress string, label string, password string, mainPrefix string, config *WalletConfig) (wallet wallet.Wallet) {
	for {
		chainID, err := api.GetChainID(targetGRPCAddress)
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

		wallet, err = ConnectWallet(targetGRPCAddress, privKey, label, chainID, mainPrefix, config)
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
func ConnectWallet(targetGRPCAddress string, privKey cmcryptotypes.PrivKey, label string, chainID string, mainPrefix string, config *WalletConfig) (w wallet.Wallet, err error) {
	pubKey := privKey.PubKey()
	bech32Addr, err := bech32.ConvertAndEncode(mainPrefix, pubKey.Address())
	if err != nil {
		return
	}
	account, err := api.GetAccount(targetGRPCAddress, bech32Addr)
	if err != nil {
		if strings.Contains(err.Error(), "connect: connection refused") {
			log.Info("connection refused, retrying...")
			time.Sleep(100 * time.Millisecond)
			return ConnectWallet(targetGRPCAddress, privKey, label, chainID, mainPrefix, config)
		}
		return
	}

	if account.AccountNumber == 0 {
		log.Info("account not yet setup, will retry in a while...")
		time.Sleep(1000 * time.Millisecond)
		return ConnectWallet(targetGRPCAddress, privKey, label, chainID, mainPrefix, config)
	}

	if config == nil {
		config = DefaultWalletConfig()
	}

	w = wallet.Wallet{
		AccountNumber:    account.AccountNumber,
		ChainID:          chainID,
		PrivKey:          privKey,
		PubKey:           pubKey,
		Bech32Addr:       bech32Addr,
		MainPrefix:       mainPrefix,
		DefaultGas:       wallet.DefaultGas,
		GRPCURL:          targetGRPCAddress,
		MsgFlushInterval: config.MsgFlushInterval,
		MsgQueue:         make(chan wallet.MsgQueueItem, config.MsgQueueLength),
		ResponseChannel:  make(chan wallet.SubmitMsgResponse, config.ResponseChannelLength),
		StopChannel:      make(chan int, 3),
	}

	go w.RunProcessMsgQueue()

	return
}

// getPrivKeyFromCLI -
func getPrivKeyFromCLI(name, passphrase string) (cryptotypes.PrivKey, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	os.Stdin.Close() // force passphrase input from io.Reader
	kb, err := keyring.New(version.Name, "file", path.Join(home, ".carbon"), strings.NewReader(passphrase+"\n"))
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
