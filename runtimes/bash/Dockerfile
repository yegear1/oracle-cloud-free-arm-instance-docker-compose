FROM python:3.9-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    jq \
    curl \
    bash \
    coreutils \
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir oci-cli

COPY oracle_cloud_instance_creator.sh entrypoint.sh ./
RUN sed -i 's/\r$//' oracle_cloud_instance_creator.sh entrypoint.sh && \
    chmod +x oracle_cloud_instance_creator.sh entrypoint.sh

CMD ["./entrypoint.sh"]
