# Carbon Wallet Go

A lightweight wallet used to bootstrap signing and sending messages via gRPC to the Carbon blockchain.

This wallet manager is used for services that require a only a single signer

The wallet has its own broadcast queue and messages may be enqueued to be sent in sequence if needed.

