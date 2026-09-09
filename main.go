package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	serviceName            = "oracle-fisher"
	appName                = "oracle-fisher"
	launchNetworkAttempts  = 3
	launchRetryBackoffUnit = 2 * time.Second
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

func unescapeQuotes(s string) string {
	return strings.ReplaceAll(s, `\"`, `"`)
}

func initLogger() {
	env := getEnv("APP_ENV", getEnv("ENV", "production"))
	if env != "development" {
		env = "production"
	}

	level := slog.LevelInfo
	switch strings.ToLower(getEnv("LOG_LEVEL", "info")) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) > 0 {
				return a
			}
			switch a.Key {
			case slog.MessageKey:
				a.Key = "message"
			case slog.LevelKey:
				a.Key = "level"
				if lvl, ok := a.Value.Any().(slog.Level); ok {
					name := "info"
					switch {
					case lvl >= slog.LevelError:
						name = "error"
					case lvl >= slog.LevelWarn:
						name = "warn"
					case lvl >= slog.LevelInfo:
						name = "info"
					default:
						name = "debug"
					}
					a.Value = slog.StringValue(name)
				}
			case slog.TimeKey:
				a.Key = "timestamp"
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.UTC().Format("2006-01-02T15:04:05.000Z"))
				}
			}
			return a
		},
	}).WithAttrs([]slog.Attr{
		slog.String("service", serviceName),
		slog.String("app", appName),
		slog.String("env", env),
	})
	slog.SetDefault(slog.New(handler))
}

func slogError(msg string, err error, args ...any) {
	if err != nil {
		args = append(args, "error", err.Error(), "stack_trace", strings.TrimSpace(string(debug.Stack())))
	}
	slog.Error(msg, args...)
}

// isTransientNetworkError identifica falha de transporte (dial/timeout) antes de um ServiceError da OCI.
func isTransientNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := common.IsServiceError(err); ok {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "timeout exceeded") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "tls handshake timeout")
}

