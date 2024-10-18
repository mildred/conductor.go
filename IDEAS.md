conductor daemon-reload
-----------------------

Reload all services in well-known directories

conductor service declare DIR [VAR=VAL...]
------------------------------------------

Declare a new service using the data in the provided DIR.
The files in DIR are templated using the provided variables
This configuration declares:

- a service name (unique)
- possibly other services (a list of DIR + key/value pairs that will need templating)
- the service concurrency (number of instances it should be running in parallel)
- a pod template
- reverse proxy configuration
- systemd socket activation data

If the service with the given name already exists, then the service is updated. Upon update, the service configuration hash or generation id changes, and earlier deployments for this service are stopped and new deployments are started.

The service configuration is stored in a directory named after the service

Implementation:
- initial service declaration creates a service with `systemd-run --no-block --unit=NAME.service -pExecStop="conductor service _stop DIR" -p... conductor service start DIR`
- service update reloads the service with `systemctl reload NAME.service`



conductor service _start|_stop|_reload|_cleanup SERVICEDIR
----------------------------------------------------------

ExecStart=|ExecStop=|ExecReload=|ExecStopPost= for the service. It ensures that:

- the deployments for the services are started (unless the service is stopped) according to its concurrency
- the reverse proxy is configured (a unit is created with systemd-run that will continue to be active even if the service itself is stopped)
- old deployments with stale configuration is stopped

During a reload, the list of changes is generated to see if the service it self changed (perform a rolling restart) or if the reverse proxy config changed (reload the reverse proxy config), or both


conductor deployment _start|_stop|_cleanup DEPLOYMENTDIR
--------------------------------------------------------

ExecStart=|ExecStop=|ExecStopPost= for a service deployment

A service deployment links to a SERVICEDIR but it also contains:
- the configuration hash or generation id of the service
- anything from SERVICEDIR that is needed. It must be possible to have all the needed configuration even if SERVICEDIR has been updated with a new configuration
- the pod file after templating
- runtime data such as the pod ip address

Procedure for startup:
- start the processes
- write down runtime configuration
- configure reverse proxy
- check customizable health checks

Procedure for shutdown:
- custom hook to notify processes that shutting down is in progress
- remove from load balancer
- custom hook to wait for processes to stop doing anything important
- stopping of the processes


conductor cgi declare MANIFEST
------------------------------

Declares a CGI function.
Digest of the manifest is computed and is used to access the function.
The function can declare aliases

What this does is:
- it creates a unit via systemd-run to run the function from a socket
- it declares a unix domain socket for this function
- it register with the load balancer endpoints to hook on the socket

The manifest contains:
- a name, purely for description
- a list of aliases it wants
- the interface it talks to withthe socket (CGI, ...)
- the execution engine (pure exec, podman pod, ...)

This could be an alias of "conductor deployment declare"

- CGI function = socket


conductor cgi identify MANIFEST
-------------------------------

Shows the manifest digest on the standard output


conductor cgi list
------------------

List all the declared functions


conductor cgi rm ID
-------------------

Remove function by id

conductor cgi serve
-------------------

This is a service that:

- monitor the currently installed cgi functions
- for each function:
    - provide a unix domain socket
    - configure the load-balancer to use this socket
- listening on each socket, when a connection is made
  (perhaps let systemd sockets handle this part)
    - spawn the cgi function
    - allow the cgi function to talk CGI or HTTP over stdio
    - spawning the function is made under a systemd scope via systemd-run
    - allow to preload some functions (if they are talking HTTP and not CGI) in
      order to get better performance

