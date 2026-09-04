#!/bin/bash

# Carrega variáveis globais do .env se existir
if [ -f .env ]; then
    source .env
fi

# ----------------------CONFIGURAÇÕES GLOBAIS E DE FALLBACK--------------------------------------------------------------

# Intervalo entre ciclos de tentativas (em segundos)
requestInterval="${requestInterval:-60}"

# Recursos de hardware padrão (Always Free: até 4 OCPUs, 24GB RAM, 100GB Boot Volume)
default_cpus="${cpus:-4}"
default_ram="${ram:-24}"
default_boot_volume="${bootVolume:-100}"
default_profile="${OCI_PROFILE:-DEFAULT}"
default_display_name="${DISPLAY_NAME:-big-arm}"

# ----------------------FUNÇÃO DE NOTIFICAÇÃO (WEBHOOK / WHATSAPP / DISCORD)---------------------------------------------

send_notification() {
    local msg="$1"
    if [[ -z "${NOTIFICATION_WEBHOOK_URL}" ]]; then
        return 0
    fi

    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [Webhook] Enviando notificacao..."
    local method="${NOTIFICATION_WEBHOOK_METHOD:-POST}"
    local curl_args=("-s" "-S" "-X" "$method" "$NOTIFICATION_WEBHOOK_URL")

    # Adiciona headers customizados separados por ponto e virgula (;)
    if [[ -n "${NOTIFICATION_WEBHOOK_HEADERS}" ]]; then
        IFS=';' read -ra HEADERS <<< "$NOTIFICATION_WEBHOOK_HEADERS"
        for h in "${HEADERS[@]}"; do
            local h_trimmed
            h_trimmed="$(echo "$h" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
            if [[ -n "$h_trimmed" ]]; then
                curl_args+=("-H" "$h_trimmed")
            fi
        done
    else
        curl_args+=("-H" "Content-Type: application/json")
    fi

    # Formata body customizado (com suporte à tag {message}) ou payload padrão
    if [[ -n "${NOTIFICATION_WEBHOOK_BODY}" ]]; then
        # Escapa quebras de linha para JSON válido se necessário
        local escaped_msg
        escaped_msg=$(echo "$msg" | sed ':a;N;$!ba;s/\n/\\n/g' | sed 's/"/\\"/g')
        local body="${NOTIFICATION_WEBHOOK_BODY//\{message\}/$escaped_msg}"
        curl_args+=("-d" "$body")
    elif [[ "$method" == "POST" || "$method" == "PUT" || "$method" == "PATCH" ]]; then
        local escaped_msg
        escaped_msg=$(echo "$msg" | sed ':a;N;$!ba;s/\n/\\n/g' | sed 's/"/\\"/g')
        curl_args+=("-d" "{\"message\":\"$escaped_msg\",\"content\":\"$escaped_msg\",\"text\":\"$escaped_msg\"}")
    fi

    local response
    response=$(curl "${curl_args[@]}" 2>&1)
    local curl_status=$?
    if [ $curl_status -eq 0 ]; then
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] [Webhook] Notificacao enviada com sucesso."
    else
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] [Webhook] Falha ao enviar notificacao (curl: $curl_status). Resposta: $response"
    fi
}

# ----------------------CARREGAMENTO DE CONTAS (MULTI-ACCOUNT OU .ENV)---------------------------------------------------

accounts_list=()

if [ -f accounts.json ] && jq -e '. | length > 0' accounts.json >/dev/null 2>&1; then
    echo "--- Modo Multi-Contas Detectado (accounts.json) ---"
    mapfile -t accounts_list < <(jq -c '.[]' accounts.json)
    echo "Total de contas carregadas: ${#accounts_list[@]}"
else
    echo "--- Modo Conta Única Detectado (.env) ---"
    if [[ -z "${TENANCY_ID}" ]]; then
        echo "[ERRO] TENANCY_ID nao esta definido no .env e nenhuma conta valida foi encontrada em accounts.json."
        exit 1
    fi
    # Monta objeto JSON sintético para a conta única
    single_account_json=$(jq -n \
        --arg profile "$default_profile" \
        --arg tenancy_id "$TENANCY_ID" \
        --arg image_id "$IMAGE_ID" \
        --arg subnet_id "$SUBNET_ID" \
        --arg ssh_key "${PATH_TO_PUBLIC_SSH_KEY:-/root/.oci/chave_vps_arm.pub}" \
        --arg display_name "$default_display_name" \
        --argjson cpus "$default_cpus" \
        --argjson ram "$default_ram" \
        --argjson boot_volume "$default_boot_volume" \
        '{profile: $profile, tenancy_id: $tenancy_id, image_id: $image_id, subnet_id: $subnet_id, ssh_key: $ssh_key, display_name: $display_name, cpus: $cpus, ram: $ram, boot_volume: $boot_volume}')
    accounts_list=("$single_account_json")
fi

# ----------------------LOOP PRINCIPAL DE CRIAÇÃO-----------------------------------------------------------------------

