// Copyright 2022 Evan Hazlett
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
package main

import (
	"fmt"
	"net"
	"os"

	"github.com/fynca/fynca"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

func getHostname() string {
	if h, _ := os.Hostname(); h != "" {
		return h
	}

	return "unknown"
}

func getIP(clix *cli.Context) string {
	ip := "127.0.0.1"
	devName := clix.String("nic")
	ifaces, err := net.Interfaces()
	if err != nil {
		logrus.Warnf("unable to detect network interfaces")
		return ip
	}
	for _, i := range ifaces {
		if devName == "" || i.Name == devName {
			a := getInterfaceIP(i)
			if a != "" {
				return a
			}
		}
	}

	logrus.Warnf("unable to find interface %s", devName)
	return ip
}

func getInterfaceIP(iface net.Interface) string {
	addrs, err := iface.Addrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		return ip.To4().String()
	}

	return ""
}

func defaultConfig(clix *cli.Context) (*fynca.Config, error) {
	ip := getIP(clix)
	defaultConfig := fynca.DefaultConfig()
	defaultConfig.GRPCAddress = fmt.Sprintf("%s:%d", ip, 8080)
	return defaultConfig, nil
}
