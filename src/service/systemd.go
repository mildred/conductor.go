package service

import (
	"context"

	"github.com/coreos/go-systemd/v22/dbus"

	"github.com/mildred/conductor.go/src/utils"
)

func (service *Service) UnitStatus(ctx context.Context) (dbus.UnitStatus, error) {
	sd, err := utils.NewSystemdClient(ctx)
	if err != nil {
		return dbus.UnitStatus{}, err
	}

	list, err := sd.ListUnitsByNamesContext(ctx, []string{ServiceUnit(service.BasePath)})
	if err != nil {
		return dbus.UnitStatus{}, err
	}

	for _, item := range list {
		if item.Name == ServiceUnit(service.BasePath) {
			return item, nil
		}
	}

	return dbus.UnitStatus{}, nil
}