declare -A success_accounts
cycle=0

while true; do
    cycle=$((cycle + 1))
    echo "================================================================================"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Iniciando ciclo de tentativas #$cycle"
    echo "================================================================================"

    all_done=true

    for row in "${accounts_list[@]}"; do
        # Extrai dados da conta com fallbacks
        profile=$(echo "$row" | jq -r '.profile // empty')
        profile="${profile:-$default_profile}"

        # Se esta conta já obteve a máquina, pula
        if [[ "${success_accounts[$profile]}" == "1" ]]; then
            continue
        fi

        all_done=false

        tenancy_id=$(echo "$row" | jq -r '.tenancy_id // empty')
        tenancy_id="${tenancy_id:-$TENANCY_ID}"

        image_id=$(echo "$row" | jq -r '.image_id // empty')
        image_id="${image_id:-$IMAGE_ID}"

        subnet_id=$(echo "$row" | jq -r '.subnet_id // empty')
        subnet_id="${subnet_id:-$SUBNET_ID}"

        ssh_key=$(echo "$row" | jq -r '.ssh_key // empty')
        ssh_key="${ssh_key:-$PATH_TO_PUBLIC_SSH_KEY}"

        acc_display_name=$(echo "$row" | jq -r '.display_name // empty')
        acc_display_name="${acc_display_name:-$default_display_name-$profile}"

        acc_cpus=$(echo "$row" | jq -r '.cpus // empty')
        acc_cpus="${acc_cpus:-$default_cpus}"

        acc_ram=$(echo "$row" | jq -r '.ram // empty')
        acc_ram="${acc_ram:-$default_ram}"

        acc_boot_volume=$(echo "$row" | jq -r '.boot_volume // empty')
        acc_boot_volume="${acc_boot_volume:-$default_boot_volume}"

        echo "[*] Processando Perfil: $profile (Tenancy: $tenancy_id)"

        # 1. Busca dinâmica de Zonas de Disponibilidade (Availability Domains) para a região da conta
        ads_json=$(oci iam availability-domain list --compartment-id "$tenancy_id" --profile "$profile" 2>/dev/null)
        ads=$(echo "$ads_json" | jq -r '.data[].name' 2>/dev/null)

        # Fallback caso a listagem automática falhe ou a variável fixa AVAILABILITY_DOMAIN exista
        if [ -z "$ads" ] && [ -n "$AVAILABILITY_DOMAIN" ]; then
            ads="$AVAILABILITY_DOMAIN"
        fi

        if [ -z "$ads" ]; then
            echo "    [!] Erro ao listar Availability Domains para o perfil $profile. Verifique credenciais em oci_keys/config."
            continue
        fi

        # 2. Itera sobre cada Zona de Disponibilidade descoberta
        for ad in $ads; do
            timestamp="$(date '+%Y-%m-%d %H:%M:%S')"
            echo "    [$timestamp] -> Tentando no AD: $ad ($acc_cpus OCPUs, ${acc_ram}GB RAM, ${acc_boot_volume}GB Disco)..."

            oci compute instance launch --no-retry \
                --auth api_key \
                --profile "$profile" \
                --display-name "$acc_display_name" \
                --compartment-id "$tenancy_id" \
                --image-id "$image_id" \
                --subnet-id "$subnet_id" \
                --availability-domain "$ad" \
                --shape 'VM.Standard.A1.Flex' \
                --shape-config "{\"ocpus\":$acc_cpus,\"memoryInGBs\":$acc_ram}" \
                --boot-volume-size-in-gbs "$acc_boot_volume" \
                --ssh-authorized-keys-file "$ssh_key"

            if [ $? -eq 0 ]; then
                success_time="$(date '+%Y-%m-%d %H:%M:%S')"
                echo "    [$success_time] [SUCESSO] Instância criada com sucesso para o perfil '$profile' no AD '$ad'!"
                success_accounts["$profile"]="1"
                send_notification "🎉 Instância ARM criada com sucesso na Oracle Cloud!\nConta: $profile\nAD: $ad\nNome: $acc_display_name\nHardware: $acc_cpus OCPUs, ${acc_ram}GB RAM, ${acc_boot_volume}GB Disco"
                break # Encerra os ADs desta conta, pois a instância já foi provisionada
            else
                echo "    [$(date '+%Y-%m-%d %H:%M:%S')] [X] Falha no AD $ad (Sem capacidade no momento)."
            fi
        done
    done

    # Se todas as contas configuradas obtiveram sucesso
    if [ "$all_done" = true ]; then
        echo "================================================================================"
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] Todas as instâncias solicitadas foram criadas com sucesso! Encerrando."
        echo "================================================================================"
        send_notification "🏁 Todas as instâncias ARM solicitadas foram criadas com sucesso na Oracle Cloud!"
        exit 0
    fi

    echo "Aguardando $requestInterval segundos antes do proximo ciclo de tentativas..."
    sleep "$requestInterval"
done
