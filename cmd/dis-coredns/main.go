package main

import (
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/coremain"
	_ "github.com/coredns/coredns/plugin/bind"
	_ "github.com/coredns/coredns/plugin/errors"
	_ "github.com/han-ry19/DIS-reconstruct/plugin/disredis"
)

func main() { dnsserver.Directives = append(dnsserver.Directives, "disredis"); coremain.Run() }
