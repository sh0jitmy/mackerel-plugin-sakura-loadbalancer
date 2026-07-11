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

package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strconv"

	mp "github.com/mackerelio/go-mackerel-plugin"
	"github.com/sacloud/iaas-api-go"
	"github.com/sacloud/iaas-api-go/types"
	"github.com/sh0jitmy/mackerel-plugin-sakura-loadbalancer/internal/service"
)

func main() {
	optToken := flag.String("token", "", "Sakura Cloud Access Token")
	optSecret := flag.String("secret", "", "Sakura Cloud Access Token Secret")
	optZone := flag.String("zone", "is1a", "Sakura Cloud Zone")
	optLBID := flag.String("lb-id", "", "Sakura Cloud Load Balancer ID")
	optServerIP := flag.String("server-ip", "", "Sakura Cloud Load Balancer Backend Server IP")
	optPrefix := flag.String("metric-key-prefix", "loadbalancer", "Metric Key Prefix")
	flag.Parse()

	// Parse parameters with environment variable fallback
	token := *optToken
	if token == "" {
		token = os.Getenv("SAKURACLOUD_ACCESS_TOKEN")
		if token == "" {
			token = os.Getenv("SAKURA_ACCESS_TOKEN")
		}
	}

	secret := *optSecret
	if secret == "" {
		secret = os.Getenv("SAKURACLOUD_ACCESS_TOKEN_SECRET")
		if secret == "" {
			secret = os.Getenv("SAKURA_ACCESS_TOKEN_SECRET")
		}
	}

	zone := *optZone
	if zone == "" {
		zone = os.Getenv("SAKURACLOUD_ZONE")
		if zone == "" {
			zone = os.Getenv("SAKURA_ZONE")
			if zone == "" {
				zone = "is1a"
			}
		}
	}

	isMetaMode := os.Getenv("MACKEREL_AGENT_PLUGIN_META") != ""

	// Validation is only strictly enforced when fetching metrics.
	// In metadata output mode, empty parameters are allowed to prevent errors in plugin discovery.
	if !isMetaMode {
		if token == "" || secret == "" {
			log.Fatal("Sakura Cloud Access Token and Secret are required (via -token and -secret flags or SAKURACLOUD_ACCESS_TOKEN and SAKURACLOUD_ACCESS_TOKEN_SECRET env vars)")
		}
		if *optLBID == "" {
			log.Fatal("-lb-id flag is required")
		}
		if *optServerIP == "" {
			log.Fatal("-server-ip flag is required")
		}
	}

	var lbID types.ID
	if *optLBID != "" {
		id, err := strconv.ParseInt(*optLBID, 10, 64)
		if err != nil {
			log.Fatalf("invalid load balancer ID: %v", err)
		}
		lbID = types.ID(id)
	}

	// Initialize API client
	var client *iaas.Client
	var lbOp service.LoadBalancerStatusGetter
	if token != "" && secret != "" {
		client = iaas.NewClient(token, secret)
		lbOp = iaas.NewLoadBalancerOp(client)
	}

	// Create Mackerel plugin instance
	plugin := &service.LoadBalancerPlugin{
		TargetServerIP: *optServerIP,
		LoadBalancerID: lbID,
		Zone:           zone,
		Prefix:         *optPrefix,
		Client:         lbOp,
		Context:        context.Background(),
	}

	// Run standard mackerel plugin routine
	helper := mp.NewMackerelPlugin(plugin)
	helper.Run()
}
