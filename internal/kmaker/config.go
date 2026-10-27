package kmaker

import (
	"fmt"
	"net"
	"os"

	pb "gitlab.com/seabeam/schematic/platform/services/kmaker/model/types/services/kmaker"
	hostpb "gitlab.com/seabeam/schematic/platform/services/kmaker/model/types/sys/host"
)

const (
	defaultListenHost            = "0.0.0.0"
	defaultListenPort      int32 = 8443
	defaultSpiffePrefix          = "local.dev"
	defaultTokenAudience         = "kmaker"
	defaultCertLifetime          = int64(86400)
	defaultKernelNamespace       = "schematic"
	defaultKernelSAName          = "kernel"
)

type ServiceAccount struct {
	Namespace string
	Name      string
}

func ResolveConfigPath(args []string) string {
	for i := 0; i < len(args)-1; i++ {
		switch args[i] {
		case "-c", "--config":
			return args[i+1]
		}
	}
	return os.Getenv("SERVICE_CONFIG")
}

func LoadConfigFile(path string) (*pb.Config, error) {
	if path == "" {
		return nil, fmt.Errorf("no config: use --config flag or set SERVICE_CONFIG")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	cfg, err := pb.ConfigFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	NormalizeConfig(cfg)
	if err := pb.ValidateConfig(cfg); err != nil {
		return nil, fmt.Errorf("validating config %s: %w", path, err)
	}
	return cfg, nil
}

func NormalizeConfig(cfg *pb.Config) {
	if cfg.Listener == nil {
		cfg.Listener = &hostpb.Host{}
	}
	if cfg.Listener.Host == "" {
		cfg.Listener.Host = defaultListenHost
	}
	if cfg.Listener.Port == 0 {
		cfg.Listener.Port = defaultListenPort
	}
	if cfg.SpiffePrefix == "" {
		cfg.SpiffePrefix = defaultSpiffePrefix
	}
	if cfg.TokenAudience == "" {
		cfg.TokenAudience = defaultTokenAudience
	}
	if cfg.KernelCertLifetimeSeconds <= 0 {
		cfg.KernelCertLifetimeSeconds = defaultCertLifetime
	}
	if len(cfg.KernelServiceAccounts) == 0 {
		cfg.KernelServiceAccounts = []*pb.KernelServiceAccount{{
			Namespace: defaultKernelNamespace,
			Name:      defaultKernelSAName,
		}}
	}
}

func ListenAddress(cfg *pb.Config) string {
	host := defaultListenHost
	port := defaultListenPort
	if cfg != nil && cfg.Listener != nil {
		if cfg.Listener.GetHost() != "" {
			host = cfg.Listener.GetHost()
		}
		if cfg.Listener.GetPort() != 0 {
			port = cfg.Listener.GetPort()
		}
	}
	return net.JoinHostPort(host, fmt.Sprintf("%d", port))
}

func AllowedServiceAccounts(cfg *pb.Config) map[ServiceAccount]struct{} {
	allowed := make(map[ServiceAccount]struct{})
	for _, sa := range cfg.GetKernelServiceAccounts() {
		if sa.GetNamespace() == "" || sa.GetName() == "" {
			continue
		}
		allowed[ServiceAccount{Namespace: sa.GetNamespace(), Name: sa.GetName()}] = struct{}{}
	}
	if len(allowed) == 0 {
		allowed[ServiceAccount{Namespace: defaultKernelNamespace, Name: defaultKernelSAName}] = struct{}{}
	}
	return allowed
}
