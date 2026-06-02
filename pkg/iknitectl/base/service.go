package base

import (
	"fmt"
	"log/slog"

	"github.com/kaweezle/iknite/pkg/host"
	"github.com/kaweezle/iknite/pkg/iknitectl/config"
	"github.com/kaweezle/iknite/pkg/iknitectl/db"
	"github.com/kaweezle/iknite/pkg/utils"
)

type StoreProvider interface {
	Store() (*db.Store, error)
}

type ServiceInterface interface {
	host.HostProvider
	config.ConfigProvider
	utils.LoggerHolder
	utils.LoggerProvider
	StoreProvider
}

type Service struct {
	utils.LogEnabled
	localHost host.Host
	config    *config.Config
	store     *db.Store
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

func (s *Service) Store() (*db.Store, error) {
	if s.store != nil {
		return s.store, nil
	}
	store, err := db.Open(s.Config().Database)
	if err != nil {
		return nil, fmt.Errorf("failed to open metadata store: %w", err)
	}
	s.store = store
	return store, nil
}

func (s *Service) CloseStore() error {
	if s.store != nil {
		if err := s.store.Close(); err != nil {
			return fmt.Errorf("failed to close metadata store: %w", err)
		}
	}
	return nil
}
