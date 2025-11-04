package ovnnb

import (
	"context"

	models "github.com/cybercoder/ovn-cni/pkg/ovnnb/models"
	"github.com/ovn-kubernetes/libovsdb/client"
	"github.com/ovn-kubernetes/libovsdb/model"
)

type Client struct {
	nbClient client.Client
}

func CreateOvnNbClient(nbEndpoint string) (*Client, error) {
	// Define database model
	dbModel, err := model.NewClientDBModel("OVN_Northbound", map[string]model.Model{
		"Logical_Switch":      &models.LogicalSwitch{},
		"Logical_Switch_Port": &models.LogicalSwitchPort{},
		// Add other table mappings
	})
	if err != nil {
		return nil, err
	}
	dbModel.SetIndexes(map[string][]model.ClientIndex{
		"Logical_Switch": {
			{Columns: []model.ColumnKey{{Column: "name"}}},
		},
	})

	// Create client with connection options
	nbClient, err := client.NewOVSDBClient(dbModel, client.WithEndpoint(nbEndpoint))
	if err != nil {
		return nil, err
	}

	// Establish connection
	ctx := context.Background()
	err = nbClient.Connect(ctx)
	if err != nil {
		return nil, err
	}

	// Start monitoring for cache updates
	_, err = nbClient.MonitorAll(ctx)
	if err != nil {
		return nil, err
	}

	return &Client{nbClient: nbClient}, nil
}

func (c *Client) Close() {
	c.nbClient.Disconnect()
}
