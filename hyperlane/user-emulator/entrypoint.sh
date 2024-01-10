#!/bin/sh
set -exu

sleep 10

# Construct .dogrc file from env vars
cat > /.dogrc <<EOF
[Connection]
apikey = $DD_API_KEY
appkey = $DD_APP_KEY
EOF

echo ".dogrc file created successfully."

# Fail script if no warp deployment file is found
if [ ! -f /deploy-artifacts/warp-deployment.json ]; then
    echo "Error: warp-deployment.json not found. Please deploy bridge."
    exit 1
fi

# Init bridge client
SEPOLIA_ROUTER=$(cat /deploy-artifacts/warp-deployment.json | jq -r '.sepolia.router')
MEV_COMMIT_CHAIN_ROUTER=$(cat /deploy-artifacts/warp-deployment.json | jq -r '.mevcommitsettlement.router')
SEPOLIA_CHAIN_ID=11155111
MEV_COMMIT_CHAIN_ID=17864
SEPOLIA_URL=https://ethereum-sepolia.publicnode.com
MEV_COMMIT_CHAIN_URL=${SETTLEMENT_RPC_URL}

# Funded account for bridge testing
EMULATOR_ADDRESS=0x04F713A0b687c84D4F66aCd1423712Af6F852B78

# Ensure balances on both chains are above 1 ETH
L1_BALANCE=$(cast balance --rpc-url $SEPOLIA_URL $EMULATOR_ADDRESS)
MEV_COMMIT_BALANCE=$(cast balance --rpc-url $MEV_COMMIT_CHAIN_URL $EMULATOR_ADDRESS)
MIN_BALANCE="1000000000000000000"  # 1.0 ether in wei
if [ "$(echo "$L1_BALANCE < $MIN_BALANCE" | bc)" -eq 1 ]; then
    echo "$EMULATOR_ADDRESS must be funded with at least 1.0 ether on Sepolia."
    exit 1
fi
if [ "$(echo "$MEV_COMMIT_BALANCE < $MIN_BALANCE" | bc)" -eq 1 ]; then
    echo "$EMULATOR_ADDRESS must be funded with at least 1.0 ether on mev-commit chain."
    exit 1
fi

bridge-cli init \
    ${SEPOLIA_ROUTER} ${MEV_COMMIT_CHAIN_ROUTER} \
    ${SEPOLIA_CHAIN_ID} ${MEV_COMMIT_CHAIN_ID} \
    ${SEPOLIA_URL} ${MEV_COMMIT_CHAIN_URL} \
    --yes

function bridge_and_post_metric() {
    SUB_CMD=$1
    CHAIN_ID=$2
    AMOUNT=$3

    bridge-cli $SUB_CMD $AMOUNT $EMULATOR_ADDRESS $EMULATOR_PRIVATE_KEY --yes
    exit_code=$?

    if [ $exit_code -eq 0 ]; then
        echo "Bridged $AMOUNT to Chain $CHAIN_ID successfully."
        BRIDGING_SUCCESS=1
    else
        echo "Failed to bridge $AMOUNT to Chain $CHAIN_ID."
        BRIDGING_SUCCESS=0
    fi

    dog --config /.dogrc metric post bridging.success $BRIDGING_SUCCESS --tags="amount_bridged:$AMOUNT,to_chain_id:$CHAIN_ID"
}

while true; do
    # Generate a random amount between 0 and 10000 wei
    RANDOM_AMOUNT=$(( (RANDOM % 10001) ))

    bridge_and_post_metric "bridge-to-mev-commit" $MEV_COMMIT_CHAIN_ID $RANDOM_AMOUNT
    bridge_and_post_metric "bridge-to-l1" $SEPOLIA_CHAIN_ID $RANDOM_AMOUNT
    sleep 10
done