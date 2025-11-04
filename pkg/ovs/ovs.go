package ovs

import (
	"context"
	"fmt"
	"log"

	ovsModel "github.com/cybercoder/ovn-cni/pkg/ovs/models"
	"github.com/google/uuid"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
)

func (c *Client) AddPort(bridgeName, portName, ifaceType string) error {
	ctx := context.Background()
	ifaceUUID := uuid.New().String()
	portUUID := uuid.New().String()
	bridge := &ovsModel.Bridge{Name: bridgeName}
	if err := c.ovsClient.Get(ctx, bridge); err != nil {
		return fmt.Errorf("failed to get bridge %q: %v", bridgeName, err)
	}

	iface := &ovsModel.Interface{
		UUID: ifaceUUID,
		Name: portName,
		Type: ifaceType,
		// MAC:  &macAddress,
		ExternalIDs: map[string]string{
			"iface-id": portName,
		},
	}

	ifaceOp, err := c.ovsClient.Create(iface)
	if err != nil {
		return fmt.Errorf("failed to create interface: %v", err)
	}

	port := &ovsModel.Port{
		UUID:       portUUID,
		Name:       portName,
		Interfaces: []string{iface.UUID},
	}
	portOp, err := c.ovsClient.Create(port)
	if err != nil {
		return fmt.Errorf("failed to create port: %v", err)
	}

	mutations := []model.Mutation{
		{
			Field:   &bridge.Ports,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   []string{port.UUID},
		},
	}
	mutateOps, err := c.ovsClient.Where(bridge).Mutate(bridge, mutations...)
	if err != nil {
		return fmt.Errorf("failed to prepare mutation: %v", err)
	}
	ops := append(ifaceOp, append(portOp, mutateOps...)...)
	reply, err := c.ovsClient.Transact(ctx, ops...)
	if err != nil {
		return fmt.Errorf("transaction failed: %v", err)
	}

	for i, r := range reply {
		if r.Error != "" {
			log.Printf("OVSDB error: %d %s (%s)", i, r.Error, r.Details)
		}
	}
	log.Printf("✅ Added port %s to bridge %s (type=%s)", portName, bridgeName, ifaceType)
	return nil
}

func (c *Client) DelPort(bridgeName, portName string) error {
	ctx := context.Background()

	// 1. Get port and bridge objects
	port := &ovsModel.Port{Name: portName}
	if err := c.ovsClient.Get(ctx, port); err != nil {
		return fmt.Errorf("failed to find port %s: %v", portName, err)
	}

	bridge := &ovsModel.Bridge{Name: bridgeName}
	if err := c.ovsClient.Get(ctx, bridge); err != nil {
		return fmt.Errorf("failed to find bridge %s: %v", bridgeName, err)
	}

	// 2. Mutate the bridge to remove the port UUID from its Ports set
	mutations := []model.Mutation{
		{
			Field:   &bridge.Ports,
			Mutator: ovsdb.MutateOperationDelete,
			Value:   []string{port.UUID},
		},
	}
	mutateOps, err := c.ovsClient.Where(bridge).Mutate(bridge, mutations...)
	if err != nil {
		return fmt.Errorf("failed to prepare bridge mutation: %v", err)
	}

	// 3. Delete the port itself (and the interface)
	portOp, err := c.ovsClient.Where(port).Delete()
	if err != nil {
		return fmt.Errorf("failed to prepare port delete: %v", err)
	}

	// 4. Also delete the Interface row(s) belonging to the port
	for _, ifaceUUID := range port.Interfaces {
		iface := &ovsModel.Interface{UUID: ifaceUUID}
		ifaceOp, err := c.ovsClient.Where(iface).Delete()
		if err != nil {
			return fmt.Errorf("failed to prepare interface delete: %v", err)
		}
		portOp = append(portOp, ifaceOp...)
	}

	// 5. Run all operations in one transaction
	ops := append(mutateOps, portOp...)
	reply, err := c.ovsClient.Transact(ctx, ops...)
	if err != nil {
		return fmt.Errorf("transaction failed: %v", err)
	}

	for i, r := range reply {
		if r.Error != "" {
			log.Printf("OVSDB error: %d %s (%s)", i, r.Error, r.Details)
		}
	}

	log.Printf("🧹 Deleted port %s from bridge %s", portName, bridgeName)
	return nil
}
