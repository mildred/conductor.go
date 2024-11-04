module github.com/mildred/conductor.go

go 1.22.7

require (
	github.com/PaesslerAG/jsonpath v0.1.1
	github.com/coreos/go-systemd/v22 v22.5.0
	github.com/cyberphone/json-canonicalization v0.0.0-20231217050601-ba74d44ecf5f
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/integrii/flaggy v1.5.2
	github.com/rodaine/table v1.3.0
	github.com/yookoala/realpath v1.0.0
	golang.org/x/crypto v0.28.0
)

require (
	github.com/PaesslerAG/gval v1.0.0 // indirect
	github.com/godbus/dbus/v5 v5.0.4 // indirect
	golang.org/x/sys v0.26.0 // indirect
)

replace github.com/integrii/flaggy => github.com/mildred/flaggy v0.0.0-20241104100254-d4f73417d7ae
