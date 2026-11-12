package service_util

import (
	"context"

	"github.com/mildred/conductor.go/src/addr"

	. "github.com/mildred/conductor.go/src/service"
)

func BringAddrUp(ctx context.Context, service *Service) error {
	addrNames, err := service.AddrNames()
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
			err = a.Up(ctx, addrName)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
