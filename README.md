# Carbon Wallet Go

A lightweight wallet used to bootstrap signing and sending messages via gRPC to the Carbon blockchain.

This wallet manager is used for services that require a only a single signer

The wallet has its own broadcast queue and messages may be enqueued to be sent in sequence if needed.

# Development / Building the standalone repo

1) Make sure you have carbon blockchain cloned into

    `~/go/src/github.com/Switcheo/carbon`
   
2) clone this repo into
   
    `~/go/src/github.com/Switcheo/carbon-wallet-go`

Note the relative path and make sure it matches as we are linking to our local private repo of carbon

# Testing

There are no tests at the moment.

Just a couple of random examples you can run while you are running the chain locally in `test/test.go`

Run with:

`go run test/test.go`