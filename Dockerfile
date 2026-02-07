FROM python:3.9-slim

RUN apt-get update && apt-get install -y \
    jq \
    curl \
    bash \
    coreutils \
    && rm -rf /var/lib/apt/lists/*

RUN pip install oci-cli

WORKDIR /app

COPY oracle_cloud_instance_creator.sh .

RUN sed -i 's/\r$//' oracle_cloud_instance_creator.sh && chmod +x oracle_cloud_instance_creator.sh

RUN echo '#!/bin/bash' > /app/run.sh && \
    echo 'echo "--- Iniciando Limpeza de Arquivos Windows ---"' >> /app/run.sh && \
    echo '# Remove o caractere \r do arquivo .env montado e salva no .env.clean' >> /app/run.sh && \
    echo 'tr -d "\r" < .env > .env.clean' >> /app/run.sh && \
    echo '# Cria uma copia do script principal que usa o .env limpo' >> /app/run.sh && \
    echo 'sed "s/source .env/source .env.clean/" oracle_cloud_instance_creator.sh > script_safe.sh' >> /app/run.sh && \
    echo 'chmod +x script_safe.sh' >> /app/run.sh && \
    echo 'echo "--- Limpeza Concluida. Iniciando Script ---"' >> /app/run.sh && \
    echo './script_safe.sh' >> /app/run.sh && \
    chmod +x /app/run.sh

CMD ["/app/run.sh"]