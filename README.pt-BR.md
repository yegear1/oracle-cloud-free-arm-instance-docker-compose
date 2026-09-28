[English](README.md) | [Português (Brasil)](README.pt-BR.md)

# Criador de Instância ARM Gratuita na Oracle Cloud (Docker)

Este projeto automatiza a criação de instâncias Always Free ARM (até 2 OCPUs e 12 GB de RAM) na Oracle Cloud Infrastructure (OCI). Escrito em **Go (Golang)** com o **SDK oficial da Oracle para Go**, busca a máxima eficiência: um binário estático compilado, em um contêiner de ~20 MB, usando apenas **~6 MB de RAM**, com conexões HTTP Keep-Alive persistentes e latência de requisição de ~50 ms.

Por causa da alta demanda, a criação de instâncias ARM costuma retornar o erro *Out of host capacity*. Este bot roda continuamente em segundo plano, tentando criar a instância em todos os Availability Domains (ADs) disponíveis até surgir capacidade.

### Recursos principais
* **Motor Go de alta performance:** Usa o `oci-go-sdk` oficial, reduzindo o uso de memória a ~6 MB e a latência de requisição a ~50 ms com HTTP Keep-Alive.
* **Build Docker multi-stage ultraleve:** Gera uma imagem mínima de ~20 MB baseada em `alpine:3.20`.
* **Conta única e múltiplas contas:** Uma conta via `.env` ou várias contas ao mesmo tempo via `accounts.json`.
* **Availability Domains dinâmicos:** Descobre e percorre automaticamente todos os ADs da região (`oci iam availability-domain list`), aumentando a chance de criação.
* **Notificações imediatas:** Alertas via WhatsApp (Evolution API, Z-API, Baileys etc.), Discord, Slack ou webhooks personalizados assim que o provisionamento é concluído.
* **Hardware configurável:** Ajuste de OCPUs, RAM e volume de boot por conta ou de forma global.
* **Multiplataforma e sem dependências:** Compila em um único binário estático, sem runtime externo ou scripts adicionais.
* **Estrutura organizada:** Chaves e arquivos de configuração isolados no diretório `oci_keys`.

---

## Pré-requisitos
* Uma ou mais contas ativas na Oracle Cloud.
* **Docker** e **Docker Compose** instalados na máquina.

---

## Instruções de configuração

### 1. Preparar credenciais (diretório `oci_keys`)
Todas as credenciais ficam no diretório `oci_keys`, na raiz do projeto.

#### Passo A: Chave de API da Oracle
1. Entre no Console da Oracle Cloud.
2. Acesse **My Profile** -> **API Keys** -> **Add API Key**.
3. Selecione **Generate API Key Pair** e baixe a chave privada.
4. Salve o arquivo como `oracle_api_key.pem` na pasta `oci_keys` (ou `oracle_api_key_<profile>.pem` para várias contas).
5. Clique em **Add**.
6. Copie o conteúdo exibido na caixa "Configuration File Preview".

#### Passo B: Arquivo de configuração
1. Crie ou edite `oci_keys/config` (sem extensão).
2. Cole o trecho de configuração da OCI. O `key_file` deve apontar para o caminho do contêiner `/root/.oci/...`.

**Exemplo de conta única (`[DEFAULT]`):**
```ini
[DEFAULT]
user=ocid1.user.oc1..aaaa...
fingerprint=xx:xx:xx:xx...
tenancy=ocid1.tenancy.oc1..aaaa...
region=sa-saopaulo-1
key_file=/root/.oci/oracle_api_key.pem
```

**Exemplo de várias contas (`[ACCOUNT1]`, `[ACCOUNT2]`):**
```ini
[ACCOUNT1]
user=ocid1.user.oc1..aaaa...
fingerprint=11:22:33...
tenancy=ocid1.tenancy.oc1..aaaa...
region=sa-saopaulo-1
key_file=/root/.oci/oracle_api_key_acc1.pem

[ACCOUNT2]
user=ocid1.user.oc1..bbbb...
fingerprint=44:55:66...
tenancy=ocid1.tenancy.oc1..bbbb...
region=sa-vinhedo-1
key_file=/root/.oci/oracle_api_key_acc2.pem
```

#### Passo C: Chaves SSH (acesso à VPS)
Gere um par de chaves SSH dentro da pasta `oci_keys`. Essa chave será usada para acessar a instância depois da criação.

```bash
ssh-keygen -t rsa -b 4096 -f ./oci_keys/chave_vps_arm
```
> **Nota:** Não defina passphrase se quiser o processo totalmente automatizado, sem prompts.

---

### 2. Opções de configuração

O projeto aceita um de dois modos:

#### Opção A: Conta única (via `.env`)
Copie `examples/.env.example` para `.env` na raiz do projeto e preencha os IDs da Oracle Cloud (OCIDs):
```bash
cp examples/.env.example .env
```

