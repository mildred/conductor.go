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
	golang.org/x/crypto v0.30.0
)

require (
	github.com/PaesslerAG/gval v1.2.4 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
)

replace github.com/integrii/flaggy => github.com/mildred/flaggy v0.0.0-20241205182850-8780e26a6fe0
