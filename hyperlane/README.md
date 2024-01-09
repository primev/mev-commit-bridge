# Hyperlane Warp Route Bridge

## Contract deployer

Address:    `0xBcA333b67fb805aB18B4Eb7aa5a0B09aB25E5ce2`

Note if the relayer is emitting errors related to unexpected contract routing, try redeploying the hyperlane contracts using a new key pair. It's likely the current deployments are clashing with previous deployments on Sepolia.

To properly set a new hyperlane deployer:
* Generate a new key pair (ex: `cast wallet new`)
* Send or [mine](https://sepolia-faucet.pk910.de/) some Sepolia ETH to `Address`
* replace `Address` above for book keeping
* replace `CONTRACT_DEPLOYER_PRIVATE_KEY` in `.env`
* allocate funds to `Address` in the allocs field of `genesis.json`

Note the deployer of [primev contracts](https://github.com/primevprotocol/contracts) can be a separate account.

## Validator Accounts (same keys as POA signers)

### Node1

Address:     `0xd9cd8E5DE6d55f796D980B818D350C0746C25b97`

### Node2

Address:     `0x788EBABe5c3dD422Ef92Ca6714A69e2eabcE1Ee4`

## Relayer

Address:     `0x0DCaa27B9E4Db92F820189345792f8eC5Ef148F6`

## User emulator

Address:     `0x04F713A0b687c84D4F66aCd1423712Af6F852B78`

## Starter .env file
To get a standard starter .env file from primev internal development, [click here.](https://www.notion.so/Private-keys-and-env-for-settlement-layer-245a4f3f4fe040a7b72a6be91131d9c2?pvs=4)

Otherwise your .env file should look like:

```
HYPERLANE_DEPLOYER_PRIVATE_KEY=0xpk1
NODE1_PRIVATE_KEY=0xpk2
NODE2_PRIVATE_KEY=0xpk3
RELAYER_PRIVATE_KEY=0xpk4
EMULATOR_PRIVATE_KEY=0xpk5
NEXT_PUBLIC_WALLET_CONNECT_ID=0xcId
DD_API_KEY=432
DD_APP_KEY=808
```
