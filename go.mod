module github.com/pgavlin/dawn

go 1.16

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/dustin/go-humanize v1.0.0
	github.com/fatih/color v1.11.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/otiai10/copy v1.6.0
	github.com/pelletier/go-toml/v2 v2.0.5
	github.com/rjeczalik/notify v0.9.2
	github.com/shirou/gopsutil/v3 v3.22.8
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.0
	go.starlark.net v0.0.0-20220926145019-14b050677505
	golang.org/x/sys v0.0.0-20220926163933-8cfa568d3c25
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
	golang.org/x/tools v0.1.12
	mvdan.cc/sh/v3 v3.3.0
)

replace go.starlark.net => github.com/pgavlin/starlark-go v0.0.0-20210619021655-f74f6ce4d501
