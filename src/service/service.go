package service

type Service struct {
	Name                string            `json:"app_name"`              // my-app
	InstanceName        string            `json:"instance_name"`         // staging
	Config              map[string]string `json:"config"`                // key-value pairs for config and templating, CHANNEL=staging
	PodTemplate         string            `json:"pod_template"`          // Template file for pod
	ProxyConfigTemplate string            `json:"proxy_config_template"` // Template file for the load-balancer config
}

// Note for services: a linked proxy config is set up when the service itself is
// enabled. The service depends on this proxy config

func StartOrRestart() error {
	// [ if restarting ] notify systemd reload in progress
	//
	// if there is a deployment starting or started with the identical config,
	// use it and wait for it to be started, else:
	//
	//     find a free deployment Name
	//
	//     copy all the current service config over to the deployment, it should
	//     appear in the list
	//
	//     start the deployment, wait for started
	//
	// [ if restarting ] if failing to start, exit uncleanly
	// [ if starting ] if failing, stop all deployments and exit uncleanly
	//
	// notify systemd ready
	//
	// stop all deployments that are of older config version
	//
	// keep running in the background and monitor that the deployments are still
	// running, else exit with an error code
	return nil
}

func Stop() error {
	// notify stop in progress
	//
	// stop all deployments
	return nil
}

func Cleanup() error {
	// remove temp files if there is any
	return nil
}

func Reload() error {
	// Loop through location and set up services
	return nil
}
