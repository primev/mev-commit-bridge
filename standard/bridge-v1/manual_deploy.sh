#!/bin/sh

L1_CHAIN_ID=${L1_CHAIN_ID:-"17000"} # Holesky
L1_RPC_URL=${L1_RPC_URL:-"https://ethereum-holesky.publicnode.com"}
SETTLEMENT_CHAIN_ID=${SETTLEMENT_CHAIN_ID:-"17864"}
SETTLEMENT_DEPLOYER_PRIVKEY=${DEPLOYER_PRIVKEY:-"0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"} # Same default deployer as core contracts

fail_if_not_set() {
    if [ -z "$1" ]; then
        echo "Error: Required environment variable not set (one of SETTLEMENT_RPC_URL, L1_DEPLOYER_PRIVKEY, and RELAYER_ADDR)"
        exit 1
    fi
}
fail_if_not_set "${SETTLEMENT_RPC_URL}"
fail_if_not_set "${L1_DEPLOYER_PRIVKEY}"
fail_if_not_set "${RELAYER_ADDR}"

CONTRACTS_PATH=${CONTRACTS_PATH:-"$HOME/.primev/contracts"}
if [ ! -d "$CONTRACTS_PATH" ]; then
    echo "Error: Contracts path not found at $CONTRACTS_PATH. Please ensure the contracts are installed and the path is correct."
    exit 1
fi

check_foundry_installed() {
    if ! command -v cast > /dev/null 2>&1; then
        echo "Error: Foundry's 'cast' command not found. Please install Foundry and ensure it is in your PATH."
        exit 1
    fi
    if ! command -v forge > /dev/null 2>&1; then
        echo "Error: Foundry's 'forge' command not found. Please install Foundry and ensure it is in your PATH."
        exit 1
    fi
    echo "Foundry CLI tools 'cast' and 'forge' are both installed."
}

check_chain_id() {
    RPC_URL="$1"
    EXPECTED_CHAIN_ID="$2"
    RETRIEVED_CHAIN_ID=$(cast chain-id --rpc-url "$RPC_URL")
    if [ "$RETRIEVED_CHAIN_ID" -ne "$EXPECTED_CHAIN_ID" ]; then
        echo "Error: Chain ID mismatch for $RPC_URL. Expected: $EXPECTED_CHAIN_ID, Got: $RETRIEVED_CHAIN_ID"
        exit 1
    else
        echo "Success: Chain ID for $RPC_URL matches the expected ID: $EXPECTED_CHAIN_ID"
    fi
}

check_create2() {
    RPC_URL="$1"
    CREATE2_AADR="0x4e59b44847b379578588920ca78fbf26c0b4956c"
    CODE=$(cast code --rpc-url "$RPC_URL" $CREATE2_AADR)
    if [ -z "$CODE" ] || [ "$CODE" = "0x" ]; then
        echo "Create2 proxy not deployed on $RPC_URL"
        exit 1
    else
        echo "Create2 proxy deployed on $RPC_URL"
    fi
}

check_balance() {
    RPC_URL="$1"
    ADDR="$2"
    BALANCE_WEI=$(cast balance "$ADDR" --rpc-url "$RPC_URL")
    ONE_ETH_WEI="1000000000000000000"

    SUFFICIENT=$(echo "$BALANCE_WEI >= $ONE_ETH_WEI" | bc)
    if [ "$SUFFICIENT" -eq 0 ]; then
        echo "Error: $ADDR has insufficient balance on chain with RPC URL $RPC_URL. Balance: $BALANCE_WEI wei"
        exit 1
    else
        echo "Confirmed: $ADDR has sufficient balance (>= 1 ETH) on chain with RPC URL $RPC_URL. Balance: $BALANCE_WEI wei"
    fi
}

check_foundry_installed

check_chain_id "$L1_RPC_URL" "$L1_CHAIN_ID"
check_chain_id "$SETTLEMENT_RPC_URL" "$SETTLEMENT_CHAIN_ID"

check_create2 "$L1_RPC_URL"
check_create2 "$SETTLEMENT_RPC_URL"

L1_DEPLOYER_ADDR=$(cast wallet address "$L1_DEPLOYER_PRIVKEY")
check_balance "$L1_RPC_URL" "$L1_DEPLOYER_ADDR"

SETTLEMENT_DEPLOYER_ADDR=$(cast wallet address "$SETTLEMENT_DEPLOYER_PRIVKEY")
check_balance "$SETTLEMENT_RPC_URL" "$SETTLEMENT_DEPLOYER_ADDR"

check_balance "$L1_RPC_URL" "$RELAYER_ADDR"

cast send \
    --rpc-url "$SETTLEMENT_RPC_URL" \
    --private-key "$SETTLEMENT_DEPLOYER_PRIVKEY" \
    "$RELAYER_ADDR" \
    --value 100ether

check_balance "$SETTLEMENT_RPC_URL" "$RELAYER_ADDR"

EXPECTED_WHITELIST_ADDR="0x57508f0B0f3426758F1f3D63ad4935a7c9383620"

check_balance "$SETTLEMENT_RPC_URL" "$EXPECTED_WHITELIST_ADDR"

SCRIPTS_PATH_PREFIX="$CONTRACTS_PATH/scripts/"

echo "changing directory to $CONTRACTS_PATH and running deploy scripts for standard bridge"
cd "$CONTRACTS_PATH" || exit

RELAYER_ADDR="$RELAYER_ADDR" forge script \
    "${SCRIPTS_PATH_PREFIX}"DeployStandardBridge.s.sol:DeploySettlementGateway \
    --rpc-url "$SETTLEMENT_RPC_URL" \
    --private-key "$SETTLEMENT_DEPLOYER_PRIVKEY" \
    --broadcast \
    --chain-id "$SETTLEMENT_CHAIN_ID" \
    -vvvv \
    --use 0.8.23

RELAYER_ADDR="$RELAYER_ADDR" forge script \
    "${SCRIPTS_PATH_PREFIX}"DeployStandardBridge.s.sol:DeployL1Gateway \
    --rpc-url "$L1_RPC_URL" \
    --private-key "$L1_DEPLOYER_PRIVKEY" \
    --broadcast \
    --chain-id "$L1_CHAIN_ID" \
    -vvvv \
    --use 0.8.23
