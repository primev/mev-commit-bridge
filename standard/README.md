# Standard bridge

## High level design

The standard bridge is built around a single agent type that assumes both the relayer and validator role. This will be referred to as the relayer node from now on.

To invoke a bridge operation, the user first sends a transaction to a contract on the source chain. The transaction should submit the necessary information to complete a cross chain transfer of funds. Importantly this transaction will emit an event which is subscribed to by the relayer.

The relayer is configured with it's own full-node for both L1, and the mev-commit chain. This can be replaced with infura/alchemy endpoints for testing.

The relayer listens to, and indexes events residing from a contract that is deployed to both L1 ethereum and the mev-commit chain. These events will be handled in FIFO ordering. In handling an event, the relayer enacts the following steps:

* TODO: clean below up. You need to compare hashes returned, to expected state root from latest block. Can retrieve latest block root hash from multiple sources? 
* Queries its trusted node for relevant merkle hashes using [eth_getProof]https://docs.alchemy.com/reference/eth-getproof rpc method that 
* there's a valid commitment of ether locked on the source chain.
* Computes a merkle proof that the commitment is valid against a queried state root. 


TODO: Note on multiple relayers running as multisig vs just one.