```bash
TENANCY_ID="ocid1.tenancy.oc1..aaaaaaa..."
IMAGE_ID="ocid1.image.oc1.sa-saopaulo-1..."
SUBNET_ID="ocid1.subnet.oc1.sa-saopaulo-1..."
PATH_TO_PUBLIC_SSH_KEY="/root/.oci/chave_vps_arm.pub"

# Fallback opcional (os ADs são descobertos automaticamente pela API da OCI)
# AVAILABILITY_DOMAIN="Uocm:SA-SAOPAULO-1-AD-1"

# Recursos de hardware (Always Free Ampere A1: 2 OCPUs, 12 GB de RAM)
cpus=2
ram=12
bootVolume=100
requestInterval=60
```

#### Opção B: Várias contas (via `accounts.json`)
Copie `examples/accounts.json.example` para `accounts.json` na raiz do projeto e configure a lista de contas:
```bash
cp examples/accounts.json.example accounts.json
```
```json
[
  {
    "profile": "ACCOUNT1",
    "tenancy_id": "ocid1.tenancy.oc1..aaaaaaa...",
    "image_id": "ocid1.image.oc1.sa-saopaulo-1...",
    "subnet_id": "ocid1.subnet.oc1.sa-saopaulo-1...",
    "ssh_key": "/root/.oci/chave_vps_arm.pub",
    "cpus": 2,
    "ram": 12,
    "boot_volume": 100,
    "display_name": "arm-instance-acc1"
  },
  {
    "profile": "ACCOUNT2",
    "tenancy_id": "ocid1.tenancy.oc1..bbbbbbb...",
    "image_id": "ocid1.image.oc1.sa-vinhedo-1...",
    "subnet_id": "ocid1.subnet.oc1.sa-vinhedo-1...",
    "ssh_key": "/root/.oci/chave_vps_arm.pub",
    "cpus": 2,
    "ram": 12,
    "boot_volume": 100,
    "display_name": "arm-instance-acc2"
  }
]
```

---

### 3. Notificação imediata via webhook (opcional)

Configure os webhooks de alerta no arquivo `.env`:

```bash
# Exemplo de API WhatsApp (Evolution API, Z-API, Baileys etc.):
NOTIFICATION_WEBHOOK_URL="https://api.example.com/message/sendText/my-instance"
NOTIFICATION_WEBHOOK_METHOD="POST"
NOTIFICATION_WEBHOOK_HEADERS="Content-Type: application/json; apikey: my-secret-token"
NOTIFICATION_WEBHOOK_BODY='{"number": "5511999999999", "text": "{message}"}'

# Exemplo de webhook do Discord:
# NOTIFICATION_WEBHOOK_URL="https://discord.com/api/webhooks/xxxx/yyyy"
```

---

## Uso

### Iniciar o script
No terminal, na pasta do projeto:
```bash
docker compose up -d --build
```

### Acompanhar o progresso
Para ver os logs em tempo real:
```bash
docker compose logs -f
```

Os logs saem como um objeto JSON por linha no stdout (`timestamp`, `level`, `service`, `app`, `env`, `message`). IDs de alta cardinalidade, como `instance_id`, são campos do evento, não dimensões de stream.

**Mensagens de log:**
* `Tentando criar instância`: tentativa de criação em tempo real (`cycle`, `profile`, `availability_domain`).
* `Sem capacidade no momento`: comportamento esperado quando não há capacidade no host. Nova tentativa no próximo ciclo.
* `Rate limit atingido`: limite de taxa detectado; o bot pausa com segurança até o próximo ciclo.
* `Instância criada com sucesso`: instância provisionada (`instance_id`) e alerta de webhook enviado.

### Encerrar o script
```bash
docker compose down
```

---

## Estrutura de arquivos

```plaintext
.
├── cmd/oracle-fisher/main.go          # Entrypoint do loop de tentativas
├── internal/config/                   # .env e accounts.json
├── internal/oci/                      # Clientes OCI, ADs, chave SSH pública
├── internal/notify/                   # Notificações via webhook
├── internal/logging/                  # Logger NDJSON no stdout
├── examples/.env.example              # Copiar para ./.env
├── examples/accounts.json.example     # Copiar para ./accounts.json
├── docker-compose.yml                 # Monta ./.env, ./accounts.json, ./oci_keys
├── Dockerfile                         # Binário estático multi-stage -> imagem ~20 MB
├── go.mod                             # Módulo oracle-fisher (oci-go-sdk v65)
├── oci_keys/                          # Credenciais montadas (/root/.oci)
│   ├── config
│   ├── oracle_api_key.pem
│   ├── chave_vps_arm
│   └── chave_vps_arm.pub
└── assets/screenshot.png
```

## Créditos
Baseado no trabalho original de futchas e maindust.
