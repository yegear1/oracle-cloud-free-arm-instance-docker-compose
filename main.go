package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/identity"
)

// AccountConfig representa os parâmetros de uma conta OCI.
type AccountConfig struct {
	Profile            string  `json:"profile"`
	TenancyID          string  `json:"tenancy_id"`
	ImageID            string  `json:"image_id"`
	SubnetID           string  `json:"subnet_id"`
	SSHKey             string  `json:"ssh_key"`
	CPUs               float32 `json:"cpus"`
	RAM                float32 `json:"ram"`
	BootVolume         int64   `json:"boot_volume"`
	DisplayName        string  `json:"display_name"`
	AvailabilityDomain string  `json:"availability_domain"`
}

// ClientSet agrupa os clientes OCI com conexão HTTP Keep-Alive persistente.
type ClientSet struct {
	Compute  core.ComputeClient
	Identity identity.IdentityClient
}

var (
	clientCache   = make(map[string]*ClientSet)
	clientCacheMu sync.Mutex
	adCache       = make(map[string][]string)
	adCacheMu     sync.Mutex
)

// loadEnv lê o arquivo .env e carrega as variáveis de ambiente sem dependências externas.
func loadEnv(path string) {
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, "\"'")
			if _, exists := os.LookupEnv(key); !exists && key != "" {
				os.Setenv(key, val)
			}
		}
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float32) float32 {
	val := getEnv(key, "")
	if val != "" {
		if f, err := strconv.ParseFloat(val, 32); err == nil {
			return float32(f)
		}
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int64) int64 {
	val := getEnv(key, "")
	if val != "" {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			return i
		}
	}
	return defaultVal
}

// sendNotification dispara notificações via webhook HTTP (WhatsApp API, Discord, Slack, etc.).
func sendNotification(msg string) {
	webhookURL := getEnv("NOTIFICATION_WEBHOOK_URL", "")
	if webhookURL == "" {
		return
	}

	log.Println("[Webhook] Enviando notificação...")
	method := strings.ToUpper(getEnv("NOTIFICATION_WEBHOOK_METHOD", "POST"))
	headersStr := getEnv("NOTIFICATION_WEBHOOK_HEADERS", "")
	bodyTemplate := getEnv("NOTIFICATION_WEBHOOK_BODY", "")

	var bodyBytes []byte
	if bodyTemplate != "" {
		escapedMsg := strings.ReplaceAll(msg, "\n", "\\n")
		escapedMsg = strings.ReplaceAll(escapedMsg, "\"", "\\\"")
		bodyStr := strings.ReplaceAll(bodyTemplate, "{message}", escapedMsg)
		bodyBytes = []byte(bodyStr)
	} else {
		payload := map[string]string{
			"message": msg,
			"content": msg,
			"text":    msg,
		}
		bodyBytes, _ = json.Marshal(payload)
	}

	req, err := http.NewRequest(method, webhookURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		log.Printf("[Webhook] Erro ao instanciar requisição: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	if headersStr != "" {
		for _, h := range strings.Split(headersStr, ";") {
			parts := strings.SplitN(h, ":", 2)
			if len(parts) == 2 {
				req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
			}
		}
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[Webhook] Falha ao enviar notificação: %v", err)
		return
	}
	defer resp.Body.Close()

	log.Printf("[Webhook] Notificação disparada com sucesso (HTTP %d).", resp.StatusCode)
}

// getSSHKeyContent localiza e lê a chave SSH pública.
func getSSHKeyContent(path string) string {
	candidates := []string{
		path,
		"/root/.oci/chave_vps_arm.pub",
		"/root/.oci/vps_ssh_key.pub",
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if data, err := os.ReadFile(candidate); err == nil {
			content := strings.TrimSpace(string(data))
			if content != "" {
				return content
			}
		}
	}
	return ""
}

// loadAccounts carrega contas a partir de accounts.json ou do .env como fallback.
func loadAccounts() []AccountConfig {
	if info, err := os.Stat("accounts.json"); err == nil && !info.IsDir() {
		if data, err := os.ReadFile("accounts.json"); err == nil {
			var accounts []AccountConfig
			if err := json.Unmarshal(data, &accounts); err == nil && len(accounts) > 0 {
				log.Printf("--- Modo Multi-Contas Ativado (%d conta(s) em accounts.json) ---", len(accounts))
				return accounts
			}
		}
	}

	log.Println("--- Modo Conta Única Ativado (.env) ---")
	tenancyID := getEnv("TENANCY_ID", "")
	if tenancyID == "" {
		log.Fatal("[ERRO CRÍTICO] TENANCY_ID não definido no .env e nenhuma conta configurada em accounts.json.")
	}

	return []AccountConfig{
		{
			Profile:            getEnv("OCI_PROFILE", "DEFAULT"),
			TenancyID:          tenancyID,
			ImageID:            getEnv("IMAGE_ID", ""),
			SubnetID:           getEnv("SUBNET_ID", ""),
			SSHKey:             getEnv("PATH_TO_PUBLIC_SSH_KEY", "/root/.oci/chave_vps_arm.pub"),
			CPUs:               getEnvFloat("cpus", 4),
			RAM:                getEnvFloat("ram", 24),
			BootVolume:         getEnvInt("bootVolume", 100),
			DisplayName:        getEnv("DISPLAY_NAME", "big-arm"),
			AvailabilityDomain: getEnv("AVAILABILITY_DOMAIN", ""),
		},
	}
}

// getClients cria ou reutiliza instâncias de clientes OCI com conexão Keep-Alive persistente.
func getClients(configFilePath, profile string) (*ClientSet, error) {
	clientCacheMu.Lock()
	defer clientCacheMu.Unlock()

	if cs, ok := clientCache[profile]; ok {
		return cs, nil
	}

	configProvider, err := common.ConfigurationProviderFromFileWithProfile(configFilePath, profile, "")
	if err != nil {
		return nil, fmt.Errorf("erro no arquivo de configuração OCI (%s): %w", configFilePath, err)
	}

	identityClient, err := identity.NewIdentityClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("erro ao inicializar IdentityClient: %w", err)
	}

	computeClient, err := core.NewComputeClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("erro ao inicializar ComputeClient: %w", err)
	}

	cs := &ClientSet{
		Compute:  computeClient,
		Identity: identityClient,
	}
	clientCache[profile] = cs
	return cs, nil
}

