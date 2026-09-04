FROM python:3.9-slim

WORKDIR /app

# Instala dependências do sistema
RUN apt-get update && apt-get install -y \
    jq \
    curl \
    bash \
    coreutils \
    && rm -rf /var/lib/apt/lists/*

# Instala a CLI oficial da Oracle Cloud
RUN pip install --no-cache-dir oci-cli

# Copia scripts da aplicação
COPY oracle_cloud_instance_creator.sh entrypoint.sh ./

# Garante permissões de execução e quebras de linha padrão Linux (LF)
RUN sed -i 's/\r$//' oracle_cloud_instance_creator.sh entrypoint.sh && \
    chmod +x oracle_cloud_instance_creator.sh entrypoint.sh

CMD ["./entrypoint.sh"]