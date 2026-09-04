# Oracle Cloud Free ARM Instance Creator (Dockerized)

This project automates the creation of Always Free ARM instances (up to 4 CPUs, 24GB RAM) on Oracle Cloud Infrastructure (OCI). Built in **Go (Golang)** with the official **Oracle OCI Go SDK**, it achieves maximum efficiency: a compiled static binary running in an ultra-lightweight ~20MB container using only **~6MB of RAM**, with persistent HTTP Keep-Alive connections and ~50ms request latency.

Due to high demand, creating ARM instances often results in an *Out of host capacity* error. This bot runs continuously in the background, attempting to create the instance across all available Availability Domains (ADs) until slots become available.

### Key Features
* **High-Performance Go Engine:** Built with the official `oci-go-sdk`, reducing memory usage to ~6MB and request latency to ~50ms with HTTP Keep-Alive.
* **Ultra-Lightweight Multi-Stage Docker Build:** Produces a minimal ~20MB Docker image based on `alpine:3.20`.
* **Multi-Account & Single-Account Support:** Run for one account via `.env` or multiple accounts simultaneously via `accounts.json`.
* **Dynamic Availability Domains:** Automatically discovers and cycles through all ADs in your region (`oci iam availability-domain list`), maximizing creation chances.
* **Instant Notifications:** Immediate alerts via WhatsApp (Evolution API, Z-API, Baileys, etc.), Discord, Slack, or custom webhooks upon successful provisioning.
* **Configurable Hardware:** Easily adjust OCPUs, RAM, and boot volume limits per account or globally.
* **Cross-Platform & Zero Dependencies:** Compiles into a single static binary with no external runtime or script dependencies.
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

# Optional fallback (ADs are discovered automatically via OCI API)
# AVAILABILITY_DOMAIN="Uocm:SA-SAOPAULO-1-AD-1"

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
* `[Ciclo #X] [Conta: PROFILE] Tentando no AD: AD_NAME`: Real-time creation attempt in specified Availability Domain.
* `[X] Sem capacidade no momento no AD 'AD_NAME'`: Expected behavior when host capacity is full. Retries automatically on next cycle.
* `[!] Rate Limit atingido (HTTP 429 Too Many Requests)`: Rate limit detected; bot pauses safely until next cycle.
* `🎉 [SUCESSO] Instância criada com sucesso para a conta 'PROFILE' no AD 'AD_NAME'! ID: ocid1...`: Instance provisioned and webhook alert dispatched!

### Stopping the Script
```bash
docker compose down
```

---

### Alternative Runtimes (Python & Bash)
The root container defaults to the compiled, high-performance Go engine. Alternative runtimes are neatly organized and isolated in the `runtimes/` directory:
* **Python (`runtimes/python/`):** Built with the official `oci` Python SDK. Run with `docker build -t oracle-fisher-py runtimes/python/` or locally via `cd runtimes/python && pip install -r requirements.txt && python main.py`.
* **Bash (`runtimes/bash/`):** Legacy implementation with `oci-cli`. Run with `docker build -t oracle-fisher-bash runtimes/bash/`.

---

## File Structure

```plaintext
.
├── docker-compose.yml       # Docker Compose orchestration
├── Dockerfile               # Multi-stage production Dockerfile (Go static binary -> ~20MB image)
├── go.mod                   # Go module definition (oci-go-sdk v65)
├── main.go                  # Core Go engine with OCI SDK & Keep-Alive
├── accounts.json            # Active multi-account configuration
├── accounts.json.example    # Example schema for multiple accounts
├── .env                     # Global variables and webhook settings
├── AGENTS.md                # Context and guidelines for AI agents
├── LICENSE                  # License file
├── oci_keys/                # Mounted volume for credentials
│   ├── config               # OCI CLI/SDK credentials (with profiles)
│   ├── oracle_api_key.pem   # API Private Key
│   ├── chave_vps_arm        # SSH Private Key
│   └── chave_vps_arm.pub    # SSH Public Key
├── assets/                  # Media & screenshots
│   └── screenshot.png
└── runtimes/                # Alternative runtime implementations
    ├── python/              # Python implementation (oci SDK)
    │   ├── Dockerfile
    │   ├── main.py
    │   └── requirements.txt
    └── bash/                # Legacy Bash implementation (oci-cli)
        ├── Dockerfile
        ├── entrypoint.sh
        └── oracle_cloud_instance_creator.sh
```

## Credits
Based on the original work by futchas and maindust.