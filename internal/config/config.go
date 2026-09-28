package config

import (
	"encoding/json"
	"log/slog"
	"os"
	"strconv"
	"strings"
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

// LoadEnv lê o arquivo .env e carrega as variáveis de ambiente sem dependências externas.
func LoadEnv(path string) {
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

// Get retorna a variável de ambiente ou defaultVal quando ausente ou vazia.
func Get(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return defaultVal
}

// GetFloat interpreta a variável de ambiente como float32.
func GetFloat(key string, defaultVal float32) float32 {
	val := Get(key, "")
	if val != "" {
		if f, err := strconv.ParseFloat(val, 32); err == nil {
			return float32(f)
		}
	}
	return defaultVal
}

// GetInt interpreta a variável de ambiente como int64.
func GetInt(key string, defaultVal int64) int64 {
	val := Get(key, "")
	if val != "" {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			return i
		}
	}
	return defaultVal
}

// LoadAccounts carrega contas a partir de accounts.json ou do .env como fallback.
func LoadAccounts() []AccountConfig {
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
	tenancyID := Get("TENANCY_ID", "")
	if tenancyID == "" {
		slog.Error("TENANCY_ID não definido no .env e nenhuma conta configurada em accounts.json")
		os.Exit(1)
	}

	return []AccountConfig{
		{
			Profile:            Get("OCI_PROFILE", "DEFAULT"),
			TenancyID:          tenancyID,
			ImageID:            Get("IMAGE_ID", ""),
			SubnetID:           Get("SUBNET_ID", ""),
			SSHKey:             Get("PATH_TO_PUBLIC_SSH_KEY", "/root/.oci/chave_vps_arm.pub"),
			CPUs:               GetFloat("cpus", 2),
			RAM:                GetFloat("ram", 12),
			BootVolume:         GetInt("bootVolume", 100),
			DisplayName:        Get("DISPLAY_NAME", "big-arm"),
			AvailabilityDomain: Get("AVAILABILITY_DOMAIN", ""),
		},
	}
}
