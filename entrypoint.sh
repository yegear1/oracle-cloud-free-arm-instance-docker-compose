#!/bin/bash
set -e

echo "--- Iniciando Container Oracle Fisher ---"

# Sanitiza arquivos de configuração montados para remover caracteres Windows (CRLF)
if [ -f .env ]; then
    tr -d '\r' < .env > .env.tmp && mv .env.tmp .env
fi

if [ -f accounts.json ]; then
    tr -d '\r' < accounts.json > accounts.json.tmp && mv accounts.json.tmp accounts.json
fi

echo "--- Iniciando Script de Criacao de Instancias ---"
exec ./oracle_cloud_instance_creator.sh
