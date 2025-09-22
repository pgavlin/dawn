package main

import (
	"errors"
	"os"
	"runtime/pprof"
	"runtime/trace"

	"github.com/pgavlin/starlark-go/starlark"
)

type profiler struct {
	cpuPath   string
	starPath  string
	tracePath string

	cpu   *os.File
	star  *os.File
	trace *os.File
}

func (p *profiler) start() (err error) {
	if p.cpuPath != "" {
		if p.cpu, err = os.Create(p.cpuPath); err != nil {
			return err
		}
		if err = pprof.StartCPUProfile(p.cpu); err != nil {
			return err
		}
	}
	if p.starPath != "" {
		if p.star, err = os.Create(p.starPath); err != nil {
			return err
		}
		if err = starlark.StartProfile(p.star); err != nil {
			return err
		}
	}
	if p.tracePath != "" {
		if p.trace, err = os.Create(p.tracePath); err != nil {
			return err
		}
		if err = trace.Start(p.trace); err != nil {
			return err
		}
	}
	return nil
}

func (p *profiler) stop() error {
	var err error
	if p.cpuPath != "" {
		pprof.StopCPUProfile()
		err = errors.Join(err, p.cpu.Close())
	}
	if p.starPath != "" {
		err = errors.Join(func() error {
			if err := starlark.StopProfile(); err != nil {
				return err
			}
			return p.star.Close()
		}())
	}
	if p.tracePath != "" {
		trace.Stop()
		err = errors.Join(err, p.trace.Close())
	}
	return err
}