// getADs obtém dinamicamente os Availability Domains da região da conta.
func getADs(ctx context.Context, cs *ClientSet, tenancyID, profile, fallbackAD string) []string {
	adCacheMu.Lock()
	defer adCacheMu.Unlock()

	if ads, ok := adCache[profile]; ok && len(ads) > 0 {
		return ads
	}

	req := identity.ListAvailabilityDomainsRequest{
		CompartmentId: common.String(tenancyID),
	}

	resp, err := cs.Identity.ListAvailabilityDomains(ctx, req)
	if err == nil && len(resp.Items) > 0 {
		var ads []string
		for _, item := range resp.Items {
			if item.Name != nil {
				ads = append(ads, *item.Name)
			}
		}
		if len(ads) > 0 {
			adCache[profile] = ads
			return ads
		}
	} else if err != nil {
		log.Printf("Aviso: não foi possível listar ADs dinamicamente para '%s': %v", profile, err)
	}

	if fallbackAD != "" {
		return []string{fallbackAD}
	}
	return nil
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime)
	loadEnv(".env")

	configFilePath := getEnv("OCI_CONFIG_FILE", "/root/.oci/config")
	requestInterval := getEnvInt("requestInterval", 60)
	accounts := loadAccounts()

	successAccounts := make(map[string]bool)
	cycle := 0
	ctx := context.Background()

	for {
		cycle++
		log.Println("================================================================================")
		log.Printf("Iniciando ciclo de tentativas #%d", cycle)
		log.Println("================================================================================")

		allDone := true

		for _, acc := range accounts {
			profile := acc.Profile
			if profile == "" {
				profile = getEnv("OCI_PROFILE", "DEFAULT")
			}

			if successAccounts[profile] {
				continue
			}

			allDone = false

			tenancyID := acc.TenancyID
			if tenancyID == "" {
				tenancyID = getEnv("TENANCY_ID", "")
			}
			imageID := acc.ImageID
			if imageID == "" {
				imageID = getEnv("IMAGE_ID", "")
			}
			subnetID := acc.SubnetID
			if subnetID == "" {
				subnetID = getEnv("SUBNET_ID", "")
			}
			sshKeyPath := acc.SSHKey
			if sshKeyPath == "" {
				sshKeyPath = getEnv("PATH_TO_PUBLIC_SSH_KEY", "/root/.oci/chave_vps_arm.pub")
			}
			cpus := acc.CPUs
			if cpus == 0 {
				cpus = getEnvFloat("cpus", 4)
			}
			ram := acc.RAM
			if ram == 0 {
				ram = getEnvFloat("ram", 24)
			}
			bootVolume := acc.BootVolume
			if bootVolume == 0 {
				bootVolume = getEnvInt("bootVolume", 100)
			}
			displayName := acc.DisplayName
			if displayName == "" {
				displayName = fmt.Sprintf("%s-%s", getEnv("DISPLAY_NAME", "big-arm"), profile)
			}
			fallbackAD := acc.AvailabilityDomain
			if fallbackAD == "" {
				fallbackAD = getEnv("AVAILABILITY_DOMAIN", "")
			}

			cs, err := getClients(configFilePath, profile)
			if err != nil {
				log.Printf("[!] Pulando conta '%s' devido a erro nas credenciais: %v", profile, err)
				continue
			}

			ads := getADs(ctx, cs, tenancyID, profile, fallbackAD)
			if len(ads) == 0 {
				log.Printf("[!] Nenhum Availability Domain encontrado para a conta '%s'.", profile)
				continue
			}

			sshKeyContent := getSSHKeyContent(sshKeyPath)
			metadataMap := make(map[string]string)
			if sshKeyContent != "" {
				metadataMap["ssh_authorized_keys"] = sshKeyContent
			}

			for _, ad := range ads {
				log.Printf(
					"[Ciclo #%d] [Conta: %s] Tentando no AD: %s (%.1f OCPUs, %.1fGB RAM, %dGB Disco)...",
					cycle, profile, ad, cpus, ram, bootVolume,
				)

				launchReq := core.LaunchInstanceRequest{
					LaunchInstanceDetails: core.LaunchInstanceDetails{
						CompartmentId:      common.String(tenancyID),
						AvailabilityDomain: common.String(ad),
						Shape:              common.String("VM.Standard.A1.Flex"),
						DisplayName:        common.String(displayName),
						ShapeConfig: &core.LaunchInstanceShapeConfigDetails{
							Ocpus:       common.Float32(cpus),
							MemoryInGBs: common.Float32(ram),
						},
						SourceDetails: core.InstanceSourceViaImageDetails{
							ImageId:             common.String(imageID),
							BootVolumeSizeInGBs: common.Int64(bootVolume),
						},
						CreateVnicDetails: &core.CreateVnicDetails{
							SubnetId:       common.String(subnetID),
							AssignPublicIp: common.Bool(true),
						},
						Metadata: metadataMap,
					},
				}

				resp, err := cs.Compute.LaunchInstance(ctx, launchReq)
				if err == nil {
					instanceID := "OK"
					if resp.Instance.Id != nil {
						instanceID = *resp.Instance.Id
					}
					log.Printf("🎉 [SUCESSO] Instância criada com sucesso para a conta '%s' no AD '%s'! ID: %s", profile, ad, instanceID)
					successAccounts[profile] = true
					sendNotification(fmt.Sprintf(
						"🎉 Instância ARM criada com sucesso na Oracle Cloud!\nConta: %s\nAD: %s\nNome: %s\nHardware: %.0f OCPUs, %.0fGB RAM, %dGB Disco",
						profile, ad, displayName, cpus, ram, bootVolume,
					))
					break
				}

				// Tratamento refinado de exceções com OCI ServiceError
				if servErr, ok := common.IsServiceError(err); ok {
					statusCode := servErr.GetHTTPStatusCode()
					lowerMsg := strings.ToLower(servErr.GetMessage())
					if strings.Contains(lowerMsg, "out of host capacity") || statusCode == 500 {
						log.Printf("[X] Sem capacidade no momento no AD '%s'.", ad)
					} else if statusCode == 429 {
						log.Printf("[!] Rate Limit atingido (HTTP 429 Too Many Requests) no AD '%s'.", ad)
					} else {
						log.Printf("[!] Erro na API OCI [%d - %s]: %s", statusCode, servErr.GetCode(), servErr.GetMessage())
					}
				} else {
					log.Printf("Erro inesperado no AD '%s': %v", ad, err)
				}
			}
		}

		if allDone {
			log.Println("================================================================================")
			log.Println("Todas as instâncias solicitadas foram criadas com sucesso! Encerrando.")
			log.Println("================================================================================")
			sendNotification("🏁 Todas as instâncias ARM solicitadas foram criadas com sucesso na Oracle Cloud!")
			os.Exit(0)
		}

		log.Printf("Aguardando %d segundos antes do próximo ciclo...", requestInterval)
		time.Sleep(time.Duration(requestInterval) * time.Second)
	}
}
