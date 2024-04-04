module github.com/pgavlin/dawn

go 1.21

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/dustin/go-humanize v1.0.0
	github.com/fatih/color v1.11.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/otiai10/copy v1.6.0
	github.com/pelletier/go-toml/v2 v2.2.0
	github.com/rjeczalik/notify v0.9.3
	github.com/shirou/gopsutil/v3 v3.22.8
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.9.0
	go.starlark.net v0.0.0-20240329153429-e6e8e7ce1b7a
	golang.org/x/sys v0.18.0
	golang.org/x/term v0.0.0-20220526004731-065cf7ba2467
	golang.org/x/tools v0.19.0
	mvdan.cc/sh/v3 v3.3.0
)

require (
	github.com/chzyer/readline v0.0.0-20180603132655-2972be24d48e // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/mattn/go-colorable v0.1.8 // indirect
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/tklauser/go-sysconf v0.3.10 // indirect
	github.com/tklauser/numcpus v0.4.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	golang.org/x/mod v0.16.0 // indirect
	golang.org/x/sync v0.6.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace go.starlark.net => github.com/pgavlin/starlark-go v0.0.0-20221013154258-638b622cb2ca
