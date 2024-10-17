package deployment

// Note for deployment hooks, there could be different ways to hook:
//
// - using system commands or script, executed into a specific scope via systemd-run
//
// - using podman exec on a container (except pre-start and post-stop), this is
// already scoped in the podman pod.
//
// - or as a HTTP query to the pod IP address with a retry mechanism

func Prepare() error {
	// create the deployment config by taking the current service config
	//
	// template the pod and add it to the config
	//
	// run the pre-start hooks (via systemd-run specific scope): nothing special to do, just in case
	return nil
}

func Start() error {
	// start the pod or fail
	//
	// find pod ip address, add it to the deployment config
	//
	// run the post-start hook (via systemd-run specific scope)
	//
	//     this can generate prometheus config for monitoring
	//     (generic systemd-run command or script)
	//
	//     a generic script can wait for the pod to be ready
	//     (better as a command inside a container or HTTP request)
	//
	//     this can also perform data migrations
	//
	// notify load balancer of ip address, hook into existing load balancer
	// configuration for the service and add the IP address
	//
	// notify that the deployment is ready
	return nil
}

func Stop() error {
	// notify the deployment is stopping
	//
	// run the pre-stop hooks (via systemd-run specific scope)
	//
	//    notify the service it should be stopping and should no longer accept
	//    connections or jobs
	//    (command inside the container or HTTP request)
	//
	//    wait for the service to be quiet
	//    (command inside the container or HTTP request)
	//
	// remove the IP address from the load balancer
	//
	// Stop the containers
	return nil
}

func Cleanup() error {
	// run the post-stop hooks (via systemd-run specific scope), just in case
	//
	// remove deployment files
	return nil
}
