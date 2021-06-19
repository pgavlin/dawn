module github.com/pgavlin/dawn

go 1.16

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/dustin/go-humanize v1.0.0
	github.com/fatih/color v1.11.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/otiai10/copy v1.6.0
	github.com/pelletier/go-toml/v2 v2.0.0-beta.3
	github.com/rjeczalik/notify v0.9.2
	github.com/shirou/gopsutil/v3 v3.21.4
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.1-0.20210427113832-6241f9ab9942
	go.starlark.net v0.0.0-20210429133630-0c63ff3779a6
	golang.org/x/term v0.0.0-20210503060354-a79de5458b56
	mvdan.cc/sh/v3 v3.3.0
)

replace go.starlark.net => github.com/pgavlin/starlark-go v0.0.0-20210619021655-f74f6ce4d501
