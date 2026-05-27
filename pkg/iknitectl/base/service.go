package base

import (
	"fmt"
	"log/slog"

	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/iknitectl/config"
	"github.com/kaweezle/iknite/pkg/utils"
)

type ServiceInterface interface {
	host.HostProvider
	config.ConfigProvider
	utils.LoggerHolder
	utils.LoggerProvider
}

type Service struct {
	utils.LogEnabled
	localHost host.Host
	config    *config.Config
}

var _ ServiceInterface = (*Service)(nil)

func NewService(localHost host.Host, logger *slog.Logger, opts *config.ConfigOptions) *Service {
	c := &config.Config{}
	err := opts.Resolve(localHost, c)
	if err != nil {
		panic(fmt.Sprintf("Failed to resolve config: %v", err))
	}
	return &Service{
		localHost:  localHost,
		config:     c,
		LogEnabled: utils.LogEnabled{LogEntry: logger},
	}
}

func (s *Service) Host() host.Host {
	return s.localHost
}

func (s *Service) Config() *config.Config {
	return s.config
}
