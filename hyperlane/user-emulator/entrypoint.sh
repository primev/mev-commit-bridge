#!/bin/sh
set -exu

sleep 10

# dog --help

# Construct .dogrc file from env vars
cat > /.dogrc <<EOF
[Connection]
apikey = $DD_API_KEY
appkey = $DD_APP_KEY
EOF

echo ".dogrc file created successfully."

# dog --config /.dogrc metric post testjan8 100

# Init bridge 
SEPOLIA_ROUTER=$(cat /deploy-artifacts/warp-deployment.json | jq -r '.sepolia.router')
MEV_COMMIT_CHAIN_ROUTER=$(cat /deploy-artifacts/warp-deployment.json | jq -r '.mevcommitsettlement.router')
SEPOLIA_CHAIN_ID=11155111
MEV_COMMIT_CHAIN_ID=17864
SEPOLIA_URL=https://ethereum-sepolia.publicnode.com
MEV_COMMIT_CHAIN_URL=${SETTLEMENT_RPC_URL}

bridge-cli init \
    ${SEPOLIA_ROUTER} ${MEV_COMMIT_CHAIN_ROUTER} \
    ${SEPOLIA_CHAIN_ID} ${MEV_COMMIT_CHAIN_ID} \
    ${SEPOLIA_URL} ${MEV_COMMIT_CHAIN_URL} \
    --yes

# bridge-cli --help

# Bridge addr to use is 0x04F713A0b687c84D4F66aCd1423712Af6F852B78
