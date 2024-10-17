package install

import (
	"os"
	"path"
)

func Install(destdir string) error {
	err := os.MkdirAll(path.Dir(destdir+ConductorCGIFunctionServiceLocation), 0755)
	if err != nil {
		return err
	}

	err = os.WriteFile(destdir+ConductorCGIFunctionServiceLocation, []byte(ConductorCGIFunctionService), 0644)
	if err != nil {
		return err
	}

	return nil
}

func Uninstall(destdir string) error {
	err := os.Remove(destdir + ConductorCGIFunctionServiceLocation)
	if err != nil {
		return err
	}

	return nil
}
