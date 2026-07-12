// Copyright 2026 sh0jitmy
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Author: sh0jitmy

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/sacloud/iaas-api-go"
	"github.com/sacloud/iaas-api-go/types"
)

// TestMain manages the execution of tests and performs goroutine leak detection.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type mockLoadBalancerClient struct {
	statusFunc func(ctx context.Context, zone string, id types.ID) (*iaas.LoadBalancerStatusResult, error)
}

func (m *mockLoadBalancerClient) Status(ctx context.Context, zone string, id types.ID) (*iaas.LoadBalancerStatusResult, error) {
	return m.statusFunc(ctx, zone, id)
}

func TestLoadBalancerPlugin_MetricKeyPrefix(t *testing.T) {
	t.Parallel()
	p := &LoadBalancerPlugin{Prefix: "custom_prefix"}
	assert.Equal(t, "custom_prefix", p.MetricKeyPrefix())

	pDefault := &LoadBalancerPlugin{}
	assert.Equal(t, "loadbalancer", pDefault.MetricKeyPrefix())
}

func TestLoadBalancerPlugin_GraphDefinition(t *testing.T) {
	t.Parallel()
	p := &LoadBalancerPlugin{Prefix: "test_lb"}
	graphs := p.GraphDefinition()

	assert.Contains(t, graphs, "target.status")
	assert.Contains(t, graphs, "target.cps")
	assert.Contains(t, graphs, "target.active_conn")
	assert.Contains(t, graphs, "server.status.#")
	assert.Contains(t, graphs, "server.cps.#")
	assert.Contains(t, graphs, "server.active_conn.#")

	assert.Equal(t, "Test_lb Target Server Status", graphs["target.status"].Label)
}

func TestLoadBalancerPlugin_FetchMetrics_SuccessAllUp(t *testing.T) {
	t.Parallel()
	mockClient := &mockLoadBalancerClient{
		statusFunc: func(ctx context.Context, zone string, id types.ID) (*iaas.LoadBalancerStatusResult, error) {
			return &iaas.LoadBalancerStatusResult{
				Status: []*iaas.LoadBalancerStatus{
					{
						VirtualIPAddress: "192.0.2.1",
						Port:             80,
						CPS:              100,
						Servers: []*iaas.LoadBalancerServerStatus{
							{
								IPAddress:  "192.168.1.10",
								Port:       80,
								Status:     types.ServerInstanceStatuses.Up,
								CPS:        45,
								ActiveConn: 5,
							},
						},
					},
				},
			}, nil
		},
	}

	p := &LoadBalancerPlugin{
		TargetServerIP: "192.168.1.10",
		LoadBalancerID: types.ID(123456789012),
		Zone:           "is1a",
		Client:         mockClient,
		Context:        context.Background(),
		Debug:          true,
	}

	metrics, err := p.FetchMetrics()
	require.NoError(t, err)
	assert.NotNil(t, metrics)

	// Summary metrics
	assert.InDelta(t, 1.0, metrics["target.status.status"], 1e-9)
	assert.InDelta(t, 45.0, metrics["target.cps.cps"], 1e-9)
	assert.InDelta(t, 5.0, metrics["target.active_conn.active_conn"], 1e-9)

	// Instance metrics
	assert.InDelta(t, 1.0, metrics["server.status.192_0_2_1_80"], 1e-9)
	assert.InDelta(t, 45.0, metrics["server.cps.192_0_2_1_80"], 1e-9)
	assert.InDelta(t, 5.0, metrics["server.active_conn.192_0_2_1_80"], 1e-9)
}

