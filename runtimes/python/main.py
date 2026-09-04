#!/usr/bin/env python3
"""
Oracle Cloud Free ARM Instance Creator (Pure Python with OCI SDK)
Automates Always Free ARM instance provisioning with Keep-Alive sessions,
dynamic Availability Domain discovery, multi-account support, and webhook alerts.
"""

import os
import sys
import time
import json
import logging
import requests

try:
    import oci
except ImportError:
    print("[ERRO] O pacote 'oci' não está instalado. Instale com: pip install -r requirements.txt")
    sys.exit(1)

# Configuração de logging
logging.basicConfig(
    level=logging.INFO,
    format="[%(asctime)s] %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S"
)
logger = logging.getLogger("oracle-fisher")


def load_dotenv(filepath: str = ".env"):
    """Carrega variáveis do arquivo .env para o os.environ sem bibliotecas externas."""
    if not os.path.isfile(filepath):
        return
    try:
        with open(filepath, "r", encoding="utf-8", errors="ignore") as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("#"):
                    continue
                if "=" in line:
                    key, val = line.split("=", 1)
                    key = key.strip()
                    val = val.strip().strip("'\"")
                    if key and key not in os.environ:
                        os.environ[key] = val
    except Exception as e:
        logger.warning(f"Não foi possível ler o arquivo {filepath}: {e}")


def send_notification(msg: str):
    """Envia notificação via Webhook (compatível com WhatsApp API, Discord, Slack, etc.)."""
    webhook_url = os.getenv("NOTIFICATION_WEBHOOK_URL", "").strip()
    if not webhook_url:
        return

    logger.info("[Webhook] Enviando notificação...")
    method = os.getenv("NOTIFICATION_WEBHOOK_METHOD", "POST").upper()
    headers_str = os.getenv("NOTIFICATION_WEBHOOK_HEADERS", "").strip()
    body_template = os.getenv("NOTIFICATION_WEBHOOK_BODY", "").strip()

    headers = {"Content-Type": "application/json"}
    if headers_str:
        for item in headers_str.split(";"):
            if ":" in item:
                k, v = item.split(":", 1)
                headers[k.strip()] = v.strip()

    try:
        if body_template:
            escaped_msg = msg.replace("\n", "\\n").replace('"', '\\"')
            body_str = body_template.replace("{message}", escaped_msg)
            resp = requests.request(
                method=method,
                url=webhook_url,
                headers=headers,
                data=body_str.encode("utf-8"),
                timeout=15
            )
        else:
            payload = {"message": msg, "content": msg, "text": msg}
            resp = requests.request(
                method=method,
                url=webhook_url,
                headers=headers,
                json=payload,
                timeout=15
            )

        logger.info(f"[Webhook] Notificação disparada com sucesso (HTTP {resp.status_code}).")
    except Exception as ex:
        logger.error(f"[Webhook] Falha ao enviar notificação: {ex}")


def get_ssh_key_content(key_path: str) -> str:
    """Lê a chave SSH pública a partir do caminho informado ou locais padrão."""
    candidates = [
        key_path,
        "/root/.oci/chave_vps_arm.pub",
        "/root/.oci/vps_ssh_key.pub"
    ]
    for path in candidates:
        if path and os.path.isfile(path):
            try:
                with open(path, "r", encoding="utf-8") as f:
                    content = f.read().strip()
                    if content:
                        return content
            except Exception as e:
                logger.warning(f"Erro ao ler arquivo de chave SSH em {path}: {e}")
    return ""


def load_accounts() -> list:
    """Carrega contas do accounts.json ou sintetiza conta única a partir do .env."""
    accounts = []
    if os.path.isfile("accounts.json"):
        try:
            with open("accounts.json", "r", encoding="utf-8") as f:
                data = json.load(f)
                if isinstance(data, list) and len(data) > 0:
                    accounts = data
        except Exception as e:
            logger.warning(f"Não foi possível processar accounts.json: {e}")

    if accounts:
        logger.info(f"--- Modo Multi-Contas Ativado ({len(accounts)} conta(s) em accounts.json) ---")
        return accounts

    logger.info("--- Modo Conta Única Ativado (.env) ---")
    tenancy_id = os.getenv("TENANCY_ID", "").strip()
    if not tenancy_id:
        logger.error("[ERRO CRÍTICO] TENANCY_ID não definido no .env e nenhuma conta configurada em accounts.json.")
        sys.exit(1)

    return [{
        "profile": os.getenv("OCI_PROFILE", "DEFAULT"),
        "tenancy_id": tenancy_id,
        "image_id": os.getenv("IMAGE_ID", "").strip(),
        "subnet_id": os.getenv("SUBNET_ID", "").strip(),
        "ssh_key": os.getenv("PATH_TO_PUBLIC_SSH_KEY", "/root/.oci/chave_vps_arm.pub").strip(),
        "cpus": float(os.getenv("cpus", 4)),
        "ram": float(os.getenv("ram", 24)),
        "boot_volume": int(os.getenv("bootVolume", 100)),
        "display_name": os.getenv("DISPLAY_NAME", "big-arm"),
        "availability_domain": os.getenv("AVAILABILITY_DOMAIN", "").strip()
    }]


