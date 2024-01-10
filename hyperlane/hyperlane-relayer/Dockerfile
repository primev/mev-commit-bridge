ARG AGENT_BASE_IMAGE
ARG SETTLEMENT_RPC_URL
FROM ${AGENT_BASE_IMAGE}

# Relayer config, see: 
# - https://docs.hyperlane.xyz/docs/operators/relayers/setup
# - https://docs.hyperlane.xyz/docs/operators/agent-configuration#config-layers

ENV CONFIG_FILES="/deploy-artifacts/agent-config.json"

# TODO: enforce minimum gas payment in prod

CMD ./relayer \
    --relayChains mevcommitsettlement,sepolia \
    --chains.mevcommitsettlement.connection.url ${SETTLEMENT_RPC_URL} \
    --chains.sepolia.connection.url "https://sepolia.infura.io/v3/104a92cc9950435facd5cdab911b9778" \
    --db /relayer-db \
    --defaultSigner.key $RELAYER_PRIVATE_KEY \
    --allowLocalCheckpointSyncers true \
    --gasPaymentEnforcement '[{"type": "none"}]'
