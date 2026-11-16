package apply

import (
	"context"
	"fmt"
	"io"
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
	BasePath          string
	absBasePath       string
	ShowDiff          bool
	Interactive       bool
	NoReload          bool
	NoRestart         bool
	servicesToRestart []string
	servicesToReload  []string
}

func Apply(ctx context.Context, opts ApplyOpts) error {
	var err error

	log_prefix := log.Prefix()
	defer func() {
		log.SetPrefix(log_prefix)
	}()

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
		err := applyFile(ctx, &opts, f)
		if err != nil {
			return err
		}
	}

	if !opts.NoReload {
		if len(opts.servicesToReload) > 0 {
			log.Printf("\nServices to reload: %v\n\n", opts.servicesToReload)
		} else {
			log.Printf("\nNo services to reload\n\n")
		}

		err := service_public.ReloadServices(ctx, service_public.ReloadServiceOpts{
			Inclusive:          true,
			Verbose:            true,
			ReloadServiceNames: opts.servicesToReload,
		})
		if err != nil {
			return err
		}
	}

	if !opts.NoRestart {
		if len(opts.servicesToRestart) > 0 {
			log.Println()
			log.Printf("Services to restart: %v", opts.servicesToRestart)
		}

		for _, service_name := range opts.servicesToRestart {
			log.Println()
			log.Printf("restart %s", service_name)
			log.SetPrefix(fmt.Sprintf("%restart %s: ", log_prefix, service_name))
			err := service_public.Restart(ctx, service_name, service_public.RestartOpts{})
			if err != nil {
				return err
			}
			log.SetPrefix(log_prefix)
		}
	}

	return nil
}

func applyFile(ctx context.Context, opts *ApplyOpts, f os.DirEntry) error {
	defer utils.AddPrefix(log.Default(), fmt.Sprintf("apply %s: ", f.Name()))()

	if f.IsDir() && strings.HasSuffix(f.Name(), ".service") {
		return applyServiceDir(ctx, opts, f)
	} else {
		log.Printf("ignoring (unknown type)")
		return nil
	}
}

func applyServiceDir(ctx context.Context, opts *ApplyOpts, f os.DirEntry) error {
	service_name := strings.TrimSuffix(f.Name(), ".service")
	dest_path := path.Join(service.ServiceDirHome, service_name)
	cur_path := path.Join(opts.absBasePath, f.Name())

	log.Printf("overwrite service %s", dest_path)

	err := os.MkdirAll(service.ServiceDirHome, 0755)
	if err != nil {
		return err
	}

	os.Stdout.Sync()
	os.Stderr.Sync()

	log.Println()
	cmd := exec.CommandContext(ctx, "diff", "-urN", dest_path, cur_path)
	if opts.ShowDiff {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	}
	cmd.Stdin = os.Stdin
	err = cmd.Run()
	if err != nil {
		e := err.(*exec.ExitError)
		if e == nil {
			return err
		}
	} else if err == nil {
		log.Printf("no changes detected")
		return nil
	}

	if opts.Interactive {
		res, err := utils.CliQuestionString(fmt.Sprintf("\n\nApply changes to %s? (type \"yes\" to approve)", dest_path))
		fmt.Println()
		if err != nil {
			return err
		}
		if res != "yes" {
			return fmt.Errorf("Operation aborted by user")
		}
	}

	cmd = exec.CommandContext(ctx, "rsync", "--del", "-a", cur_path+"/", dest_path+"/")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	err = cmd.Run()
	if err != nil {
		return err
	}

	log.Printf("service updated")

	opts.servicesToReload = append(opts.servicesToReload, service_name)

	return nil
}
