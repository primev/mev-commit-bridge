ARG AGENT_BASE_IMAGE
FROM ${AGENT_BASE_IMAGE}

# Relayer config, see: 
# - https://docs.hyperlane.xyz/docs/operators/relayers/setup
# - https://docs.hyperlane.xyz/docs/operators/agent-configuration#config-layers

ENV CONFIG_FILES="/deploy-artifacts/agent-config.json"

# ENV HYP_RELAYER_TRACING_FMT="full"
# ENV HYP_RELAYER_TRACING_LEVEL="trace"

# TODO: enforce minimum gas payment in prod

CMD ./relayer \
    --relayChains mevcommitsettlement,sepolia \
    --chains.mevcommitsettlement.connection.url ${SETTLEMENT_RPC_URL} \
    --chains.sepolia.connection.url ${SEPOLIA_RPC_URL} \
    --db /relayer-db \
    --defaultSigner.key $RELAYER_PRIVATE_KEY \
    --allowLocalCheckpointSyncers true \
    --gasPaymentEnforcement '[{"type": "none"}]' \
    --tracing.fmt "full" \
    --tracing.level "trace"
