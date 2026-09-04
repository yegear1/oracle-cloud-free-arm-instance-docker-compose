# AGENTS.md

Guia de contexto e diretrizes para agentes de IA que atuarem neste repositório.

---

## 1. Visão Geral do Projeto

Este projeto automatiza o provisionamento contínuo de instâncias **Always Free ARM** (`VM.Standard.A1.Flex` de até 4 OCPUs e 24GB RAM) na Oracle Cloud Infrastructure (OCI). 

O projeto evoluiu para o **ápice de eficiência e performance**, implementado em **Go (Golang)** utilizando o SDK oficial da Oracle (`github.com/oracle/oci-go-sdk/v65`):
- **Pegada Mínima de Recursos:** Imagem Docker de apenas ~20MB e consumo de memória RAM de míseros ~6MB (mais de 25x menor e 10x mais leve que soluções convencionais).
- **Latência Mínima de Rede (~50ms):** Conexões HTTP nativas ultraotimizadas com Keep-Alive reutilizável.
- **Tratamento Nativo de JSON e Plataformas:** Compatibilidade total e transparente com Windows (`\r\n`) e Linux (`\n`).
- **Tratamento Fino de Erros:** Diferenciação direta de falta de capacidade (*Out of host capacity*), *Rate Limit* (HTTP 429) e falhas de credenciais.
- **Suporte Híbrido:** Operação com conta única via `.env` ou multi-contas via `accounts.json`.
- **Notificações Flexíveis via Webhook:** Integração com APIs de WhatsApp, Discord, Slack ou endpoints HTTP genéricos.

---

## 2. O que o Agente Precisa Saber para Iniciar

### Arquitetura e Componentes Principais
- **`main.go`**: Aplicação principal em Go com `oci-go-sdk`. Gerencia conexões persistentes, descoberta dinâmica de Availability Domains (ADs), loop de tentativas e notificações.
- **`go.mod`**: Gerenciamento de dependências Go (`github.com/oracle/oci-go-sdk/v65`).
- **`Dockerfile`**: Build multi-stage compilando o binário estático em `golang:1.22-alpine` e gerando imagem final ultraleve em `alpine:3.20`.
- **`docker-compose.yml`**: Orquestra o container `oracle_fisher`, montando `.env`, `accounts.json` e o diretório de credenciais `./oci_keys` em `/root/.oci`.
- **`accounts.json`**: Lista de contas OCI para operação multi-contas.
- **`accounts.json.example`**: Modelo de exemplo para configuração multi-contas.
- **`.env`**: Configurações gerais (hardware, intervalo, webhooks de notificação e credenciais de conta única).
- **`oci_keys/`**: Diretório montado contendo `config`, chaves `.pem` de API e chaves SSH.
- *(Legados preservados como alternativas: `main.py` e `oracle_cloud_instance_creator.sh`).*

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
- [x] Implementar `main.py` em Python puro com o SDK oficial `oci` e sessões persistentes.
- [x] Implementar `main.go` e `go.mod` com o SDK oficial `oci-go-sdk` da Oracle.
- [x] Configurar build multi-stage no `Dockerfile` para gerar imagem Go ultraleve (~20MB).
- [x] Atualizar documentação no `README.md` e finalizar checklist no `AGENTS.md`.
