package cgi

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Config struct {
	PathInfoStrip int
}

func ReadCGIRequest(cfg *Config) (req *http.Request, res *http.Response, err error) {
	res = &http.Response{}
	req, err = http.ReadRequest(bufio.NewReader(os.Stdin))
	if err != nil {
		return nil, nil, err
	}

	res.Status = "200 OK"
	res.StatusCode = 200
	res.Request = req
	res.Proto = req.Proto
	res.ProtoMajor = req.ProtoMajor
	res.ProtoMinor = req.ProtoMinor
	res.Close = true
	res.Header = http.Header{}

	return
}

func GetCGIVars(cfg *Config, req *http.Request) (res map[string]string, err error) {
	res = map[string]string{}
	// https://www.rfc-editor.org/rfc/rfc3875#section-4
	res["AUTH_TYPE"] = ""
	res["CONTENT_LENGTH"] = strconv.FormatInt(req.ContentLength, 10)
	res["CONTENT_TYPE"] = req.Header.Get("Content-Type")
	res["GATEWAY_INTERFACE"] = "CGI/1.1"
	if cfg.PathInfoStrip >= 0 {
		splits := strings.SplitN(req.URL.Path, "/", cfg.PathInfoStrip+2)
		if len(splits) > cfg.PathInfoStrip+1 {
			res["PATH_INFO"] = "/" + splits[cfg.PathInfoStrip+1]
		} else {
			res["PATH_INFO"] = ""
		}
	} else {
		res["PATH_INFO"] = ""
	}
	res["PATH_TRANSLATED"] = os.Getenv("PATH_INFO") // TODO: find better
	res["QUERY_STRING"] = req.URL.RawQuery
	res["REMOTE_ADDR"] = req.RemoteAddr
	res["REMOTE_HOST"] = ""
	res["REMOTE_IDENT"] = ""
	res["REMOTE_USER"] = ""
	res["REQUEST_METHOD"] = req.Method
	if cfg.PathInfoStrip >= 0 {
		res["SCRIPT_NAME"] = req.URL.Path
		path := ""
		for i, pathcomp := range strings.Split(req.URL.Path, "/") {
			path = path + "/" + pathcomp
			if i >= cfg.PathInfoStrip {
				break
			}
		}
		res["SCRIPT_NAME"] = path
	} else {
		res["SCRIPT_NAME"] = req.URL.Path
	}
	res["SERVER_NAME"] = req.URL.Hostname()
	res["SERVER_PORT"] = req.URL.Port()
	res["SERVER_PROTOCOL"] = "INCLUDED"
	res["SERVER_SOFTWARE"] = "cgi-adapter/1.0"
	return
}

func SetCGIVars(cfg *Config, req *http.Request) error {
	vars, err := GetCGIVars(cfg, req)
	if err != nil {
		return err
	}
	for k, v := range vars {
		err := os.Setenv(k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func ReadCGIResponse(cfg *Config, out io.Reader, res *http.Response) error {
	var err error
	scan := bufio.NewReader(out)
	for {
		linebuf, err := scan.ReadBytes('\n')
		if err != nil {
			return err
		}

		line := strings.TrimRight(string(linebuf), "\r\n")
		if line == "" {
			break
		}

		h := strings.SplitN(line, ":", 2)
		if len(h) == 2 {
			if strings.ToLower(h[0]) == "status" {
				st := strings.Split(h[1], " ")
				if code, e := strconv.ParseInt(st[0], 10, 0); e != nil {
					res.StatusCode = int(code)
					res.Status = h[1]
				}
			} else {
				res.Header.Add(h[0], strings.TrimLeft(h[1], " "))
			}
		}
	}

	content_length := res.Header.Get("Content-Length")
	if content_length != "" {
		res.ContentLength, err = strconv.ParseInt(content_length, 10, 0)
		if err != nil {
			res.ContentLength = -1
			fmt.Fprintf(os.Stderr, "Failed to parse Content-Length %s: %e", content_length, err)
		}
	} else {
		res.ContentLength = -1
	}

	res.Body = io.NopCloser(scan)

	return nil
}

func WriteCGIResponse(cfg *Config, res *http.Response) error {
	return res.Write(os.Stdout)
}

func ExecCGI(cfg *Config, args []string) error {
	req, res, err := ReadCGIRequest(cfg)
	if err != nil {
		return err
	}

	err = SetCGIVars(cfg, req)
	if err != nil {
		return err
	}

	cmd := exec.Command(args[0], args[1:len(args)-1]...)
	cmd.Stdin = req.Body
	cmd.Stderr = os.Stderr

	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	err = ReadCGIResponse(cfg, out, res)
	if err != nil {
		return err
	}

	err = WriteCGIResponse(cfg, res)
	if err != nil {
		return err
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}

	return nil
}
