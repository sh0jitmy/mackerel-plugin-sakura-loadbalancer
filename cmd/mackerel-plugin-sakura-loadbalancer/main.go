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
	"fmt"
	"log"
	"os"
	"strconv"

	mp "github.com/mackerelio/go-mackerel-plugin"
	"github.com/sacloud/iaas-api-go"
	"github.com/sacloud/iaas-api-go/types"
	loadbalancer "github.com/sh0jitmy/mackerel-plugin-sakura-loadbalancer"
	"github.com/sh0jitmy/mackerel-plugin-sakura-loadbalancer/internal/service"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:    "mackerel-plugin-sakura-loadbalancer",
		Usage:   "Mackerel agent plugin for Sakura Cloud Load Balancer",
		Version: loadbalancer.Version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "token",
				EnvVars: []string{"SAKURACLOUD_ACCESS_TOKEN", "SAKURA_ACCESS_TOKEN"},
				Usage:   "Sakura Cloud Access Token (recommended to pass via environment variables for security)",
			},
			&cli.StringFlag{
				Name:    "secret",
				EnvVars: []string{"SAKURACLOUD_ACCESS_TOKEN_SECRET", "SAKURA_ACCESS_TOKEN_SECRET"},
				Usage:   "Sakura Cloud Access Token Secret (recommended to pass via environment variables for security)",
			},
			&cli.StringFlag{
				Name:    "zone",
				Value:   "is1a",
				EnvVars: []string{"SAKURACLOUD_ZONE", "SAKURA_ZONE"},
				Usage:   "Sakura Cloud Zone",
			},
			&cli.StringFlag{
				Name:  "lb-id",
				Usage: "Sakura Cloud Load Balancer Resource ID",
			},
			&cli.StringFlag{
				Name:  "server-ip",
				Usage: "Sakura Cloud Load Balancer Backend Server IP",
			},
			&cli.StringFlag{
				Name:  "metric-key-prefix",
				Value: "loadbalancer",
				Usage: "Metric Key Prefix",
			},
			&cli.BoolFlag{
				Name:    "debug",
				EnvVars: []string{"SAKURA_LB_DEBUG", "DEBUG"},
				Usage:   "Enable debug logging (prints verbose logs to stderr)",
			},
		},
		Action: func(c *cli.Context) error {
			token := c.String("token")
			secret := c.String("secret")
			zone := c.String("zone")
			lbIDStr := c.String("lb-id")
			serverIP := c.String("server-ip")
			prefix := c.String("metric-key-prefix")
			debug := c.Bool("debug")

			isMetaMode := os.Getenv("MACKEREL_AGENT_PLUGIN_META") != ""

			if !isMetaMode {
				if token == "" || secret == "" {
					return cli.Exit("Sakura Cloud Access Token and Secret are required (via --token and --secret flags or SAKURACLOUD_ACCESS_TOKEN and SAKURACLOUD_ACCESS_TOKEN_SECRET env vars)", 1)
				}
				if lbIDStr == "" {
					return cli.Exit("--lb-id flag is required", 1)
				}
				if serverIP == "" {
					return cli.Exit("--server-ip flag is required", 1)
				}
			}

			var lbID types.ID
			if lbIDStr != "" {
				id, err := strconv.ParseInt(lbIDStr, 10, 64)
				if err != nil {
					return fmt.Errorf("invalid load balancer ID: %w", err)
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
				TargetServerIP: serverIP,
				LoadBalancerID: lbID,
				Zone:           zone,
				Prefix:         prefix,
				Client:         lbOp,
				Context:        context.Background(),
				Debug:          debug,
			}

			// Output Mackerel definitions or values
			helper := mp.NewMackerelPlugin(plugin)
			if isMetaMode {
				helper.OutputDefinitions()
			} else {
				helper.OutputValues()
			}
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
