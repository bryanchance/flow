package main

import (
	"fmt"
	"net"
	"os"

	"git.underland.io/ehazlett/finca"
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

func defaultConfig(clix *cli.Context) (*finca.Config, error) {
	ip := getIP(clix)
	defaultConfig := finca.DefaultConfig()
	defaultConfig.GRPCAddress = fmt.Sprintf("%s:%d", ip, 8080)
	return defaultConfig, nil
}
