package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/tork/coordinator"
	"github.com/tork/datastore"
	"github.com/tork/mq"
	"github.com/tork/runtime"
	"github.com/tork/worker"
	"github.com/urfave/cli/v2"
)

type mode string

const (
	MODE_STANDALONE  mode = "standalone"
	MODE_COORDINATOR mode = "coordinator"
	MODE_WORKER      mode = "worker"
)

func main() {
	app := &cli.App{
		Name:        "tork",
		Description: "a distributed workflow engine",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "mode",
				Usage:    "standalone|worker|coordinator",
				Required: true,
			},
		},
		Action: func(ctx *cli.Context) error {
			// loggging
			zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

			m := mode(ctx.String("mode"))
			if m != MODE_STANDALONE && m != MODE_WORKER && m != MODE_COORDINATOR {
				return errors.Errorf("invalid mode: %s", m)
			}

			b := mq.NewInMemoryBroker()

			// start the worker
			var w *worker.Worker
			if m == MODE_WORKER || m == MODE_STANDALONE {
				rt, err := runtime.NewDockerRuntime()
				if err != nil {
					return err
				}

				w = worker.NewWorker(worker.Config{
					Broker:  b,
					Runtime: rt,
				})

				if err := w.Start(); err != nil {
					return err
				}
			}

			var c *coordinator.Coordinator
			if m == MODE_COORDINATOR || m == MODE_STANDALONE {
				// create a coordinator
				c = coordinator.NewCoordinator(coordinator.Config{
					Broker:        b,
					TaskDataStore: datastore.NewInMemoryDatastore(),
				})
				// start the coordinator
				if err := c.Start(); err != nil {
					return err
				}
			}

			// wait for the termination signal
			// so we can do a clean shutdown
			quit := make(chan os.Signal, 1)
			signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
			<-quit
			log.Debug().Msg("shutting down")
			if w != nil {
				w.Stop()
			}
			if c != nil {
				c.Stop()
			}

			return nil
		},
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
