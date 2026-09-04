#!/bin/bash

source .env

if [[ -z "${TENANCY_ID}" ]]; then
    echo "TENANCY_ID is unset or empty. Please change in .env file"
    exit 1
else
    echo "TENANCY_ID is set correctly"
fi

# To verify that the authentication with Oracle cloud works
echo "Checking Connection with this request: "
oci iam compartment list
if [ $? -ne 0 ]; then
    echo "Connection to Oracle cloud is not working. Check your setup and config again!"
    exit 1
fi

# ----------------------CUSTOMIZE & CONFIGURATION-----------------------------------------------------------------------

# Don't go too low or you run into 429 TooManyRequests
requestInterval="${requestInterval:-60}" # seconds

# VM params (defaults to Always Free tier: 4 OCPUs, 24GB RAM, 100GB Boot Volume)
cpus="${cpus:-4}"
ram="${ram:-24}"
bootVolume="${bootVolume:-100}"

profile="${OCI_PROFILE:-DEFAULT}"
displayName="${DISPLAY_NAME:-big-arm}"

# ----------------------NOTIFICATION FUNCTION---------------------------------------------------------------------------

send_notification() {
    local msg="$1"
    if [[ -z "${NOTIFICATION_WEBHOOK_URL}" ]]; then
        return 0
    fi

    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Enviando notificacao via webhook..."
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

    # Adiciona body customizado com suporte a {message} ou payload padrao compatível
    if [[ -n "${NOTIFICATION_WEBHOOK_BODY}" ]]; then
        local body="${NOTIFICATION_WEBHOOK_BODY//\{message\}/$msg}"
        curl_args+=("-d" "$body")
    elif [[ "$method" == "POST" || "$method" == "PUT" || "$method" == "PATCH" ]]; then
        curl_args+=("-d" "{\"message\":\"$msg\",\"content\":\"$msg\",\"text\":\"$msg\"}")
    fi

    local response
    response=$(curl "${curl_args[@]}" 2>&1)
    local curl_status=$?
    if [ $curl_status -eq 0 ]; then
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] Notificacao enviada com sucesso."
    else
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] Falha ao enviar notificacao (status curl: $curl_status). Resposta: $response"
    fi
}

# ----------------------ENDLESS LOOP TO REQUEST AN ARM INSTANCE---------------------------------------------------------

attempt=0

while true; do
    attempt=$((attempt + 1))
    timestamp="$(date '+%Y-%m-%d %H:%M:%S')"
    echo "[$timestamp] [Tentativa #$attempt] Solicitando instancia ARM ($cpus OCPUs, ${ram}GB RAM, ${bootVolume}GB Disco, Perfil: $profile)..."

    oci compute instance launch --no-retry  \
    --auth api_key \
    --profile "$profile" \
    --display-name "$displayName" \
    --compartment-id "$TENANCY_ID" \
    --image-id "$IMAGE_ID" \
    --subnet-id "$SUBNET_ID" \
    --availability-domain "$AVAILABILITY_DOMAIN" \
    --shape 'VM.Standard.A1.Flex' \
    --shape-config "{'ocpus':$cpus,'memoryInGBs':$ram}" \
    --boot-volume-size-in-gbs "$bootVolume" \
    --ssh-authorized-keys-file "$PATH_TO_PUBLIC_SSH_KEY"

    # Check if the command was successful
    if [ $? -eq 0 ]; then
        success_time="$(date '+%Y-%m-%d %H:%M:%S')"
        echo "[$success_time] Instancia criada com sucesso!"
        send_notification "🎉 Instancia ARM criada com sucesso na Oracle Cloud! Nome: $displayName ($cpus OCPUs, ${ram}GB RAM, ${bootVolume}GB Disco)"
        break
    else
        echo "[$timestamp] Falha na tentativa #$attempt (sem capacidade disponivel). Tentando novamente em $requestInterval segundos..."
    fi

    sleep "$requestInterval"
done
