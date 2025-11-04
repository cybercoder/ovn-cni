package ovs

import (
	"context"
	"log"

	ovsModels "github.com/cybercoder/ovn-cni/pkg/ovs/models"
	"github.com/ovn-kubernetes/libovsdb/client"
	"github.com/ovn-kubernetes/libovsdb/model"
)

type Client struct {
	ovsClient client.Client
}

func CreateOVSclient() (*Client, error) {
	dbModel, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{
		"Bridge":    &ovsModels.Bridge{},
		"Port":      &ovsModels.Port{},
		"Interface": &ovsModels.Interface{},
	})
	if err != nil {
		log.Printf("failed to create DB model: %v", err)
		return nil, err
	}

	ovsClient, err := client.NewOVSDBClient(
		dbModel,
		client.WithEndpoint("unix:/var/run/openvswitch/db.sock"),
	)
	if err != nil {
		log.Printf("failed to create OVS client: %v", err)
		return nil, err
	}

	ctx := context.Background()
	if err := ovsClient.Connect(ctx); err != nil {
		log.Printf("failed to connect to OVSDB: %v", err)
		return nil, err
	}
	_, err = ovsClient.MonitorAll(ctx)
	if err != nil {
		log.Printf("failed to monitor OVSDB: %v", err)
		return nil, err
	}
	return &Client{ovsClient: ovsClient}, nil
}

func (c *Client) Close() {
	c.ovsClient.Disconnect()
}
