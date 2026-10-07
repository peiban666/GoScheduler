// Command goscheduler-node
package main

import (
	"flag"
	"os"
	"runtime"
	"strings"

	"github.com/gaggad/goscheduler/internal/modules/rpc/auth"
	"github.com/gaggad/goscheduler/internal/modules/rpc/server"
	"github.com/gaggad/goscheduler/internal/modules/utils"
	"github.com/gaggad/goscheduler/internal/util"
	log "github.com/sirupsen/logrus"
)

var (
	AppVersion           = "1.6"
	BuildDate, GitCommit string
)

func main() {
	var serverAddr string
	var allowRoot bool
	var version bool
	var CAFile string
	var certFile string
	var keyFile string
	var enableTLS bool
	var logLevel string
	flag.BoolVar(&allowRoot, "allow-root", false, "./goscheduler-node -allow-root")
	flag.StringVar(&serverAddr, "s", "0.0.0.0:5921", "./goscheduler-node -s ip:port")
	flag.BoolVar(&version, "v", false, "./goscheduler-node -v")
	flag.BoolVar(&enableTLS, "enable-tls", false, "./goscheduler-node -enable-tls")
	flag.StringVar(&CAFile, "ca-file", "", "./goscheduler-node -ca-file path")
	flag.StringVar(&certFile, "cert-file", "", "./goscheduler-node -cert-file path")
	flag.StringVar(&keyFile, "key-file", "", "./goscheduler-node -key-file path")
	flag.StringVar(&logLevel, "log-level", "info", "-log-level error")
	flag.Parse()
	level, err := log.ParseLevel(logLevel)
	if err != nil {
		log.Fatal(err)
	}
	log.SetLevel(level)

	if version {
		util.PrintAppVersion(AppVersion, GitCommit, BuildDate)
		return
	}

	if enableTLS {
		if !utils.FileExist(CAFile) {
			log.Fatalf("failed to read ca cert file: %s", CAFile)
		}
		if !utils.FileExist(certFile) {
			log.Fatalf("failed to read server cert file: %s", certFile)
			return
		}
		if !utils.FileExist(keyFile) {
			log.Fatalf("failed to read server key file: %s", keyFile)
			return
		}
	}

	certificate := auth.Certificate{
		CAFile:   strings.TrimSpace(CAFile),
		CertFile: strings.TrimSpace(certFile),
		KeyFile:  strings.TrimSpace(keyFile),
	}

	if runtime.GOOS != "windows" && os.Getuid() == 0 && !allowRoot {
		log.Fatal("Do not run goscheduler-node as root user")
		return
	}

	server.Start(serverAddr, enableTLS, certificate)
}
