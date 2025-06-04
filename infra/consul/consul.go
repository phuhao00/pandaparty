package consulx

import (
	"fmt"

	"github.com/hashicorp/consul/api"
	"github.com/phuhao00/pandaparty/config" // Added import
)

type ConsulClient struct {
	client *api.Client
}

func (c *ConsulClient) GetReal() *api.Client {
	return c.client
}

func NewConsulClient(cfg config.ConsulConfig) (*ConsulClient, error) {
	apiClientConfig := api.DefaultConfig() // Renamed 'config' to 'apiClientConfig'
	if cfg.Addr != "" {
		apiClientConfig.Address = cfg.Addr
	}
	client, err := api.NewClient(apiClientConfig)
	if err != nil {
		return nil, err
	}
	return &ConsulClient{client: client}, nil
}

func (c *ConsulClient) RegisterService(id, name, address string, port int) error {
	reg := &api.AgentServiceRegistration{
		ID:      id,
		Name:    name,
		Address: address,
		Port:    port,
	}
	return c.client.Agent().ServiceRegister(reg)
}

func (c *ConsulClient) DiscoverService(name string) ([]*api.ServiceEntry, error) {
	services, _, err := c.client.Health().Service(name, "", true, nil)
	return services, err
}

// DeregisterService removes a service registration from Consul.
func (c *ConsulClient) DeregisterService(serviceID string) error {
	return c.client.Agent().ServiceDeregister(serviceID)
}

// ServiceInfo represents a service instance
type ServiceInfo struct {
	ID      string
	Name    string
	Address string
	Port    int
	Tags    []string
	Meta    map[string]string
}

// GetHealthyServices returns all healthy instances of a given service
func (c *ConsulClient) GetHealthyServices(serviceName string) ([]*ServiceInfo, error) {
	if c.client == nil {
		return nil, fmt.Errorf("consul client is not initialized")
	}

	// Query healthy services
	services, _, err := c.client.Health().Service(serviceName, "", true, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query service %s: %w", serviceName, err)
	}

	var result []*ServiceInfo
	for _, entry := range services {
		if entry.Service == nil {
			continue
		}

		service := &ServiceInfo{
			ID:      entry.Service.ID,
			Name:    entry.Service.Service,
			Address: entry.Service.Address,
			Port:    entry.Service.Port,
			Tags:    entry.Service.Tags,
			Meta:    entry.Service.Meta,
		}

		// Use node address if service address is empty
		if service.Address == "" && entry.Node != nil {
			service.Address = entry.Node.Address
		}

		result = append(result, service)
	}

	return result, nil
}

// GetService returns a specific service instance by ID
func (c *ConsulClient) GetService(serviceID string) (*ServiceInfo, error) {
	if c.client == nil {
		return nil, fmt.Errorf("consul client is not initialized")
	}

	services, err := c.client.Agent().Services()
	if err != nil {
		return nil, fmt.Errorf("failed to get services: %w", err)
	}

	if service, exists := services[serviceID]; exists {
		return &ServiceInfo{
			ID:      service.ID,
			Name:    service.Service,
			Address: service.Address,
			Port:    service.Port,
			Tags:    service.Tags,
			Meta:    service.Meta,
		}, nil
	}

	return nil, fmt.Errorf("service with ID %s not found", serviceID)
}
