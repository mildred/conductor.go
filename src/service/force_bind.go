package service

import (
	"os"
)

type ServiceDaemonNoForceBind struct {
}

type ServiceDaemonForceBind struct {
	ForceBind *ForceBind `json:"force_bind,omitempty"`
}

type ForceBind struct {
	Permissive bool              `json:"permissive"`
	Rules      map[string]string `json:"rules"`
}

func (p *ServiceDaemonNoForceBind) ForceBindEnabled() bool {
	return false
}

func (p *ServiceDaemonNoForceBind) ForceBindFlags() []string {
	return nil
}

func (p *ServiceDaemonForceBind) ForceBindEnabled() bool {
	if p.ForceBind == nil {
		return false
	}

	if p.ForceBind.Rules != nil && len(p.ForceBind.Rules) > 0 {
		return true
	}

	if !p.ForceBind.Permissive {
		return true
	}

	return false
}

func (p *ServiceDaemonForceBind) ForceBindFlags() (flags []string) {
	if p.ForceBind.Permissive {
		flags = append(flags, "--permissive")
	}
	if p.ForceBind.Rules != nil {
		for k, v := range p.ForceBind.Rules {
			flags = append(flags, "--bind", k+"="+v)
		}
	}
	return flags
}

func WrapForceBind(flags []string, cmd ...string) ([]string, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}

	return append(append([]string{self, "force-bind"}, flags...), cmd...), nil
}
