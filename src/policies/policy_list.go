package policies

import (
	"fmt"
	"os"
	"path"
)

func policy_migrate_to_dir(dir string, ent os.DirEntry) error {
	if ent.IsDir() {
		return nil
	}

	name := ent.Name()
	err := os.Rename(path.Join(dir, name), path.Join(dir, name+".json"))
	if err != nil {
		return err
	}

	err = os.MkdirAll(path.Join(dir, name), 0777)
	if err != nil {
		return err
	}

	err = os.Rename(path.Join(dir, name+".json"), path.Join(dir, name, ConfigName))
	if err != nil {
		return err
	}

	return nil
}

func PolicyList() ([]string, error) {
	var policy_dirs []string

	for _, dir := range PolicyDirs {
		entries, err := os.ReadDir(dir)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}

		for _, ent := range entries {
			err := policy_migrate_to_dir(dir, ent)
			if err != nil {
				return nil, fmt.Errorf("Failed to migrate policy %s/%s", dir, ent.Name())
			}

			policy_dir := path.Join(dir, ent.Name())
			_, err = os.Stat(path.Join(policy_dir, ConfigName))
			if err != nil && !os.IsNotExist(err) {
				return nil, err
			} else if err != nil {
				// ignore error, this is not a valid service dir
				continue
			}

			policy_dir, err = PolicyRealpath(policy_dir)
			if err != nil {
				return nil, err
			}

			policy_dirs = append(policy_dirs, policy_dir)
		}
	}

	return policy_dirs, nil
}
