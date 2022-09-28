module github.com/pgavlin/dawn

go 1.19

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/dustin/go-humanize v1.0.0
	github.com/fatih/color v1.11.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/otiai10/copy v1.6.0
	github.com/pelletier/go-toml/v2 v2.0.5
	github.com/rjeczalik/notify v0.9.3-0.20210809113154-3472d85e95cd
	github.com/shirou/gopsutil/v3 v3.22.8
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.0
	go.starlark.net v0.0.0-20220926145019-14b050677505
	golang.org/x/sys v0.0.0-20220926163933-8cfa568d3c25
	golang.org/x/term v0.0.0-20220526004731-065cf7ba2467
	golang.org/x/tools v0.1.12
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
	golang.org/x/mod v0.6.0-dev.0.20220419223038-86c51ed26bb4 // indirect
	golang.org/x/sync v0.0.0-20220722155255-886fb9371eb4 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace go.starlark.net => github.com/pgavlin/starlark-go v0.0.0-20220928151244-7f018528dd5d
