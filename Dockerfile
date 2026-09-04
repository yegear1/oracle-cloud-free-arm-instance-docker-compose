FROM python:3.9-slim

WORKDIR /app

# Instala dependências do Python (OCI SDK e requests)
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copia aplicação principal em Python
COPY main.py ./

# Mantém scripts legados disponíveis como fallback
COPY oracle_cloud_instance_creator.sh entrypoint.sh ./
RUN chmod +x oracle_cloud_instance_creator.sh entrypoint.sh

# Executa a aplicação nativa com saída unbuffered para logs em tempo real
CMD ["python", "-u", "main.py"]