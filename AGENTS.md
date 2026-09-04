# AGENTS.md

Guia de contexto e diretrizes para agentes de IA que atuarem neste repositório.

---

## 1. Visão Geral do Projeto

Este projeto automatiza o provisionamento contínuo de instâncias **Always Free ARM** (`VM.Standard.A1.Flex` de até 4 OCPUs e 24GB RAM) na Oracle Cloud Infrastructure (OCI). Devido à alta demanda, a criação direta costuma retornar erro de falta de capacidade (*Out of host capacity*). O script roda dentro de um container Docker em loop contínuo (intervalo padrão de 60 segundos) até conseguir alocar as máquinas para as contas configuradas, percorrendo dinamicamente as Zonas de Disponibilidade (Availability Domains).

---

## 2. O que o Agente Precisa Saber para Iniciar

### Arquitetura e Componentes Principais
- **`docker-compose.yml`**: Orquestra o container `oracle_fisher`, montando `.env`, `accounts.json` e o diretório de credenciais `./oci_keys` para `/root/.oci`.
- **`Dockerfile` e `entrypoint.sh`**: Base `python:3.9-slim`, instala `oci-cli`, `jq`, `curl`. O `entrypoint.sh` sanitiza quebras de linha Windows (CRLF para LF) antes da inicialização.
- **`oracle_cloud_instance_creator.sh`**: Script bash principal com:
  - Suporte a múltiplas contas via `accounts.json` (com fallback para `.env` no caso de conta única).
  - Descoberta dinâmica de Availability Domains (ADs) por região.
  - Controle de estado por conta via mapa associativo (`declare -A success_accounts`).
  - Notificação instantânea via webhook (WhatsApp API, Discord, etc.) ao criar cada máquina.
  - Logs detalhados com timestamp e contador de ciclos.
- **`oci_keys/`**: Diretório que armazena:
  - `config`: Arquivo de configuração da OCI CLI (com perfis `[DEFAULT]`, `[ACCOUNT1]`, etc.).
  - `oracle_api_key*.pem`: Chave(s) privada(s) da API da Oracle.
  - `vps_ssh_key` e `vps_ssh_key.pub`: Par de chaves SSH para acesso à VPS criada.
- **`accounts.json`**: Lista de contas para criação simultânea.
- **`.env`**: Configurações globais (hardware, intervalo, webhooks de notificação).

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
- [ ] Refatorar Dockerfile e criar `entrypoint.sh` dedicado para inicialização e higienização de CRLF.
- [ ] Atualizar `docker-compose.yml` para mapear `accounts.json` e `.env`.
- [ ] Criar `accounts.json.example` e padronizar chaves SSH.
- [ ] Implementar suporte a múltiplas contas e busca dinâmica de Availability Domains (ADs) no script principal.
- [ ] Atualizar documentação no `README.md` e marcar tasks finalizadas no `AGENTS.md`.
