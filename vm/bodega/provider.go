package bodega

import (
	"fmt"
	"log"
	"time"

	"github.com/cockroachdb/roachprod/vm"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
)

const (
	ProviderName = "bodega"
	numRetries   = 30
)

// init will inject the bodega provider into vm.Providers, but only
// if the bodega token is available on the local path.
func init() {
	manager, err := newBodegaManager()
	if err != nil {
		log.Println("Bodega is not available")
	} else {
		vm.Providers[ProviderName] = &Provider{manager: *manager}
	}
}

// providerOpts implements the vm.ProviderFlags interface for bodega.Provider.
type providerOpts struct {
	DiskSize int
	Location string
	Model    string
}

func (o *providerOpts) ConfigureCreateFlags(flags *pflag.FlagSet) {
	flags.IntVar(&o.DiskSize, ProviderName+"-disk_size", 32, "Bodega disk size")
	flags.StringVar(&o.Location, ProviderName+"-location", "AWS-US-WEST-1", "Bodega location")
	flags.StringVar(&o.Model, ProviderName+"-model", "aws-m4.large", "Bodega model")
}

func (o *providerOpts) ConfigureClusterFlags(flags *pflag.FlagSet) {
}

// Provider implements the vm.Provider interface for Bodega.
type Provider struct {
	opts    providerOpts
	manager bodegaManager
}

// CleanSSH is part of the vm.Provider interface.  This implementation is a no-op.
func (p *Provider) CleanSSH() error {
	return nil
}

// ConfigSSH is part of the vm.Provider interface.  This implementation is a no-op.
func (p *Provider) ConfigSSH() error {
	return nil
}

// Create is part of the vm.Provider interface.
func (p *Provider) Create(names []string, opts vm.CreateOpts) error {
	orderID, err := p.manager.placeOrder(names, p.opts)
	if err != nil {
		return errors.Wrapf(err, "failed to place order with Bodega")
	}

	for t := 0; !p.manager.fulfilled(orderID); t++ {
		if t >= numRetries {
			return fmt.Errorf("order not fulfilled after %d minutes", t)
		}
		log.Printf("It has been %d minutes since the creation of order %s", t, orderID)
		time.Sleep(1 * time.Minute)
	}
	log.Printf("order %s is fulfilled", orderID)

	return p.manager.addOrderInfo(orderID, names)
}

// Delete is part of the vm.Provider interface.
func (p *Provider) Delete(vms vm.List) error {
	return nil
}

// Extend is part of the vm.Provider interface.
func (p *Provider) Extend(vms vm.List, lifetime time.Duration) error {
	return nil
}

// FindActiveAccount is part of the vm.Provider interface.
func (p *Provider) FindActiveAccount() (string, error) {
	return "ubuntu", nil
}

// Flags is part of the vm.Provider interface.
func (p *Provider) Flags() vm.ProviderFlags {
	return &p.opts
}

// List is part of the vm.Provider interface.
func (p *Provider) List() (vm.List, error) {
	orderMap, err := p.manager.orderToMachineMap()
	if err != nil {
		return nil, err
	}

	var allMachines vm.List
	for orderID := range orderMap {
		order, err := p.manager.getOrder(orderID)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to consume order %s", orderID)
		}
		machines := order["fulfilled_items"].(map[string]interface{})
		for name, spec := range machines {
			newMachine := new(vm.VM)
			newMachine.Name = name
			newMachine.Provider = ProviderName
			newMachine.ProviderID = ProviderName

			specMap := spec.(map[string]interface{})
			createTime, err := time.Parse(time.RFC3339, specMap["time_created"].(string))
			if err != nil {
				return nil, errors.Wrapf(err, "unable to parse time string")
			}
			newMachine.CreatedAt = createTime
			newMachine.PublicIP = specMap["ipv4"].(string)
			newMachine.RemoteUser = specMap["username"].(string)
			newMachine.Zone = specMap["location"].(string)
			allMachines = append(allMachines, *newMachine)
		}
	}
	return allMachines, nil
}

// Name returns the name of the Provider, which will also surface in VM.Provider
func (p *Provider) Name() string {
	return ProviderName
}
