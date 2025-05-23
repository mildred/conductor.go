package utils

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/rodaine/table"

	"github.com/mildred/conductor.go/src/dirs"
)

func ExtendTimeout(ctx context.Context, extra_time time.Duration) {
	const loop_duration = 30 * time.Second
	for ctx.Err() == nil {
		extend := loop_duration + extra_time
		_, err := daemon.SdNotify(false, fmt.Sprintf("EXTEND_TIMEOUT_USEC=%d", extend.Microseconds()))
		if err != nil {
			log.Printf("Error extending systemd timeout: %s", err.Error())
		}

		ctx1, cancel := context.WithTimeout(ctx, loop_duration)
		<-ctx1.Done()
		cancel()
	}
}

func NewSystemdClient(ctx context.Context) (*dbus.Conn, error) {
	if dirs.AsRoot {
		return dbus.NewSystemConnectionContext(ctx)
	} else {
		return dbus.NewUserConnectionContext(ctx)
	}
}

type UnitStatusSpec struct {
	Name    string
	Pattern string
	regex   *regexp.Regexp
	Units   []dbus.UnitStatus
}

type UnitStatusSpecs []*UnitStatusSpec

func UnitsStatus(ctx context.Context, sd *dbus.Conn, specs UnitStatusSpecs) ([]dbus.UnitStatus, error) {
	patterns := []string{}
	for _, spec := range specs {
		patterns = append(patterns, spec.Pattern)
		if spec.regex == nil {
			spec.regex = regexp.MustCompile("^" + strings.ReplaceAll(regexp.QuoteMeta(spec.Pattern), "\\*", ".*") + "$")
		}
	}

	units, err := sd.ListUnitsByPatternsContext(ctx, nil, patterns)
	if err != nil {
		return nil, err
	}

	unassigned := []dbus.UnitStatus{}
	for _, u := range units {
		assigned := false
		for _, spec := range specs {
			if spec.regex.MatchString(u.Name) {
				spec.Units = append(spec.Units, u)
				assigned = true
			}
		}
		if !assigned {
			unassigned = append(unassigned, u)
		}
	}

	return unassigned, nil
}

func (specs UnitStatusSpecs) ToTable() table.Table {
	tbl := table.New("", "Unit", "Loaded", "Active", "")
	for _, unit_spec := range specs {
		for _, u := range unit_spec.Units {
			tbl.AddRow(unit_spec.Name, u.Name, u.LoadState, u.ActiveState, "("+u.SubState+")")
		}
		if len(unit_spec.Units) == 0 {
			tbl.AddRow(unit_spec.Name, unit_spec.Pattern, "", "", "")
		}
	}
	return tbl
}
