# BOINC Client Prometheus exporter

It's a dirty hackish prototype that I cannot be bothered to expand more :D. Not using Prometheus library because it would no longer be a 1 go source file.

Tested against BOINC client 7.14.2 and 7.16.6 over localhost and remote.

If you want more metrics then either add them and throw me a PR. Or just don't. Your choice. I don't care :)

Distributed under MIT license because I don't care what you do with it and I don't want any liability yada yada thingy.

## Building

Just do `go build main.go` or something. The password is set via the first argument (use `./exporter somepassword`).
