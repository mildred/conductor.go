package deployment_internal

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/coreos/go-systemd/v22/activation"
	"github.com/coreos/go-systemd/v22/daemon"

	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/status_socket"
	"github.com/mildred/conductor.go/src/tmpl"
	"github.com/mildred/conductor.go/src/trace_connections"
	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/deployment"
)

// Note for deployment hooks, there could be different ways to hook:
//
// - using system commands or script, executed into a specific scope via systemd-run
//
// - using podman exec on a container (except pre-start and post-stop), this is
// already scoped in the podman pod.
//
// - or as a HTTP query to the pod IP address with a retry mechanism

func StartPod(ctx context.Context, depl *Deployment) error {
	reset_log_prefix := utils.AddPrefix(log.Default(), "start pod: ")
	defer reset_log_prefix()

	var saved_listen_pid = os.Getenv("LISTEN_PID")
	var saved_listen_fds = os.Getenv("LISTEN_FDS")
	var saved_listen_fdnames = os.Getenv("LISTEN_FDNAMES")
	if depl.Pod.HasSocketProxy() {
		os.Unsetenv("LISTEN_PID")
		os.Unsetenv("LISTEN_FDS")
		os.Unsetenv("LISTEN_FDNAMES")
	}

	//
	// Start the pod or fail
	//

	log.Printf("Start the deployment pod\n")
	err := KubePlayPod(depl, PodStart, ".")
	if err != nil {
		err = fmt.Errorf("failed to start deployment pod, %v", err)
		SdNotifyOrLog(err.Error())
		return err
	}

	//
	// Find the pod IP address, add it to config
	//

	log.Printf("Looking up pod IP address...\n")
	addr, err := depl.FindPodIPAddress(ctx)
	if err != nil {
		err = fmt.Errorf("failed to find pod IP address, %v", err)
		SdNotifyOrLog(err.Error())
		return err
	}
	log.Printf("Found pod IP address: %s\n", addr)

	depl.Pod.IPAddress = addr

	err = depl.Save(ConfigName)
	if err != nil {
		err = fmt.Errorf("failed to save pod IP address, %v", err)
		SdNotifyOrLog(err.Error())
		return err
	}

	err = RunStartPostHooks(ctx, depl)
	if err != nil {
		return err
	}

	err = StartDeploymentConfig(ctx, depl)
	if err != nil {
		return err
	}

	if depl.Pod.HasSocketProxy() {
		os.Setenv("LISTEN_PID", saved_listen_pid)
		os.Setenv("LISTEN_FDS", saved_listen_fds)
		os.Setenv("LISTEN_FDNAMES", saved_listen_fdnames)

		activation_files := activation.Files(false)
		if len(activation_files) < 1 {
			return fmt.Errorf("Cannot start proxy, no activation file descriptors passed")
		}

		//
		// Start systemd-socket-proxyd if socket activation must be performed through
		// the proxy (the proxy is notifying READY=1)
		//
		// - Do not run proxy in background because then the proxy stopping will not
		//   trigger a deployment service stop
		// - Do not notify MAINPID to proxy because deployment service unit is
		//   ExitType=cgroup
		// - Wait on the proxy to stop then trigger a systemctl stop of the deployment
		//   service unit (let the socket running)
		// - Do not notify READY=1 as the proxy will do the notification for us
		//

		if depl.Pod.SocketProxy.WaitConnect {
			target := depl.Pod.SocketProxy.ExpandedTarget(depl.Vars())
			log.Printf("Wait service to be ready on %s\n", target)

			var timeout time.Duration = 0
			if depl.Pod.SocketProxy.WaitConnectTimeout != "" {
				timeout, err = time.ParseDuration(depl.Pod.SocketProxy.WaitConnectTimeout)
				if err != nil {
					return err
				}
			}

			err = utils.ExponentialSleep(ctx, 500*time.Millisecond, 1*time.Second, timeout, func(elapsed, delay time.Duration) error {
				conn, err := net.DialTimeout(depl.Pod.SocketProxy.TargetProtocol(depl.Vars()), target, 500*time.Millisecond)
				if err == nil {
					conn.Close()
					return utils.STOP
				} else {
					log.Printf("service not yet answering on %s (%v), will retry in %v...", target, err, delay)
					return nil
				}
			})
			if err != nil {
				return err
			}
		}

		log.Printf("Startup sequence completed\n")

		log.Printf("Start the socket activation proxy\n")

		// log.Println(strings.Join(os.Environ(), "\n"))
		// for _, f := range activation.Files(false) {
		// 	log.Printf("activation file: fd=%v", f.Fd())
		// }
		// fmt.Fprintf(os.Stderr, "+ ls -l /proc/self/fd /proc/%d/fd\n", os.Getpid())
		// cmd = exec.CommandContext(ctx, "ls", "-l", "/proc/self/fd", fmt.Sprintf("/proc/%d/fd", os.Getpid()))
		// cmd.Stdout = os.Stdout
		// cmd.Stderr = os.Stderr
		// cmd.ExtraFiles = activation.Files(false)
		// err = cmd.Run()
		// if err != nil {
		// 	log.Printf("systemd-socket-proxyd failed: %v\n", err)
		// 	SdNotifyOrLog(err.Error())
		// 	return err
		// }

		pid_chan := make(chan int, 1)
		defer close(pid_chan)

		requests := make(chan trace_connections.Request, 0)
		defer close(requests)

		ctx1, cancel1 := context.WithCancel(ctx)
		defer cancel1()

		err = trace_connections.StartTrace(ctx1, int(activation_files[0].Fd()), pid_chan, requests)
		if err != nil {
			log.Printf("Cannot enable deployment %s, %v", status_socket.LifeStatusSocketName, err)
		} else {
			log.Printf("Started tracing socket connections and report to %s", status_socket.LifeStatusSocketName)
			err = status_socket.StartDeploymentStatus(ctx1, depl, func() interface{} {
				return trace_connections.AskMetrics(requests)
			})
			if err != nil {
				SdNotifyOrLog(err.Error())
				return err
			}
		}

		args := depl.Pod.SocketProxy.CmdlineArgs(depl.Vars())
		fmt.Fprintf(os.Stderr, "+ systemd-socket-proxyd %s\n", strings.Join(args, " "))
		cmd := exec.CommandContext(ctx, "/bin/sh", append([]string{"-c", "export LISTEN_PID=$$; exec \"$0\" \"$@\"", "/usr/lib/systemd/systemd-socket-proxyd"}, args...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.ExtraFiles = activation_files
		err = cmd.Start()
		if err != nil {
			log.Printf("systemd-socket-proxyd failed: %v\n", err)
			SdNotifyOrLog(err.Error())
			return err
		}
		pid_chan <- cmd.Process.Pid
		err = cmd.Wait()
		if err != nil {
			log.Printf("systemd-socket-proxyd failed: %v\n", err)
			SdNotifyOrLog(err.Error())
			return err
		}

		cancel1()

		// Proxy stopped gracefully, it means that it wants the service to stop
		// waiting for further activation

		log.Printf("systemd-socket-proxyd stopped, triggering a deployment stop\n")

		reset_log_prefix()
		err = StopPodOrDaemon(ctx, depl)
		if err != nil {
			SdNotifyOrLog(err.Error())
			return err
		}

		defer utils.AddPrefix(log.Default(), "stop pod: ")()

		cmd = utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "stop", "--no-block", DeploymentServiceUnit(depl.DeploymentName))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = utils.RunWithJournal(ctx, cmd, DeploymentServiceUnit(depl.DeploymentName))
		if err != nil {
			err = fmt.Errorf("failed to trigger a deployment stop through systemctl, %v", err)
			SdNotifyOrLog(err.Error())
			return err
		}

		return nil

	} else {

		log.Printf("Startup sequence completed\n")

		//
		// Notify that the deployment is ready
		//

		log.Printf("Notify READY=1 and stop main process\n")
		_, err = daemon.SdNotify(false, daemon.SdNotifyReady)
		if err != nil {
			return err
		}

		return nil

	}

}

func StopPodOrDaemon(ctx context.Context, depl *Deployment) error {
	defer utils.AddPrefix(log.Default(), "stop pod: ")()

	//
	// Run the pre-stop hooks (via systemd-run specific scope)
	//
	//    notify the service it should be stopping and should no longer accept
	//    connections or jobs
	//    (command inside the container or HTTP request)
	//
	//    wait for the service to be quiet
	//    (command inside the container or HTTP request)
	//

	log.Printf("executing pre-stop hooks...")
	err := depl.RunHooks(ctx, "pre-stop", depl.PartName, depl.Vars(), 60*time.Second)
	if err != nil {
		log.Printf("pre-stop hooks failed, continuing...")
	}

	//
	// Remove the IP address from the load balancer
	//

	log.Printf(" Removing deployment from load-balancer...\n")
	cmd := utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "stop", DeploymentConfigUnit(depl.DeploymentName))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = utils.RunWithJournal(ctx, cmd, DeploymentConfigUnit(depl.DeploymentName))
	if err != nil {
		log.Printf("ERROR when removing from the load-balancer: %v\n", err)
	}

	if depl.Pod != nil {

		//
		// Stop the containers
		//

		log.Printf("Stopping the containers...\n")
		err = KubePlayPod(depl, PodStop, ".")
		if err != nil {
			err = fmt.Errorf("failed to stop pod, %v", err)
			SdNotifyOrLog(err.Error())
			return err
		}

		//
		// Reset the pod IP address
		//

		depl.Pod.IPAddress = ""
		err = depl.Save(ConfigName)
		if err != nil {
			err = fmt.Errorf("failed to save pod to reset IP address, %v", err)
			SdNotifyOrLog(err.Error())
			return err
		}
	} else if depl.Daemon != nil {

		err = terminateMainProcess(ctx, depl)
		if err != nil {
			SdNotifyOrLog(err.Error())
			return err
		}

	} else {
		return fmt.Errorf("Cannot stop something that is neither a pod or a daemon")
	}

	log.Printf("Stop sequence completed\n")
	return nil
}

