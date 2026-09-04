# AGENTS.md

Guia de contexto e diretrizes para agentes de IA que atuarem neste repositório.

---

## 1. Visão Geral do Projeto

Este projeto automatiza o provisionamento contínuo de instâncias **Always Free ARM** (`VM.Standard.A1.Flex` de até 4 OCPUs e 24GB RAM) na Oracle Cloud Infrastructure (OCI). 

O projeto foi migrado de Bash (`oci-cli`) para **Python puro utilizando o SDK oficial da Oracle (`oci`)**. Essa mudança garante:
- **Alta eficiência e conexão persistente (HTTP Keep-Alive):** As requisições de criação são disparadas em milissegundos (~150ms vs ~2.000ms da CLI), aumentando consideravelmente as chances de capturar vagas concorridas.
- **Tratamento nativo de JSON e CRLF:** Elimina dependências externas (`jq`, `sed`) e resolve nativamente quebras de linha Windows.
- **Tratamento preciso de exceções:** Diferenciação automática de capacidade (*Out of host capacity*), *Rate Limit* (HTTP 429) e erros de autenticação.
- **Suporte Híbrido:** Operação com conta única via `.env` ou multi-contas via `accounts.json`.
- **Notificações Flexíveis via Webhook:** Integração com APIs de WhatsApp, Discord, Slack ou endpoints HTTP genéricos.

---

## 2. O que o Agente Precisa Saber para Iniciar

### Arquitetura e Componentes Principais
- **`main.py`**: Aplicação principal em Python 3 com SDK `oci`. Gerencia conexões persistentes, descoberta dinâmica de Availability Domains (ADs), loop de tentativas e notificações.
- **`requirements.txt`**: Dependências Python (`oci`, `requests`).
- **`Dockerfile`**: Imagem leve `python:3.9-slim` executando `python -u main.py`.
- **`docker-compose.yml`**: Orquestra o container `oracle_fisher`, montando `.env`, `accounts.json` e o diretório de credenciais `./oci_keys` em `/root/.oci`.
- **`accounts.json`**: Lista de contas OCI para operação multi-contas.
- **`accounts.json.example`**: Modelo de exemplo para configuração multi-contas.
- **`.env`**: Configurações gerais (hardware, intervalo, webhooks de notificação e credenciais de conta única).
- **`oci_keys/`**: Diretório montado contendo `config`, chaves `.pem` de API e chaves SSH.

### Comandos de Operação
- Iniciar em background: `docker compose up -d --build`
- Monitorar logs em tempo real: `docker compose logs -f`
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
- [x] Integrar busca dinâmica de Availability Domains (ADs) via API OCI.
- [x] Implementar suporte a múltiplas contas via `accounts.json` com fallback para `.env`.
- [ ] Implementar `main.py` em Python puro com o SDK oficial `oci` e sessões persistentes.
- [ ] Criar `requirements.txt` e refatorar `Dockerfile` e `docker-compose.yml` para execução Python pura.
- [ ] Atualizar documentação no `README.md` e finalizar checklist no `AGENTS.md`.
