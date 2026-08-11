// Command web is the storefront web app's entry point. Mirrors cmd/web/main.go
// in the source repo: load config, initialize the selected shared clients
// via cfg.InitSelected, then hand off to the composition root
// (internal/infrastructure/api/web.Start) for everything else — router,
// middleware, every vertical slice's handler, and the HTTP server itself.
// Unlike the source repo's run, there's no PayPhone/PayPal strategy
// selection or application struct assembly here: that's all resolved
// inside web.Start now (PayPal isn't ported at all — spec assumption #5).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/v8tix/beecore-eda/config"
	"github.com/v8tix/jsonx"

	web "github.com/v8tix/beecore-store-v2/internal/infrastructure/api/web"
)

func main() {
	var cfgDirFlag string
	var cfgFileFlag string

	flag.StringVar(&cfgDirFlag, "d", "/Users/vrock/Public/Common/infra/Deilora", "The configuration directory")
	flag.StringVar(&cfgFileFlag, "f", "config", "The configuration file")
	flag.Parse()
	cfgFile := fmt.Sprintf("%s/%s", cfgDirFlag, cfgFileFlag)

	if err := run(cfgFile); err != nil {
		panic(err)
	}
}

func run(file string) error {
	cfg := load(file)

	servicesToInit := []config.Service{
		config.ServiceLogger,
		config.ServiceJWT,
		config.ServiceHTTPClient,
		config.ServiceRedis,
		config.ServiceWorker,
		config.ServiceCloudStorage,
		config.ServiceSessionStore,
		config.ServiceKMS,
		config.ServicePostgres,
		config.ServiceAuth,
	}

	if err := cfg.InitSelected(context.Background(), servicesToInit); err != nil {
		log.Fatal(err)
	}
	defer cfg.CloseAllClients()

	return web.Start(cfg)
}

func load(file string) *config.Cfg {
	data, err := os.ReadFile(file)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	cfg, err := jsonx.Decoder[*config.Cfg]().FromBytes(data)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return cfg
}
