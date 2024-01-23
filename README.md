# Bridge to mev-commit chain

This repository houses docker infra, cli and testing tools for operating and interacting with a bridge between L1 ethereum and the mev-commit chain.

## Hyperlane warp route

The [hyperlane](./hyperlane) directory houses a [hyperlane warp route](https://docs.hyperlane.xyz/docs/protocol/warp-routes) between L1 ethereum and the mev-commit chain, with two validators attesting to mev-commit chain state, and a message relayer.

## Standard bridge

The [standard](./standard) directory houses a simple bridging protocol implementation between L1 ethereum and the mev-commit chain. Simplicity and readability are most important here, thus both the relayer and validator role will be assumed by a single node type built in golang.
