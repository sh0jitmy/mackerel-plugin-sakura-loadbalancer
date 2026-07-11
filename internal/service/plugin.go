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
	"fmt"
	"strings"

	mp "github.com/mackerelio/go-mackerel-plugin"
	"github.com/sacloud/iaas-api-go"
	"github.com/sacloud/iaas-api-go/types"
)

// LoadBalancerStatusGetter defines the interface for retrieving Load Balancer status.
// This allows mocking the Sakura Cloud API calls during tests.
type LoadBalancerStatusGetter interface {
	Status(ctx context.Context, zone string, id types.ID) (*iaas.LoadBalancerStatusResult, error)
}

// LoadBalancerPlugin implements the mackerel-agent-plugin for Sakura Cloud Load Balancer.
type LoadBalancerPlugin struct {
	TargetServerIP string
	LoadBalancerID types.ID
	Zone           string
	Prefix         string
	Client         LoadBalancerStatusGetter
	Context        context.Context
}

// MetricKeyPrefix returns the prefix of metric keys.
func (p *LoadBalancerPlugin) MetricKeyPrefix() string {
	if p.Prefix == "" {
		return "loadbalancer"
	}
	return p.Prefix
}

func title(s string) string {
	if len(s) == 0 {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// GraphDefinition returns the graphs defined by this plugin.
func (p *LoadBalancerPlugin) GraphDefinition() map[string]mp.Graphs {
	labelPrefix := title(p.MetricKeyPrefix())

	return map[string]mp.Graphs{
		"target.status": {
			Label: labelPrefix + " Target Server Status",
			Unit:  "integer",
			Metrics: []mp.Metrics{
				{Name: "status", Label: "Status (1=UP, 0=DOWN)"},
			},
		},
		"target.cps": {
			Label: labelPrefix + " Target Server CPS",
			Unit:  "float",
			Metrics: []mp.Metrics{
				{Name: "cps", Label: "CPS"},
			},
		},
		"target.active_conn": {
			Label: labelPrefix + " Target Server Active Connections",
			Unit:  "integer",
			Metrics: []mp.Metrics{
				{Name: "active_conn", Label: "Active Connections"},
			},
		},
		"server.status.#": {
			Label: labelPrefix + " Server Instance Status",
			Unit:  "integer",
			Metrics: []mp.Metrics{
				{Name: "*", Label: "%1 Status"},
			},
		},
		"server.cps.#": {
			Label: labelPrefix + " Server Instance CPS",
			Unit:  "float",
			Metrics: []mp.Metrics{
				{Name: "*", Label: "%1 CPS"},
			},
		},
		"server.active_conn.#": {
			Label: labelPrefix + " Server Instance Active Connections",
			Unit:  "integer",
			Metrics: []mp.Metrics{
				{Name: "*", Label: "%1 Active Connections"},
			},
		},
	}
}

// FetchMetrics fetches the status from the Load Balancer and returns metrics.
func (p *LoadBalancerPlugin) FetchMetrics() (map[string]float64, error) {
	if p.Client == nil {
		return nil, fmt.Errorf("API client is not initialized")
	}

	result, err := p.Client.Status(p.Context, p.Zone, p.LoadBalancerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get load balancer status: %w", err)
	}

	metrics := make(map[string]float64)

	targetFound := false
	targetAllUp := true
	var totalCPS float64
	var totalActiveConn float64

	for _, vipStatus := range result.Status {
		escapedVIP := strings.ReplaceAll(vipStatus.VirtualIPAddress, ".", "_")
		portStr := vipStatus.Port.String()

		for _, serverStatus := range vipStatus.Servers {
			if serverStatus.IPAddress == p.TargetServerIP {
				targetFound = true
				isUp := strings.ToLower(string(serverStatus.Status)) == "up"
				if !isUp {
					targetAllUp = false
				}

				cpsVal := serverStatus.CPS.Float64()
				connVal := serverStatus.ActiveConn.Float64()

				totalCPS += cpsVal
				totalActiveConn += connVal

				// Individual instance key format (IP Address and Port included to allow multiple virtual server mapping)
				instanceSuffix := fmt.Sprintf("%s_%s", escapedVIP, portStr)

				statusVal := 0.0
				if isUp {
					statusVal = 1.0
				}

				metrics[fmt.Sprintf("server.status.%s", instanceSuffix)] = statusVal
				metrics[fmt.Sprintf("server.cps.%s", instanceSuffix)] = cpsVal
				metrics[fmt.Sprintf("server.active_conn.%s", instanceSuffix)] = connVal
			}
		}
	}

	// Set summary target metrics
	if targetFound {
		if targetAllUp {
			metrics["target.status.status"] = 1.0
		} else {
			metrics["target.status.status"] = 0.0
		}
		metrics["target.cps.cps"] = totalCPS
		metrics["target.active_conn.active_conn"] = totalActiveConn
	} else {
		// Target server not configured on this Load Balancer
		metrics["target.status.status"] = 0.0
		metrics["target.cps.cps"] = 0.0
		metrics["target.active_conn.active_conn"] = 0.0
	}

	return metrics, nil
}
