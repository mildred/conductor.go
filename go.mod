module github.com/mildred/conductor.go

go 1.23

require (
	github.com/PaesslerAG/jsonpath v0.1.1
	github.com/blang/semver v3.5.1+incompatible
	github.com/coreos/go-systemd/v22 v22.5.0
	github.com/cyberphone/json-canonicalization v0.0.0-20241213102144-19d51d7fe467
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/integrii/flaggy v1.5.2
	github.com/rhysd/go-github-selfupdate v1.2.3
	github.com/rodaine/table v1.3.0
	github.com/yookoala/realpath v1.0.0
	golang.org/x/crypto v0.33.0
)

require (
	github.com/PaesslerAG/gval v1.2.4 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/go-github/v30 v30.1.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/inconshreveable/go-update v0.0.0-20160112193335-8152e7eb6ccf // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/taigrr/systemctl v1.0.10 // indirect
	github.com/tailscale/hujson v0.0.0-20250605163823-992244df8c5a // indirect
	github.com/tcnksm/go-gitconfig v0.1.2 // indirect
	github.com/ulikunitz/xz v0.5.12 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/oauth2 v0.26.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
)

replace github.com/integrii/flaggy => github.com/mildred/flaggy v0.0.0-20241205182850-8780e26a6fe0
