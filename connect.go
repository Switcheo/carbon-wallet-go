package carbonwalletgo

import (
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/version"
	"os"
	"path"
	"strings"
	"time"

	"github.com/Switcheo/carbon-wallet-go/api"
	"github.com/Switcheo/carbon-wallet-go/wallet"
	cmcryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	log "github.com/sirupsen/logrus"

	"github.com/cosmos/cosmos-sdk/crypto"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

// ConnectCliWallet connect to a cli wallet
func ConnectCliWallet(targetGRPCAddress string, label string, password string, mainPrefix string) (wallet wallet.Wallet) {
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

		wallet, err = ConnectWallet(targetGRPCAddress, privKey, label, chainID, mainPrefix)
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
func ConnectWallet(targetGRPCAddress string, privKey cmcryptotypes.PrivKey, label string, chainID string, mainPrefix string) (w wallet.Wallet, err error) {
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
			return ConnectWallet(targetGRPCAddress, privKey, label, chainID, mainPrefix)
		}
		return
	}

	if account.AccountNumber == 0 {
		log.Info("account not yet setup, will retry in a while...")
		time.Sleep(1000 * time.Millisecond)
		return ConnectWallet(targetGRPCAddress, privKey, label, chainID, mainPrefix)
	}

	w = wallet.Wallet{
		AccountNumber:   account.AccountNumber,
		ChainID:         chainID,
		PrivKey:         privKey,
		PubKey:          pubKey,
		Bech32Addr:      bech32Addr,
		MainPrefix:      mainPrefix,
		DefaultGas:      wallet.DefaultGas,
		MsgQueue:        make(chan wallet.MsgQueueItem, 10),
		ResponseChannel: make(chan wallet.SubmitMsgResponse, 100),
		StopChannel:     make(chan int, 10),
		GRPCURL:         targetGRPCAddress,
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
