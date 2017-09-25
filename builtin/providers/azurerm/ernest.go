package azurerm

import (
	"errors"
	"log"
	"strings"

	"github.com/Azure/azure-sdk-for-go/storage"
)

// ResourceArmLoadBalancerRules : ..
func (armClient *ArmClient) ResourceArmLoadBalancerRules(lbID, ruleID string) (rules map[string]interface{}, err error) {
	id, err := parseAzureResourceID(ruleID)
	if err != nil {
		return rules, err
	}
	name := id.Path["loadBalancingRules"]

	loadBalancer, exists, err := retrieveLoadBalancerById(lbID, armClient)
	if err != nil {
		return rules, errors.New("Error Getting LoadBalancer By ID {{err}}")
	}
	if !exists {
		rules["id"] = ""
		log.Printf("[INFO] LoadBalancer %q not found. Removing from state", name)
		return rules, nil
	}

	config, _, exists := findLoadBalancerRuleByName(loadBalancer, name)
	if !exists {
		rules["id"] = ""
		log.Printf("[INFO] LoadBalancer Rule %q not found. Removing from state", name)
		return rules, nil
	}

	rules["name"] = config.Name
	rules["resource_group_name"] = id.ResourceGroup

	rules["protocol"] = config.LoadBalancingRulePropertiesFormat.Protocol
	rules["frontend_port"] = config.LoadBalancingRulePropertiesFormat.FrontendPort
	rules["backend_port"] = config.LoadBalancingRulePropertiesFormat.BackendPort

	if config.LoadBalancingRulePropertiesFormat.EnableFloatingIP != nil {
		rules["enable_floating_ip"] = config.LoadBalancingRulePropertiesFormat.EnableFloatingIP
	}

	if config.LoadBalancingRulePropertiesFormat.IdleTimeoutInMinutes != nil {
		rules["idle_timeout_in_minutes"] = config.LoadBalancingRulePropertiesFormat.IdleTimeoutInMinutes
	}

	if config.LoadBalancingRulePropertiesFormat.FrontendIPConfiguration != nil {
		fipID, err := parseAzureResourceID(*config.LoadBalancingRulePropertiesFormat.FrontendIPConfiguration.ID)
		if err != nil {
			return rules, err
		}

		rules["frontend_ip_configuration_name"] = fipID.Path["frontendIPConfigurations"]
		rules["frontend_ip_configuration_id"] = config.LoadBalancingRulePropertiesFormat.FrontendIPConfiguration.ID
	}

	if config.LoadBalancingRulePropertiesFormat.BackendAddressPool != nil {
		rules["backend_address_pool_id"] = config.LoadBalancingRulePropertiesFormat.BackendAddressPool.ID
	}

	if config.LoadBalancingRulePropertiesFormat.Probe != nil {
		rules["probe_id"] = config.LoadBalancingRulePropertiesFormat.Probe.ID
	}

	if config.LoadBalancingRulePropertiesFormat.LoadDistribution != "" {
		rules["load_distribution"] = config.LoadBalancingRulePropertiesFormat.LoadDistribution
	}

	return rules, nil

}

// GetVMStorageImageReference :
func (armClient *ArmClient) GetVMStorageImageReference(resGroup, name string) map[string]interface{} {
	resp, err := armClient.vmClient.Get(resGroup, name, "")
	if err != nil {
		return nil
	}

	ref := make(map[string]interface{})
	image := resp.VirtualMachineProperties.StorageProfile.ImageReference
	ref["offer"] = *image.Offer
	ref["publisher"] = *image.Publisher
	ref["sku"] = *image.Sku

	if image.Version != nil {
		ref["version"] = *image.Version
	}

	return ref
}

