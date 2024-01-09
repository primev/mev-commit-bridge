FROM python:3.9-alpine

RUN apk add --no-cache bash jq

RUN pip install datadog

COPY cli.sh /usr/local/bin/bridge-cli
RUN chmod +x /usr/local/bin/bridge-cli

COPY hyperlane/user-emulator/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# TODO: DD keys

ENTRYPOINT [ "/entrypoint.sh" ]