func TestLoadBalancerPlugin_FetchMetrics_SuccessSomeDown(t *testing.T) {
	t.Parallel()
	mockClient := &mockLoadBalancerClient{
		statusFunc: func(ctx context.Context, zone string, id types.ID) (*iaas.LoadBalancerStatusResult, error) {
			return &iaas.LoadBalancerStatusResult{
				Status: []*iaas.LoadBalancerStatus{
					{
						VirtualIPAddress: "192.0.2.1",
						Port:             80,
						CPS:              100,
						Servers: []*iaas.LoadBalancerServerStatus{
							{
								IPAddress:  "192.168.1.10",
								Port:       80,
								Status:     types.ServerInstanceStatuses.Up,
								CPS:        45,
								ActiveConn: 5,
							},
						},
					},
					{
						VirtualIPAddress: "192.0.2.1",
						Port:             443,
						CPS:              200,
						Servers: []*iaas.LoadBalancerServerStatus{
							{
								IPAddress:  "192.168.1.10",
								Port:       443,
								Status:     types.ServerInstanceStatuses.Down,
								CPS:        0,
								ActiveConn: 0,
							},
						},
					},
				},
			}, nil
		},
	}

	p := &LoadBalancerPlugin{
		TargetServerIP: "192.168.1.10",
		LoadBalancerID: types.ID(123456789012),
		Zone:           "is1a",
		Client:         mockClient,
		Context:        context.Background(),
	}

	metrics, err := p.FetchMetrics()
	require.NoError(t, err)
	assert.NotNil(t, metrics)

	// Summary metrics: one UP and one DOWN means target status is DOWN (0.0)
	assert.InDelta(t, 0.0, metrics["target.status.status"], 1e-9)
	assert.InDelta(t, 45.0, metrics["target.cps.cps"], 1e-9)
	assert.InDelta(t, 5.0, metrics["target.active_conn.active_conn"], 1e-9)

	// Instance metrics
	assert.InDelta(t, 1.0, metrics["server.status.192_0_2_1_80"], 1e-9)
	assert.InDelta(t, 0.0, metrics["server.status.192_0_2_1_443"], 1e-9)
}

func TestLoadBalancerPlugin_FetchMetrics_ServerNotFound(t *testing.T) {
	t.Parallel()
	mockClient := &mockLoadBalancerClient{
		statusFunc: func(ctx context.Context, zone string, id types.ID) (*iaas.LoadBalancerStatusResult, error) {
			return &iaas.LoadBalancerStatusResult{
				Status: []*iaas.LoadBalancerStatus{
					{
						VirtualIPAddress: "192.0.2.1",
						Port:             80,
						CPS:              100,
						Servers: []*iaas.LoadBalancerServerStatus{
							{
								IPAddress:  "192.168.1.20",
								Port:       80,
								Status:     types.ServerInstanceStatuses.Up,
								CPS:        45,
								ActiveConn: 5,
							},
						},
					},
				},
			}, nil
		},
	}

	p := &LoadBalancerPlugin{
		TargetServerIP: "192.168.1.10", // Target server is not configured
		LoadBalancerID: types.ID(123456789012),
		Zone:           "is1a",
		Client:         mockClient,
		Context:        context.Background(),
	}

	metrics, err := p.FetchMetrics()
	require.NoError(t, err)
	assert.NotNil(t, metrics)

	// Summary metrics: not found means 0.0
	assert.InDelta(t, 0.0, metrics["target.status.status"], 1e-9)
	assert.InDelta(t, 0.0, metrics["target.cps.cps"], 1e-9)
	assert.InDelta(t, 0.0, metrics["target.active_conn.active_conn"], 1e-9)

	// No instance metrics should exist for the target server
	assert.Len(t, metrics, 3) // Only summary metrics exist
}

func TestLoadBalancerPlugin_FetchMetrics_APIError(t *testing.T) {
	t.Parallel()
	mockClient := &mockLoadBalancerClient{
		statusFunc: func(ctx context.Context, zone string, id types.ID) (*iaas.LoadBalancerStatusResult, error) {
			return nil, errors.New("API error occurred")
		},
	}

	p := &LoadBalancerPlugin{
		TargetServerIP: "192.168.1.10",
		LoadBalancerID: types.ID(123456789012),
		Zone:           "is1a",
		Client:         mockClient,
		Context:        context.Background(),
	}

	metrics, err := p.FetchMetrics()
	require.Error(t, err)
	assert.Nil(t, metrics)
	assert.Contains(t, err.Error(), "failed to get load balancer status")
}

func TestLoadBalancerPlugin_FetchMetrics_NilClient(t *testing.T) {
	t.Parallel()
	p := &LoadBalancerPlugin{
		TargetServerIP: "192.168.1.10",
		LoadBalancerID: types.ID(123456789012),
		Zone:           "is1a",
		Client:         nil,
		Context:        context.Background(),
	}

	metrics, err := p.FetchMetrics()
	require.Error(t, err)
	assert.Nil(t, metrics)
	assert.Contains(t, err.Error(), "API client is not initialized")
}
