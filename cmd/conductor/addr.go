package main

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/integrii/flaggy"

	"github.com/mildred/conductor.go/src/addr"
	"github.com/mildred/conductor.go/src/config"
)

func cmd_addr_down() *flaggy.Subcommand {
	var hostname string

	cmd := flaggy.NewSubcommand("down")
	cmd.Description = "Bring an address down"
	cmd.AddPositionalValue(&hostname, "hostname", 1, true, "The hostname to bring down")

	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		cfg, err := config.ReadConfig()
		if err != nil {
			return err
		}

		var a = addr.NewAddr(cfg)
		err = a.Init()
		if err != nil {
			return err
		}

		defer a.Close()

		return a.Down(context.Background(), hostname)
	})
	return cmd
}

func cmd_addr_up() *flaggy.Subcommand {
	var hostname string

	cmd := flaggy.NewSubcommand("up")
	cmd.Description = "Bring an address up"
	cmd.AddPositionalValue(&hostname, "hostname", 1, true, "The hostname to bring up")

	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		cfg, err := config.ReadConfig()
		if err != nil {
			return err
		}

		var a = addr.NewAddr(cfg)
		err = a.Init()
		if err != nil {
			return err
		}

		defer a.Close()

		return a.Up(context.Background(), hostname)
	})
	return cmd
}

func cmd_addr_update() *flaggy.Subcommand {
	cmd := flaggy.NewSubcommand("update")
	cmd.Description = "Update /etc/hosts"

	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		cfg, err := config.ReadConfig()
		if err != nil {
			return err
		}

		var a = addr.NewAddr(cfg)
		err = a.Init()
		if err != nil {
			return err
		}

		defer a.Close()

		return a.Update()
	})
	return cmd
}

func cmd_addr_ula_prefix() *flaggy.Subcommand {
	var generate, no_cidr bool
	var size int

	cmd := flaggy.NewSubcommand("ula-prefix")
	cmd.Bool(&generate, "g", "generate", "Show a random ULA prefix instead")
	cmd.Int(&size, "", "size", "Size of the generated ULA prefix")
	cmd.Bool(&no_cidr, "n", "no-cidr", "Hide the CIDR")
	cmd.Description = "Show the system ULA prefix"

	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		if generate {
			if size == 0 {
				size = 64
			}
			fmt.Println(config.GenerateRandomULAPrefix(size, !no_cidr))
		} else {
			cfg, err := config.ReadConfig()
			if err != nil {
				return err
			}

			var a = addr.NewAddr(cfg)
			err = a.Init()
			if err != nil {
				return err
			}

			defer a.Close()

			if no_cidr {
				fmt.Println(a.ULAPrefixNoCidr())
			} else {
				fmt.Println(a.ULAPrefix)
			}
		}

		return nil
	})
	return cmd
}

func cmd_addr() *flaggy.Subcommand {
	cmd := flaggy.NewSubcommand("addr")
	cmd.Description = "Address subcommand"
	cmd.AttachSubcommand(cmd_addr_update(), 1)
	cmd.AttachSubcommand(cmd_addr_up(), 1)
	cmd.AttachSubcommand(cmd_addr_down(), 1)
	cmd.AttachSubcommand(cmd_addr_ula_prefix(), 1)
	cmd.RequireSubcommand = true
	return cmd
}
