# AGENTS.md

Guia de contexto e diretrizes para agentes de IA que atuarem neste repositório.

---

## 1. Visão Geral do Projeto

Este projeto automatiza o provisionamento contínuo de instâncias **Always Free ARM** (`VM.Standard.A1.Flex` de até 4 OCPUs e 24GB RAM) na Oracle Cloud Infrastructure (OCI). Devido à alta demanda, a criação direta costuma retornar erro de falta de capacidade (*Out of host capacity*). O script roda dentro de um container Docker em loop contínuo (intervalo padrão de 60 segundos) até conseguir alocar a máquina.

---

## 2. O que o Agente Precisa Saber para Iniciar

### Arquitetura e Componentes Principais
- **`docker-compose.yml`**: Orquestra o container `oracle_fisher`, mapeando `.env` para `/app/.env` e o diretório de credenciais `./oci_keys` para `/root/.oci`.
- **`Dockerfile`**: Base `python:3.9-slim`, instala `oci-cli`, `jq`, `curl`. Possui um wrapper `run.sh` que converte quebras de linha Windows (CRLF para LF) do `.env` antes da execução.
- **`oracle_cloud_instance_creator.sh`**: Script bash principal. Testa a conexão com a OCI (`oci iam compartment list`) e executa o loop `while true` com `oci compute instance launch --no-retry`.
- **`oci_keys/`**: Diretório que armazena:
  - `config`: Arquivo de configuração da OCI CLI (apontando `key_file=/root/.oci/oracle_api_key.pem`).
  - `oracle_api_key.pem`: Chave privada da API da Oracle.
  - `chave_vps_arm` e `chave_vps_arm.pub`: Par de chaves SSH para acesso à VPS criada.
- **`.env`**: Parâmetros da OCI (`TENANCY_ID`, `IMAGE_ID`, `SUBNET_ID`, `AVAILABILITY_DOMAIN`, recursos de hardware e webhooks de notificação).

### Comandos de Operação
- Iniciar em background: `docker compose up -d --build`
- Monitorar logs: `docker compose logs -f`
- Encerrar processo: `docker compose down`

### Padrão de Commits
- Seguir estritamente **Conventional Commits** (`feat:`, `fix:`, `docs:`, `refactor:`, `chore:`, etc.).
- Commits atômicos e focados.

---

## 3. Tasks do Projeto

- [x] Inicializar documentação `AGENTS.md` com contexto e diretrizes.
- [x] Corrigir erro de sintaxe no `.env` (aspas duplicadas).
- [x] Parametrizar recursos de hardware (`cpus`, `ram`, `bootVolume`) e intervalo a partir do `.env` com fallbacks.
- [x] Adicionar timestamp e contador de tentativas nos logs do script.
- [x] Implementar sistema flexível de notificação via webhook (WhatsApp, Discord, Telegram ou customizado).
- [x] Atualizar `.env` e `README.md` documentando todas as novas opções de configuração.