// sendNotification dispara notificações via webhook HTTP (WhatsApp API, Discord, Slack, etc.).
func sendNotification(msg string) {
	webhookURL := getEnv("NOTIFICATION_WEBHOOK_URL", "")
	if webhookURL == "" {
		return
	}

	slog.Info("Enviando notificação via webhook")
	method := strings.ToUpper(getEnv("NOTIFICATION_WEBHOOK_METHOD", "POST"))
	headersStr := unescapeQuotes(getEnv("NOTIFICATION_WEBHOOK_HEADERS", ""))
	bodyTemplate := unescapeQuotes(getEnv("NOTIFICATION_WEBHOOK_BODY", ""))

	var bodyBytes []byte
	if bodyTemplate != "" {
		escapedMsg := strings.ReplaceAll(msg, "\n", "\\n")
		escapedMsg = strings.ReplaceAll(escapedMsg, "\"", "\\\"")
		bodyStr := strings.ReplaceAll(bodyTemplate, "{{message}}", escapedMsg)
		bodyStr = strings.ReplaceAll(bodyStr, "{message}", escapedMsg)
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
		slogError("Erro ao instanciar requisição de webhook", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	if headersStr != "" {
		var headerMap map[string]string
		if err := json.Unmarshal([]byte(headersStr), &headerMap); err == nil {
			for k, v := range headerMap {
				req.Header.Set(k, v)
			}
		} else {
			for _, h := range strings.Split(headersStr, ";") {
				parts := strings.SplitN(h, ":", 2)
				if len(parts) == 2 {
					req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
				}
			}
		}
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		slogError("Falha ao enviar notificação via webhook", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		slog.Info("Notificação disparada com sucesso", "status_code", resp.StatusCode)
	} else {
		bodySnippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		slog.Warn("Webhook retornou status HTTP de erro",
			"status_code", resp.StatusCode,
			"response_snippet", strings.TrimSpace(string(bodySnippet)),
		)
	}
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
				slog.Info("Modo multi-contas ativado", "accounts_count", len(accounts))
				return accounts
			}
		}
	}

	slog.Info("Modo conta única ativado")
	tenancyID := getEnv("TENANCY_ID", "")
	if tenancyID == "" {
		slog.Error("TENANCY_ID não definido no .env e nenhuma conta configurada em accounts.json")
		os.Exit(1)
	}

	return []AccountConfig{
		{
			Profile:            getEnv("OCI_PROFILE", "DEFAULT"),
			TenancyID:          tenancyID,
			ImageID:            getEnv("IMAGE_ID", ""),
			SubnetID:           getEnv("SUBNET_ID", ""),
			SSHKey:             getEnv("PATH_TO_PUBLIC_SSH_KEY", "/root/.oci/chave_vps_arm.pub"),
			CPUs:               getEnvFloat("cpus", 2),
			RAM:                getEnvFloat("ram", 12),
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
		slog.Warn("Não foi possível listar Availability Domains dinamicamente",
			"profile", profile,
			"error", err.Error(),
		)
	}

	if fallbackAD != "" {
		return []string{fallbackAD}
	}
	return nil
}

func main() {
	loadEnv(".env")
	initLogger()

	configFilePath := getEnv("OCI_CONFIG_FILE", "/root/.oci/config")
	requestInterval := getEnvInt("requestInterval", 60)
	accounts := loadAccounts()

	successAccounts := make(map[string]bool)
	cycle := 0
	ctx := context.Background()

	for {
		cycle++
		slog.Debug("Iniciando ciclo de tentativas", "cycle", cycle)

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
				cpus = getEnvFloat("cpus", 2)
			}
			ram := acc.RAM
			if ram == 0 {
				ram = getEnvFloat("ram", 12)
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
				slogError("Pulando conta devido a erro nas credenciais", err, "profile", profile)
				continue
			}

			ads := getADs(ctx, cs, tenancyID, profile, fallbackAD)
			if len(ads) == 0 {
				slog.Warn("Nenhum Availability Domain encontrado", "profile", profile)
				continue
			}

			sshKeyContent := getSSHKeyContent(sshKeyPath)
			metadataMap := make(map[string]string)
			if sshKeyContent != "" {
				metadataMap["ssh_authorized_keys"] = sshKeyContent
			}

			for _, ad := range ads {
				slog.Info("Tentando criar instância",
					"cycle", cycle,
					"profile", profile,
					"availability_domain", ad,
					"cpus", cpus,
					"ram_gb", ram,
					"boot_volume_gb", bootVolume,
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

				var resp core.LaunchInstanceResponse
				var err error
				for attempt := 1; attempt <= launchNetworkAttempts; attempt++ {
					resp, err = cs.Compute.LaunchInstance(ctx, launchReq)
					if err == nil {
						break
					}
					if attempt < launchNetworkAttempts && isTransientNetworkError(err) {
						slog.Warn("Timeout de rede ao criar instância; nova tentativa no mesmo ciclo",
							"profile", profile,
							"availability_domain", ad,
							"attempt", attempt,
							"max_attempts", launchNetworkAttempts,
							"error", err.Error(),
						)
						time.Sleep(time.Duration(attempt) * launchRetryBackoffUnit)
						continue
					}
					break
				}
				if err == nil {
					instanceID := "OK"
					if resp.Instance.Id != nil {
						instanceID = *resp.Instance.Id
					}
					slog.Info("Instância criada com sucesso",
						"event", "instance_created",
						"outcome", "success",
						"profile", profile,
						"availability_domain", ad,
						"display_name", displayName,
						"instance_id", instanceID,
						"cpus", cpus,
						"ram_gb", ram,
						"boot_volume_gb", bootVolume,
					)
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
						slog.Info("Sem capacidade no momento",
							"profile", profile,
							"availability_domain", ad,
							"status_code", statusCode,
						)
					} else if statusCode == 429 {
						slog.Warn("Rate limit atingido",
							"profile", profile,
							"availability_domain", ad,
							"status_code", statusCode,
						)
					} else {
						slog.Error("Erro na API OCI",
							"profile", profile,
							"availability_domain", ad,
							"status_code", statusCode,
							"oci_code", servErr.GetCode(),
							"error", servErr.GetMessage(),
						)
					}
				} else {
					slogError("Erro inesperado ao criar instância", err,
						"profile", profile,
						"availability_domain", ad,
					)
				}
			}
		}

		if allDone {
			slog.Info("Todas as instâncias solicitadas foram criadas com sucesso",
				"event", "all_instances_created",
				"outcome", "success",
				"accounts_count", len(accounts),
			)
			sendNotification("🏁 Todas as instâncias ARM solicitadas foram criadas com sucesso na Oracle Cloud!")
			os.Exit(0)
		}

		slog.Debug("Aguardando próximo ciclo", "interval_seconds", requestInterval)
		time.Sleep(time.Duration(requestInterval) * time.Second)
	}
}