func Template(dir string, template string) error {
	depl, err := LoadDeployment(dir, path.Join(dir, ConfigName))
	if err != nil {
		return err
	}

	return tmpl.RunTemplateStdout(context.Background(), template, depl.Vars())
}

type KubePlayOperation = int

const (
	PodCreate KubePlayOperation = iota
	PodStart
	PodStop
)

func KubePlayPod(depl *Deployment, op KubePlayOperation, dir string) error {

	err := os.WriteFile(path.Join(dir, "pod.yml"), []byte(depl.TemplatedPod), 0644)
	if err != nil {
		return err
	}

	var args []string

	if op == PodStop {
		args = utils.Compact("kube", "down")
	} else {
		args = utils.Compact("kube", "play")
	}

	if op == PodStart {
		args = append(args, "--start", "--replace")
	} else if op == PodCreate {
		args = append(args, "--start=false", "--replace")
	}

	if op != PodStop && depl.TemplatedConfigMap != "" {
		err := os.WriteFile(path.Join(dir, "configmap.yml"), []byte(depl.TemplatedConfigMap), 0644)
		if err != nil {
			return err
		}
		args = append(args, "--configmap="+path.Join(dir, "configmap.yml"))
	}

	if op == PodStart || op == PodCreate {
		args = append(args, utils.Compact(
			"--annotation="+fmt.Sprintf("conductor_deployment=%s", depl.DeploymentName),
			"--annotation="+fmt.Sprintf("conductor_instance=%s", depl.InstanceName),
			"--annotation="+fmt.Sprintf("conductor_app=%s", depl.AppName),
			"--log-driver=journald")...)
	}

	args = append(args, path.Join(dir, "pod.yml"))

	fmt.Fprintf(os.Stderr, "+ podman %q\n", args)
	cmd := exec.Command("podman", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
