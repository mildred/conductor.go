package deployment_internal

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/mildred/conductor.go/src/cgi"

	. "github.com/mildred/conductor.go/src/deployment"
)

func StartFunction(ctx context.Context, depl *Deployment) error {
	var err error
	switch depl.Function.Format {
	case "cgi":
		// Nothing to start, this is started on demand
		// err = StartCGIFunction(ctx, depl, depl.Function)
		if err != nil {
			return fmt.Errorf("while starting CGI function, %v", err)
		}
	case "http-stdio":
		err = StartHttpStdioFunction(ctx, depl, depl.Function)
		if err != nil {
			return fmt.Errorf("while starting HTTP stdio function, %v", err)
		}
	default:
		err = fmt.Errorf("Unknown CGI function format %s", depl.Function.Format)
	}
	return err
}

func StartHttpStdioFunction(ctx context.Context, depl *Deployment, f *DeploymentFunction) error {
	return ExecuteDecodedFunction(ctx, depl, f, os.Stdin, nil)
}

func StartCGIFunction(ctx context.Context, depl *Deployment, f *DeploymentFunction) error {
	cfg := &cgi.Config{
		PathInfoStrip: f.PathInfoStrip,
	}

	req, res, err := cgi.ReadCGIRequest(cfg)
	if err != nil {
		return fmt.Errorf("while reading CGI request, %v", err)
	}

	err = cgi.SetCGIVars(cfg, req)
	if err != nil {
		return fmt.Errorf("while setting CGI variables, %v", err)
	}

	err = ExecuteDecodedFunction(ctx, depl, f, req.Body, func(out io.ReadCloser) error {
		err = cgi.ReadCGIResponse(cfg, out, res)
		if err != nil {
			return fmt.Errorf("while reading CGI response, %v", err)
		}

		err = cgi.WriteCGIResponse(cfg, res)
		if err != nil {
			return fmt.Errorf("while writing CGI response, %v", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func ExecuteDecodedFunction(ctx context.Context, depl *Deployment, f *DeploymentFunction, stdin io.Reader, handle_stdout func(io.ReadCloser) error) error {
	var err error
	if len(f.Exec) < 1 {
		return fmt.Errorf("Missing executable")
	}

	for _, resp_header := range f.ResponseHeaders {
		fmt.Fprintf(os.Stdout, "%s\r\n", resp_header)
	}

	if f.NoResponseHeaders {
		fmt.Fprintf(os.Stdout, "\r\n")
	}

	cmd := exec.CommandContext(ctx, f.Exec[0], f.Exec[1:]...)
	cmd.Env = append(cmd.Environ(), depl.Vars()...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	var stdout_pipe io.ReadCloser
	if handle_stdout != nil {
		stdout_pipe, err = cmd.StdoutPipe()
		if err != nil {
			return err
		}
	}

	if f.StderrAsStdout {
		cmd.Stderr = cmd.Stdout
	}

	if handle_stdout != nil {
		err = cmd.Start()
		if err != nil {
			return err
		}

		err = handle_stdout(stdout_pipe)
		if err != nil {
			return err
		}

		err = cmd.Wait()
		if err != nil {
			return err
		}
	} else {
		return cmd.Run()
	}
	return nil
}
