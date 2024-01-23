# Standard bridge

This document outlines multiple iteration plans for a simple lock and mint bridging protocol between L1 ethereum and the mev-commit chain.

## V1 High level design

V1 is intended to be as simple as possible, avoiding an intermediary validation network, on-chain light clients (as is used with IBC), or merkle attestations of cross chain messages. 

The v1 standard bridge will be built around a single agent type that assumes both the relayer and validator role. This will be referred to as the relayer node from now on.

To bridge to mev-commit chain, the user initiates a transaction to the contract on L1, which locks their ether on L1. The transaction should submit the necessary information to complete a cross chain transfer of funds. Importantly this transaction will emit an event which is subscribed to by the relayer.

The relayer is configured with it's own full-node for both L1, and the mev-commit chain. This can be replaced with a trusted rpc endpoint for testing.

The relayer listens to, and processes events residing from the contract on L1. Events will be handled in FIFO ordering, and would result in the data being relayed to the mev-commit chain, where native ether is minted. The destination contract accepts relay transactions only from the relayer EOA. More complex or decentralized attestation can be added in v2. 

Note to bridge from the mev-commit chain back to L1, the same protocol is used. Except mev-commit chain ether is burned upon initiating a bridge operation, and ether is unlocked on L1 upon bridge completion. Therefore the relayer should be concurrently monitoring both chains for events.

## V2 Notes

V2 of the standard bridge should incorporate merkle attestations of cross chain messages, and could possibly focus on a more decentralized bridging architecture. From some initial research, it seems like multiple bridging projects rely on merkle proof data being relayed across chains. The main difference seems to be what validator set is responsible for attesting to or managing the canonical merkle root to verify against.

V2 could also focus on having multiple validators.

Inspiration:

* Cosmos' IBC uses an on-chain light client to enable proofs of inclusion for particular values at particular paths on a remote blockchains. Essentially, the canonical merkle root is managed by an on-chain light client. See more [here](https://github.com/cosmos/ibc/tree/main/spec/core/ics-002-client-semantics). Note IBC won't work for our use-case but provides some inspiration.
* Hyperlane's multisig bridging protocol has an off-chain validator set (multisig) that attest to merkle root checkpoints of the subset of state that contains outgoing bridge messages from a source chain. A relayer agent subscribes to events related to these messages, and submits metadata (including merkle proof) to the destination chain. The destination chain contract logic verifies the relayed merkle proof data against the root attested to by the validators. See relevant contracts [here](https://github.com/hyperlane-xyz/hyperlane-monorepo/blob/5b4af6bf1db93102d54f114b03079cc873c08249/solidity/contracts/isms/multisig/AbstractMultisigIsm.sol) and [here](https://github.com/hyperlane-xyz/hyperlane-monorepo/blob/5b4af6bf1db93102d54f114b03079cc873c08249/solidity/contracts/isms/multisig/AbstractMerkleRootMultisigIsm.sol).
* Polygon POS bridging seems to piggyback off their state sync mechanism. To bridge from L1 to the sidechain, sidechain validators simply listen to events on an L1 contract and pass along this data to the sidechain as a part of the [state sync mechanism](https://docs.polygon.technology/pos/architecture/bor/state-sync/). To bridge from the sidechain back to L1, a tx must first reside on the sidechain. After some period of time, this tx is checkpointed on L1 by the sidechain validators. Once checkpointing is done, the hash of the transaction created on the sidechain is submitted with a proof to the `RootChainManager` contract on L1. This contract validates the transaction and associated merkle proof against the checkpointed root hash. That is, canonical merkle roots to verify against are posted to L1. See more about polygon bridging layers [here](https://docs.polygon.technology/pos/how-to/bridging/).
* For our implementation we may want to use [eth-getproof](https://docs.alchemy.com/reference/eth-getproof).
