# mev-commit bridge repo

This repository houses docker infra and cli tools for operating and interacting with a bridge between L1 ethereum and the mev-commit chain. The bridge is a [hyperlane warp route](https://docs.hyperlane.xyz/docs/protocol/warp-routes), with validators and a relayer operated by Primev.

## Bridge CLI

To use the `cli.sh` in bridging ether to or from the mev-commit chain, first make the script executable:

```bash
chmod +x cli.sh
```

Optionally, move the script to a folder in your `PATH` similar to:

```bash
sudo mv cli.sh /usr/local/bin/bridge-cli
```

Next we'll initialize bridge client parameters. Note all following commands display confirmation prompts. Use [primev docs](https://docs.primev.xyz/mev-commit-chain) to obtain relevant arguments. Router arguments are addresses of deployed hyperlane router contracts for each chain. Executing this command will save a `.bridge_config` json in the working directory:

```bash
bridge-cli init <L1 Router> <mev-commit chain Router> <L1 Chain ID> <mev-commit chain ID> <L1 URL> <MEV-Commit URL>
```

Once initialized, bridge ether to the mev-commit chain with

```bash
bridge-cli bridge-to-mev-commit <amount in wei> <dest_addr> <private_key>
```

Remember to bridge enough ether such that fees to bridge back to L1 can be paid! Bridge ether back to L1 with

```bash
bridge-cli bridge-to-l1 <amount in wei> <dest_addr> <private_key>
```

Note support for keystore and hardware wallets will be added later. 
