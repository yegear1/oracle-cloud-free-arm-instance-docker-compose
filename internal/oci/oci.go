package oci

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/identity"
)

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

// TransientNetworkError identifica falha de transporte (dial/timeout) antes de um ServiceError da OCI.
func TransientNetworkError(err error) bool {
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

// PublicSSHKey localiza e lê a chave SSH pública.
func PublicSSHKey(path string) string {
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

// Clients cria ou reutiliza instâncias de clientes OCI com conexão Keep-Alive persistente.
func Clients(configFilePath, profile string) (*ClientSet, error) {
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

// AvailabilityDomains obtém dinamicamente os Availability Domains da região da conta.
func AvailabilityDomains(ctx context.Context, cs *ClientSet, tenancyID, profile, fallbackAD string) []string {
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
