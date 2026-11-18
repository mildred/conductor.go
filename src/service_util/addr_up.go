package service_util

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/mildred/conductor.go/src/addr"

	. "github.com/mildred/conductor.go/src/service"
)

func BringAddrUp(ctx context.Context, service *Service) error {
	prefix := log.Prefix()
	defer log.SetPrefix(prefix)

	addrNames, err := service.AddrNames("")
	if err != nil {
		return err
	}
	if len(addrNames) > 0 {
		a, err := addr.OpenDefaultAddr()
		if err != nil {
			return err
		}
		defer a.Close()

		for _, addrName := range addrNames {
			log.SetPrefix(fmt.Sprintf("%s[up %s] ", prefix, addrName))
			err = a.Up(ctx, addrName, io.Discard, io.Discard)
			log.SetPrefix(prefix)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func BringAddrDown(ctx context.Context, service *Service) error {
	prefix := log.Prefix()
	defer log.SetPrefix(prefix)

	addrNames, err := service.AddrNames("")
	if err != nil {
		return err
	}
	if len(addrNames) > 0 {
		a, err := addr.OpenDefaultAddr()
		if err != nil {
			return err
		}
		defer a.Close()

		for _, addrName := range addrNames {
			log.SetPrefix(fmt.Sprintf("%s[down %s] ", prefix, addrName))
			err = a.Down(ctx, addrName, io.Discard, io.Discard)
			log.SetPrefix(prefix)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