def main():
    load_dotenv(".env")

    config_file_path = os.getenv("OCI_CONFIG_FILE", "/root/.oci/config")
    request_interval = int(os.getenv("requestInterval", 60))
    accounts = load_accounts()

    # Cache de clientes OCI com sessões HTTP Keep-Alive reutilizáveis
    clients = {}
    ad_cache = {}

    def get_oci_clients(profile_name: str):
        if profile_name not in clients:
            try:
                config = oci.config.from_file(file_location=config_file_path, profile_name=profile_name)
                identity_client = oci.identity.IdentityClient(config)
                compute_client = oci.core.ComputeClient(config)
                clients[profile_name] = (identity_client, compute_client, config)
            except Exception as e:
                logger.error(f"Falha ao carregar credenciais OCI para o perfil '{profile_name}' de {config_file_path}: {e}")
                return None, None, None
        return clients[profile_name]

    def get_ads(identity_client, tenancy_id: str, profile_name: str, fallback_ad: str = None) -> list:
        if profile_name in ad_cache:
            return ad_cache[profile_name]

        try:
            resp = identity_client.list_availability_domains(compartment_id=tenancy_id)
            ads = [ad.name for ad in resp.data]
            if ads:
                ad_cache[profile_name] = ads
                return ads
        except Exception as e:
            logger.warning(f"Não foi possível listar Zonas de Disponibilidade (ADs) para '{profile_name}': {e}")

        if fallback_ad:
            return [fallback_ad]
        return []

    success_accounts = set()
    cycle = 0

    while True:
        cycle += 1
        logger.info("=" * 80)
        logger.info(f"Iniciando ciclo de tentativas #{cycle}")
        logger.info("=" * 80)

        all_done = True

        for acc in accounts:
            profile = acc.get("profile") or os.getenv("OCI_PROFILE", "DEFAULT")
            if profile in success_accounts:
                continue

            all_done = False

            tenancy_id = acc.get("tenancy_id") or os.getenv("TENANCY_ID")
            image_id = acc.get("image_id") or os.getenv("IMAGE_ID")
            subnet_id = acc.get("subnet_id") or os.getenv("SUBNET_ID")
            ssh_key_path = acc.get("ssh_key") or os.getenv("PATH_TO_PUBLIC_SSH_KEY")
            cpus = float(acc.get("cpus") or os.getenv("cpus", 4))
            ram = float(acc.get("ram") or os.getenv("ram", 24))
            boot_volume = int(acc.get("boot_volume") or os.getenv("bootVolume", 100))
            display_name = acc.get("display_name") or f"{os.getenv('DISPLAY_NAME', 'big-arm')}-{profile}"
            fallback_ad = acc.get("availability_domain") or os.getenv("AVAILABILITY_DOMAIN")

            identity_client, compute_client, _ = get_oci_clients(profile)
            if not identity_client or not compute_client:
                logger.warning(f"Pulando conta '{profile}' devido a credenciais inválidas.")
                continue

            ads = get_ads(identity_client, tenancy_id, profile, fallback_ad)
            if not ads:
                logger.warning(f"Nenhum Availability Domain encontrado para a conta '{profile}'.")
                continue

            ssh_key_content = get_ssh_key_content(ssh_key_path)

            for ad in ads:
                logger.info(
                    f"[Ciclo #{cycle}] [Conta: {profile}] Tentando no AD: {ad} "
                    f"({cpus} OCPUs, {ram}GB RAM, {boot_volume}GB Disco)..."
                )

                launch_details = oci.core.models.LaunchInstanceDetails(
                    compartment_id=tenancy_id,
                    availability_domain=ad,
                    shape="VM.Standard.A1.Flex",
                    display_name=display_name,
                    shape_config=oci.core.models.LaunchInstanceShapeConfigDetails(
                        ocpus=cpus,
                        memory_in_gbs=ram
                    ),
                    source_details=oci.core.models.InstanceSourceViaImageDetails(
                        source_type="image",
                        image_id=image_id,
                        boot_volume_size_in_gbs=boot_volume
                    ),
                    create_vnic_details=oci.core.models.CreateVnicDetails(
                        subnet_id=subnet_id,
                        assign_public_ip=True
                    ),
                    metadata={"ssh_authorized_keys": ssh_key_content} if ssh_key_content else {}
                )

                try:
                    res = compute_client.launch_instance(launch_details)
                    instance_data = res.data
                    instance_id = getattr(instance_data, "id", "OK")
                    logger.info(
                        f"🎉 [SUCESSO] Instância criada com sucesso para a conta '{profile}' no AD '{ad}'! "
                        f"ID: {instance_id}"
                    )
                    success_accounts.add(profile)
                    send_notification(
                        f"🎉 Instância ARM criada com sucesso na Oracle Cloud!\n"
                        f"Conta: {profile}\n"
                        f"AD: {ad}\n"
                        f"Nome: {display_name}\n"
                        f"Hardware: {cpus} OCPUs, {ram}GB RAM, {boot_volume}GB Disco"
                    )
                    break  # Encerra tentativa de ADs para esta conta
                except oci.exceptions.ServiceError as se:
                    if "out of host capacity" in str(se.message).lower() or se.status == 500:
                        logger.info(f"[X] Sem capacidade no momento no AD '{ad}'.")
                    elif se.status == 429:
                        logger.warning(f"Rate Limit atingido (HTTP 429 Too Many Requests) no AD '{ad}'.")
                    else:
                        logger.warning(f"Erro na API OCI [{se.status} - {se.code}]: {se.message}")
                except Exception as ex:
                    logger.error(f"Erro inesperado ao tentar no AD '{ad}': {ex}")

        if all_done:
            logger.info("=" * 80)
            logger.info("Todas as instâncias solicitadas foram criadas com sucesso! Encerrando.")
            logger.info("=" * 80)
            send_notification("🏁 Todas as instâncias ARM solicitadas foram criadas com sucesso na Oracle Cloud!")
            sys.exit(0)

        logger.info(f"Aguardando {request_interval} segundos antes do próximo ciclo...")
        time.sleep(request_interval)


if __name__ == "__main__":
    main()
