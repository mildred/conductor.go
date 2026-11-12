package apply

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/mildred/conductor.go/src/service"
	"github.com/mildred/conductor.go/src/service_public"
	"github.com/mildred/conductor.go/src/utils"
)

type ApplyOpts struct {
	BasePath    string
	absBasePath string
	ShowDiff    bool
	Interactive bool
	NoReload    bool
}

func Apply(ctx context.Context, opts ApplyOpts) error {
	var err error

	if opts.BasePath == "" {
		opts.BasePath = "."
	}

	opts.absBasePath, err = filepath.Abs(opts.BasePath)
	if err != nil {
		return err
	}

	files, err := os.ReadDir(opts.BasePath)
	if err != nil {
		return err
	}

	for _, f := range files {
		err := applyFile(ctx, opts, f)
		if err != nil {
			return err
		}
	}

	if !opts.NoReload {
		err := service_public.ReloadServices(true, true)
		if err != nil {
			return err
		}
	}

	return nil
}

func applyFile(ctx context.Context, opts ApplyOpts, f os.DirEntry) error {
	defer utils.AddPrefix(log.Default(), fmt.Sprintf("apply %s: ", f.Name()))()

	if f.IsDir() && strings.HasSuffix(f.Name(), ".service") {
		return applyServiceDir(ctx, opts, f)
	} else {
		log.Printf("ignoring (unknown type)")
		return nil
	}
}

func applyServiceDir(ctx context.Context, opts ApplyOpts, f os.DirEntry) error {
	service_name := strings.TrimSuffix(f.Name(), ".service")
	dest_path := path.Join(service.ServiceDirHome, service_name)
	cur_path := path.Join(opts.absBasePath, f.Name())

	log.Printf("overwrite service %s", dest_path)

	err := os.MkdirAll(service.ServiceDirHome, 0755)
	if err != nil {
		return err
	}

	if opts.ShowDiff {
		os.Stdout.Sync()
		os.Stderr.Sync()

		log.Println()
		cmd := exec.CommandContext(ctx, "diff", "-urN", dest_path, cur_path)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		err := cmd.Run()
		if err != nil {
			e := err.(*exec.ExitError)
			if e == nil {
				return err
			}
		} else if err == nil {
			log.Printf("no changes detected")
			return nil
		}
	}

	if opts.Interactive {
		res, err := utils.CliQuestionString(fmt.Sprintf("\n\nApply changes to %s? (type \"yes\" to approve)", dest_path))
		if err != nil {
			return err
		}
		if res != "yes" {
			return fmt.Errorf("Operation aborted by user")
		}
	}

	cmd := exec.CommandContext(ctx, "rsync", "--del", "-a", cur_path+"/", dest_path+"/")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	err = cmd.Run()
	if err != nil {
		return err
	}

	log.Printf("service updated")

	return nil
}
