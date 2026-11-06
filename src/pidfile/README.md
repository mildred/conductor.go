# PIDFile Package for Go

The `pidfile` package provides a simple way to ensure that only one instance of a Go application runs at any given time by using PID (Process ID) files.

## Features

- Easy creation and management of a PID file.
- Checks if an application is already running.
- Prevents multiple instances of the application from running concurrently.

## Installation

To install the `pidfile` package, use the following `go get` command:

```sh
go get github.com/makifdb/pidfile
```

## Example
Here is a full example of an application that uses the pidfile package to ensure single instance execution

```go
package main

import (
    "fmt"
    "log"
    "os"
    "github.com/makifdb/pidfile"
)

func main() {
    // Define the PID file path
    pidFilePath := filepath.Join(os.TempDir(), "example.pid")

    // Create or update the PID file
    err := pidfile.CreateOrUpdatePIDFile(pidFilePath)
    if err != nil {
        log.Fatalf("Unable to create or update PID file: %v", err)
    }
    
    // Defer the removal of the PID file on application exit
    defer os.Remove(pidFilePath)
    
    // Your application logic goes here
    fmt.Println("Application started successfully.")

    // Application logic...
}
```

## Notes
This package assumes that the PID file is stored in a location that is writable by the application.
Make sure to handle the PID file correctly to prevent orphaned PID files, which could prevent the application from starting.

## License
This pidfile package is open-source software licensed under the MIT license.