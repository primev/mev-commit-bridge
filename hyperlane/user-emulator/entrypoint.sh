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

# TODO: incorperate metric posting and continuous briding
# dog --help
# dog --config /.dogrc metric post testjan8 100

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

# Bridge to self on mev-commit chain
bridge-cli bridge-to-mev-commit 890 $EMULATOR_ADDRESS $EMULATOR_PRIVATE_KEY --yes

# Bridge back to L1. Account must be prefunded on mev-commit chain. 
bridge-cli bridge-to-l1 890 $EMULATOR_ADDRESS $EMULATOR_PRIVATE_KEY --yes
