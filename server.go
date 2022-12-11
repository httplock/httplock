package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/httplock/httplock/internal/api"
	"github.com/httplock/httplock/internal/cert"
	"github.com/httplock/httplock/internal/config"
	"github.com/httplock/httplock/internal/proxy"
	"github.com/httplock/httplock/internal/storage"
	"github.com/spf13/cobra"
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
	conf, err := config.New(config.ConfigOpts{
		AddrAPI:   serverOpts.addrAPI,
		AddrProxy: serverOpts.addrProxy,
		ConfFile:  rootOpts.confFile,
		Log:       log,
	})
	if err != nil {
		return err
	}

	// create storage object
	s, err := storage.Get(conf)
	if err != nil {
		return err
	}

	// setup cert generation
	c := cert.NewCert()
	// TODO: load CA from config if provided
	err = c.CAGen("Reproducible Proxy CA")
	if err != nil {
		return err
	}

	// launch proxy in goroutine
	proxySvc, err := proxy.Start(conf, s, c)
	if err != nil {
		return err
	}

	// launch api service
	apiSvc, err := api.Start(conf, s, c)
	if err != nil {
		return err
	}

	// monitor signals to handle shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	ctxShutdown, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	proxySvc.Shutdown(ctxShutdown)
	apiSvc.Shutdown(ctxShutdown)

	// update index files
	err = s.Flush()
	if err != nil {
		return err
	}

	return nil
}
