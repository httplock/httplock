package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/sudo-bmitch/reproducible-proxy/api"
	"github.com/sudo-bmitch/reproducible-proxy/config"
	"github.com/sudo-bmitch/reproducible-proxy/proxy"
	"github.com/sudo-bmitch/reproducible-proxy/storage"
)

var serverOpts struct {
	addrAPI   string
	addrProxy string
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the proxy server",
	Args:  cobra.ExactArgs(0),
	RunE:  runServer,
}

func init() {
	serverCmd.PersistentFlags().StringVarP(
		&serverOpts.addrAPI,
		"addr-api", "", "",
		"API listener address",
	)
	serverCmd.PersistentFlags().StringVarP(
		&serverOpts.addrProxy,
		"addr-proxy", "", "",
		"Proxy listener address",
	)

	rootCmd.AddCommand(serverCmd)
}

func runServer(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// parse args, env, load config
	conf := config.New(config.ConfigOpts{
		AddrAPI:   serverOpts.addrAPI,
		AddrProxy: serverOpts.addrProxy,
		ConfFile:  rootOpts.confFile,
		Log:       log,
	})

	// create storage object
	s, err := storage.New(conf)
	if err != nil {
		return err
	}

	// launch proxy in goroutine
	proxySvc, err := proxy.Start(conf, s)
	if err != nil {
		return err
	}

	// launch api service
	apiSvc, err := api.Start(conf, s)
	if err != nil {
		return err
	}

	// monitor signals to handle shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	proxySvc.Shutdown(ctx)
	apiSvc.Shutdown(ctx)
	return nil
}
