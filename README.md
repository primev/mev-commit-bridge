# Bridge to mev-commit chain

This repository houses docker infra, cli and testing tools for operating and interacting with a bridge between L1 ethereum and the mev-commit chain.

## Hyperlane

See the `hyperlane` directory which houses a [hyperlane warp route](https://docs.hyperlane.xyz/docs/protocol/warp-routes) between Sepolia and the mev-commit chain, with two validators attesting to mev-commit chain state, and a message relayer.
