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

	"github.com/mildred/conductor.go/src/tmpl"

	. "github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/utils"
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
	err := depl.StartStopPod(true, ".")
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

	//
	// Run the post-start hook (via systemd-run specific scope)
	//
	//     this can generate prometheus config for monitoring
	//     (generic systemd-run command or script)
	//
	//     a generic script can wait for the pod to be ready
	//     (better as a command inside a container or HTTP request)
	//
	//     this can also perform data migrations
	//

	_, err = daemon.SdNotify(false, "EXTEND_TIMEOUT_USEC=60000000") // 60s
	if err != nil {
		return err
	}

	log.Printf("executing post-start hooks...")
	err = depl.RunHooks(ctx, "post-start", depl.PartName, depl.Vars(), 60*time.Second)
	if err != nil {
		log.Printf("post-start hooks failed, continuing...")
	}

	//
	// Add IP address to load balancer
	//
	// notify load balancer of ip address, hook into existing load balancer
	// configuration for the service and add the IP address
	//

	log.Printf("Adding deployment to load-balancer...\n")
	cmd := utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "start", DeploymentConfigUnit(depl.DeploymentName))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = utils.RunWithJournal(ctx, cmd, DeploymentConfigUnit(depl.DeploymentName))
	if err != nil {
		err = fmt.Errorf("failed to add deployment to load-balancer, %v", err)
		SdNotifyOrLog(err.Error())
		return err
	}

	if depl.Pod.HasSocketProxy() {
		os.Setenv("LISTEN_PID", saved_listen_pid)
		os.Setenv("LISTEN_FDS", saved_listen_fds)
		os.Setenv("LISTEN_FDNAMES", saved_listen_fdnames)

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

		args := depl.Pod.SocketProxy.CmdlineArgs(depl.Vars())
		fmt.Fprintf(os.Stderr, "+ systemd-socket-proxyd %s\n", strings.Join(args, " "))
		cmd = exec.CommandContext(ctx, "/bin/sh", append([]string{"-c", "export LISTEN_PID=$$; exec \"$0\" \"$@\"", "/usr/lib/systemd/systemd-socket-proxyd"}, args...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.ExtraFiles = activation.Files(false)
		err = cmd.Run()
		if err != nil {
			log.Printf("systemd-socket-proxyd failed: %v\n", err)
			SdNotifyOrLog(err.Error())
			return err
		}

		// Proxy stopped gracefully, it means that it wants the service to stop
		// waiting for further activation

		log.Printf("systemd-socket-proxyd stopped, triggering a deployment stop\n")

		reset_log_prefix()
		err = StopPod(ctx, depl)
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

func StopPod(ctx context.Context, depl *Deployment) error {
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

	//
	// Stop the containers
	//

	log.Printf("Stopping the containers...\n")
	err = depl.StartStopPod(false, ".")
	if err != nil {
		err = fmt.Errorf("failed to stop pod, %v", err)
		SdNotifyOrLog(err.Error())
		return err
	}

	log.Printf("Stop sequence completed\n")
	return nil
}

func Template(dir string, template string) error {
	depl, err := LoadDeployment(path.Join(dir, ConfigName))
	if err != nil {
		return err
	}

	return tmpl.RunTemplateStdout(context.Background(), template, depl.Vars())
}
