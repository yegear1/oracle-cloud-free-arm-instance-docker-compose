package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"

	"oracle-fisher/internal/config"
	"oracle-fisher/internal/logging"
	"oracle-fisher/internal/notify"
	"oracle-fisher/internal/oci"
)

const (
	launchNetworkAttempts  = 3
	launchRetryBackoffUnit = 2 * time.Second
)

func main() {
	config.LoadEnv(".env")
	logging.Init()

	configFilePath := config.Get("OCI_CONFIG_FILE", "/root/.oci/config")
	requestInterval := config.GetInt("requestInterval", 60)
	accounts := config.LoadAccounts()

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
				profile = config.Get("OCI_PROFILE", "DEFAULT")
			}

			if successAccounts[profile] {
				continue
			}

			allDone = false

			tenancyID := acc.TenancyID
			if tenancyID == "" {
				tenancyID = config.Get("TENANCY_ID", "")
			}
			imageID := acc.ImageID
			if imageID == "" {
				imageID = config.Get("IMAGE_ID", "")
			}
			subnetID := acc.SubnetID
			if subnetID == "" {
				subnetID = config.Get("SUBNET_ID", "")
			}
			sshKeyPath := acc.SSHKey
			if sshKeyPath == "" {
				sshKeyPath = config.Get("PATH_TO_PUBLIC_SSH_KEY", "/root/.oci/chave_vps_arm.pub")
			}
			cpus := acc.CPUs
			if cpus == 0 {
				cpus = config.GetFloat("cpus", 2)
			}
			ram := acc.RAM
			if ram == 0 {
				ram = config.GetFloat("ram", 12)
			}
			bootVolume := acc.BootVolume
			if bootVolume == 0 {
				bootVolume = config.GetInt("bootVolume", 100)
			}
			displayName := acc.DisplayName
			if displayName == "" {
				displayName = fmt.Sprintf("%s-%s", config.Get("DISPLAY_NAME", "big-arm"), profile)
			}
			fallbackAD := acc.AvailabilityDomain
			if fallbackAD == "" {
				fallbackAD = config.Get("AVAILABILITY_DOMAIN", "")
			}

			cs, err := oci.Clients(configFilePath, profile)
			if err != nil {
				logging.Error("Pulando conta devido a erro nas credenciais", err, "profile", profile)
				continue
			}

			ads := oci.AvailabilityDomains(ctx, cs, tenancyID, profile, fallbackAD)
			if len(ads) == 0 {
				slog.Warn("Nenhum Availability Domain encontrado", "profile", profile)
				continue
			}

			sshKeyContent := oci.PublicSSHKey(sshKeyPath)
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
					if attempt < launchNetworkAttempts && oci.TransientNetworkError(err) {
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
					notify.Send(fmt.Sprintf(
						"🎉 Instância ARM criada com sucesso na Oracle Cloud!\nConta: %s\nAD: %s\nNome: %s\nHardware: %.0f OCPUs, %.0fGB RAM, %dGB Disco",
						profile, ad, displayName, cpus, ram, bootVolume,
					))
					break
				}

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
					logging.Error("Erro inesperado ao criar instância", err,
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
			notify.Send("🏁 Todas as instâncias ARM solicitadas foram criadas com sucesso na Oracle Cloud!")
			os.Exit(0)
		}

		slog.Debug("Aguardando próximo ciclo", "interval_seconds", requestInterval)
		time.Sleep(time.Duration(requestInterval) * time.Second)
	}
}
