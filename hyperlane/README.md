# Hyperlane Warp Route Bridge

## Contract deployer

Address:    `0x82b941824b43F33e417be1E92273A3020a0D760c`

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

## User emulators

There are 5 emulator services that simulate EOA's bridging to/from the mev-commit chain. Use the Makefile to start them. 

Note all these accounts must be funded with Sepolia ether and enough mev-commit chain ether to pay for gas.

Emulator 1 Address: `0x04F713A0b687c84D4F66aCd1423712Af6F852B78`
Emulator 2 Address: `0x4E2D04c65C399Eb27B3E3ADA06110BCd47b5a506`
Emulator 3 Address: `0x7AEe7AD6b2EAd96532D84D20358Db0e697f060Cd`
Emulator 4 Address: `0x765235CDda5FC6a620Fea2208A333a97CEDA2E1d`
Emulator 5 Address: `0x163c7bD4C3B815B06503D8E8B5906519C319EA6f`

## Starter .env file
To get a standard starter .env file from primev internal development, [click here.](https://www.notion.so/Private-keys-and-env-for-settlement-layer-245a4f3f4fe040a7b72a6be91131d9c2?pvs=4). Note this repo is being actively developed and required .env variables may change.

Your .env file should look like:

```
HYPERLANE_DEPLOYER_PRIVATE_KEY=0xpk1
NODE1_PRIVATE_KEY=0xpk2
NODE2_PRIVATE_KEY=0xpk3
RELAYER_PRIVATE_KEY=0xpk4
NEXT_PUBLIC_WALLET_CONNECT_ID=0xcId
DD_API_KEY=432
DD_APP_KEY=808
EMULATOR1_PRIVATE_KEY=0xpk5
EMULATOR2_PRIVATE_KEY=0xpk6
EMULATOR3_PRIVATE_KEY=0xpk7
EMULATOR4_PRIVATE_KEY=0xpk8
EMULATOR5_PRIVATE_KEY=0xpk9
```
