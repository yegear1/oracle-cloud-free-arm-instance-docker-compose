# Oracle Cloud Free ARM Instance Creator (Dockerized)

This project automates the creation of Always Free ARM instances (up to 4 CPUs, 24GB RAM) on Oracle Cloud Infrastructure (OCI). Built with the **official Oracle OCI Python SDK**, it supports both **single-account** and **simultaneous multi-account** operation, maintaining persistent HTTP connections (Keep-Alive) to attempt instance creation in milliseconds across all Availability Domains (ADs).

Due to high demand, creating ARM instances often results in an *Out of host capacity* error. This bot runs within a lightweight Docker container, continuously attempting to create the instance every 60 seconds (configurable) across all availability zones until slots become available.

### Key Features
* **Pure Python with OCI SDK:** Uses the official `oci` Python SDK with persistent HTTP Keep-Alive sessions, reducing request latency to ~150ms (compared to ~2s per CLI spawn) to catch slots the second they open.
* **Multi-Account & Single-Account Support:** Run for one account via `.env` or multiple accounts simultaneously via `accounts.json`.
* **Dynamic Availability Domains:** Automatically lists and tries all ADs in your region (`oci iam availability-domain list`), maximizing success probabilities.
* **Instant Notifications:** Immediate alerts via WhatsApp (Evolution API, Z-API, Baileys, etc.), Discord, Slack, or any custom webhook.
* **Configurable Hardware:** Easily adjust OCPUs, RAM, and boot volume limits per account or globally.
* **Native JSON & Cross-Platform:** Handles Windows/Linux line endings (`\r\n` vs `\n`) seamlessly without bash scripting dependencies.
* **Organized Structure:** All keys and configuration files are isolated within the `oci_keys` directory.

---

## Prerequisites
* One or more active accounts on Oracle Cloud.
* **Docker** and **Docker Compose** installed on your machine.

---

## Setup Instructions

### 1. Prepare Credentials (`oci_keys` directory)
All credentials must be placed inside the `oci_keys` directory located in the project root.

#### Step A: Oracle API Key
1. Log in to the Oracle Cloud Console.
2. Navigate to **My Profile** -> **API Keys** -> **Add API Key**.
3. Select **Generate API Key Pair** and download the Private Key.
4. Save this file as `oracle_api_key.pem` inside the `oci_keys` folder (or `oracle_api_key_<profile>.pem` if using multiple accounts).
5. Click **Add**.
6. Copy the content displayed in the "Configuration File Preview" text box.

#### Step B: Config File
1. Create or edit `oci_keys/config` (no file extension).
2. Paste the OCI configuration snippet. Ensure the `key_file` points to the container path `/root/.oci/...`.

**Single-Account example (`[DEFAULT]`):**
```ini
[DEFAULT]
user=ocid1.user.oc1..aaaa...
fingerprint=xx:xx:xx:xx...
tenancy=ocid1.tenancy.oc1..aaaa...
region=sa-saopaulo-1
key_file=/root/.oci/oracle_api_key.pem
```

**Multi-Account example (`[ACCOUNT1]`, `[ACCOUNT2]`):**
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

#### Step C: SSH Keys (For VPS Access)
Generate an SSH key pair inside the `oci_keys` folder. This key will be used to access your instance after it is created.

```bash
ssh-keygen -t rsa -b 4096 -f ./oci_keys/chave_vps_arm
```
> **Note:** Do not add a passphrase if you want the process to be fully automated without prompts.

---

### 2. Configuration Options

You can configure the project in one of two modes:

#### Option A: Single Account (via `.env`)
Edit the `.env` file in the project root:
```bash
TENANCY_ID="ocid1.tenancy.oc1..aaaaaaa..."
IMAGE_ID="ocid1.image.oc1.sa-saopaulo-1..."
SUBNET_ID="ocid1.subnet.oc1.sa-saopaulo-1..."
PATH_TO_PUBLIC_SSH_KEY="/root/.oci/chave_vps_arm.pub"

# Hardware resources (Always Free Tier limits)
cpus=4
ram=24
bootVolume=100
requestInterval=60
```

#### Option B: Multiple Accounts (via `accounts.json`)
Copy `accounts.json.example` to `accounts.json` and fill with your accounts:
```json
[
  {
    "profile": "ACCOUNT1",
    "tenancy_id": "ocid1.tenancy.oc1..aaaaaaa...",
    "image_id": "ocid1.image.oc1.sa-saopaulo-1...",
    "subnet_id": "ocid1.subnet.oc1.sa-saopaulo-1...",
    "ssh_key": "/root/.oci/chave_vps_arm.pub",
    "cpus": 4,
    "ram": 24,
    "boot_volume": 100,
    "display_name": "arm-instance-acc1"
  },
  {
    "profile": "ACCOUNT2",
    "tenancy_id": "ocid1.tenancy.oc1..bbbbbbb...",
    "image_id": "ocid1.image.oc1.sa-vinhedo-1...",
    "subnet_id": "ocid1.subnet.oc1.sa-vinhedo-1...",
    "ssh_key": "/root/.oci/chave_vps_arm.pub",
    "cpus": 4,
    "ram": 24,
    "boot_volume": 100,
    "display_name": "arm-instance-acc2"
  }
]
```

---

### 3. Instant Notification via Webhook (Optional)

Configure alert webhooks in your `.env` file:

```bash
# WhatsApp API Example (Evolution API, Z-API, Baileys, etc.):
NOTIFICATION_WEBHOOK_URL="https://api.example.com/message/sendText/my-instance"
NOTIFICATION_WEBHOOK_METHOD="POST"
NOTIFICATION_WEBHOOK_HEADERS="Content-Type: application/json; apikey: my-secret-token"
NOTIFICATION_WEBHOOK_BODY='{"number": "5511999999999", "text": "{message}"}'

# Discord Webhook Example:
# NOTIFICATION_WEBHOOK_URL="https://discord.com/api/webhooks/xxxx/yyyy"
```

---

## Usage

### Starting the Script
Open your terminal in the project folder and run:
```bash
docker compose up -d --build
```

### Checking Progress
To view the logs in real time:
```bash
docker compose logs -f
```

**Log indicators:**
* `[*] Processando Perfil: [NOME]`: Current account being processed.
* `-> Tentando no AD: [AD_NAME]`: Dynamic attempt in specified Availability Domain.
* `[X] Falha no AD ... (Sem capacidade no momento)`: Expected behavior when region is full. It retries automatically.
* `[SUCESSO] Instância criada com sucesso`: Success! Instance provisioned and notification dispatched.

### Stopping the Script
```bash
docker compose down
```

---

## File Structure

```plaintext
.
├── docker-compose.yml               # Docker Compose orchestration
├── Dockerfile                       # Container image specification (Python 3.9-slim)
├── requirements.txt                 # Python dependencies (oci, requests)
├── main.py                          # Core Python engine with OCI SDK & Keep-Alive
├── accounts.json                    # Active multi-account configuration
├── accounts.json.example            # Example schema for multiple accounts
├── .env                             # Global variables and webhook settings
├── AGENTS.md                        # Context and guidelines for AI agents
├── entrypoint.sh                    # Optional legacy container startup script
├── oracle_cloud_instance_creator.sh # Optional legacy bash script
└── oci_keys/                        # Mounted volume for credentials
    ├── config                       # OCI CLI/SDK credentials (with profiles)
    ├── oracle_api_key.pem           # API Private Key
    ├── chave_vps_arm                # SSH Private Key
    └── chave_vps_arm.pub            # SSH Public Key
```

## Credits
Based on the original work by futchas and maindust.