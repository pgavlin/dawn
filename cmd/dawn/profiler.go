package main

import (
	"os"
	"runtime/pprof"
	"runtime/trace"

	"go.starlark.net/starlark"
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
		pprof.StartCPUProfile(p.cpu)
	}
	if p.starPath != "" {
		if p.star, err = os.Create(p.starPath); err != nil {
			return err
		}
		starlark.StartProfile(p.star)
	}
	if p.tracePath != "" {
		if p.trace, err = os.Create(p.tracePath); err != nil {
			return err
		}
		trace.Start(p.trace)
	}
	return nil
}

func (p *profiler) stop() error {
	if p.cpuPath != "" {
		pprof.StopCPUProfile()
		p.cpu.Close()
	}
	if p.starPath != "" {
		starlark.StopProfile()
		p.star.Close()
	}
	if p.tracePath != "" {
		trace.Stop()
		p.trace.Close()
	}

	return nil
}