// ListNetworkInterfaceConfigurations : ..
func (armClient *ArmClient) ListNetworkInterfaceConfigurations(resourceGroupName, networkInterfaceName string) []map[string]string {
	ipConfigurations := make([]map[string]string, 0)
	interfaces, _ := armClient.ifaceClient.List(resourceGroupName)
	for _, val := range *interfaces.Value {
		for _, ip := range *val.IPConfigurations {
			/*
				addressPools := make([]string, 0, len(*ip.LoadBalancerBackendAddressPools))
				for _, pool := range *ip.LoadBalancerBackendAddressPools {
					addressPools = append(addressPools, *pool.ID)
				}
				natRules := make([]string, 0, len(*ip.LoadBalancerInboundNatRules))
				for _, pool := range *ip.LoadBalancerInboundNatRules {
					natRules = append(natRules, *pool.ID)
				}
			*/

			ipConfiguration := map[string]string{
				"name":                                    *ip.Name,
				"subnet_id":                               *ip.Subnet.ID,
				"interface":                               *val.Name,
				"private_ip_address":                      *ip.PrivateIPAddress,
				"private_ip_address_allocation":           string(ip.PrivateIPAllocationMethod),
				"load_balancer_backend_address_pools_ids": "", //strings.Join(addressPools, ","),
				"load_balancer_inbound_nat_rules_ids ":    "", // strings.Join(natRules, ","),
			}
			if ip.PublicIPAddress != nil {
				ipConfiguration["public_ip_address_id"] = *ip.PublicIPAddress.ID
			}
			ipConfigurations = append(ipConfigurations, ipConfiguration)
		}
	}

	return ipConfigurations
}

// ListResourcesByGroup : ..
func (armClient *ArmClient) ListResourcesByGroup(resourceGroupName, filters, expand string) (m map[string][]string, err error) {
	m = make(map[string][]string)
	results, err := armClient.resourceGroupClient.ListResources(resourceGroupName, filters, expand, nil)
	if err != nil {
		log.Println(err.Error())
		return m, nil
	}

	if &results != nil {
		for _, v := range *results.Value {
			t := *v.Type
			id := *v.ID
			name := *v.Name
			if _, ok := m[t]; !ok {
				m[t] = make([]string, 0)
			}

			if t == "Microsoft.Network/virtualNetworks" {
				// Look for Subnets
				res, _ := armClient.subnetClient.List(resourceGroupName, name)
				if &res != nil {
					for _, sub := range *res.Value {
						subid := *sub.ID
						subT := "Microsoft.Network/subnets"
						if _, ok := m[subT]; !ok {
							m[subT] = make([]string, 0)
						}

						m[subT] = append(m["azurerm_subnet"], subid)
					}
				}
			}

			if t == "Microsoft.Storage/storageAccounts" {
				// Look for Storage Containers
				conT := "Microsoft.Storage/storageContainers"
				m[conT] = make([]string, 0)
				blobClient, _, _ := armClient.getBlobStorageClientForStorageAccount(resourceGroupName, name)
				containers, err := blobClient.ListContainers(storage.ListContainersParameters{})
				if err != nil {
					log.Println(err.Error())
				}
				for _, container := range containers.Containers {
					access, err := blobClient.GetContainerPermissions(container.Name, 0, "")
					if err != nil {
						log.Println(err.Error())
					}
					t := string(access.AccessType)
					parts := strings.Split(id, "/")

					tid := "/" + conT + "/" + parts[4] + "::" + parts[len(parts)-1] + "::" + container.Name + "::" + t
					m[conT] = append(m[conT], tid)
				}
			}

			m[t] = append(m[t], id)
		}
	}

	// Import loadbalancers
	rg, err := armClient.resourceGroupClient.Get(resourceGroupName)
	if &rg != nil {
		t := "Microsoft.Network/loadBalancers"
		m[t] = append(m[t], *rg.ID)
	}

	for _, id := range m["Microsoft.Network/loadBalancers"] {
		if strings.Contains(id, "loadBalancers") {
			loadBalancer, exists, err := retrieveLoadBalancerById(id, armClient)
			if err == nil && exists {
				rules := loadBalancer.LoadBalancerPropertiesFormat.LoadBalancingRules
				for _, rule := range *rules {
					t := "Microsoft.Network/loadBalancers/loadBalancingRules"
					m[t] = append(m[t], *rule.ID)
				}

				pools := loadBalancer.LoadBalancerPropertiesFormat.BackendAddressPools
				for _, pool := range *pools {
					t := "Microsoft.Network/loadBalancers/backendAddressPools"
					m[t] = append(m[t], *pool.ID)
				}

				probes := loadBalancer.LoadBalancerPropertiesFormat.Probes
				for _, probe := range *probes {
					t := "Microsoft.Network/loadBalancers/probes"
					m[t] = append(m[t], *probe.ID)
				}

			}
		}
	}

	for k, v := range m {
		log.Println(k)
		for _, s := range v {
			log.Println(" - " + s)
		}
	}
	return m, nil
}
