package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

type cpuStats struct {
	count int

	total float64
	io    float64

	totalPercent float64
	ioPercent    float64
}

func (s *cpuStats) update(delta float64) {
	if s.count == 0 {
		const logical = true
		s.count, _ = cpu.Counts(logical)
	}

	const perCPU = false
	stats, err := cpu.Times(perCPU)
	if err != nil || len(stats) == 0 {
		return
	}

	t := stats[0]
	total, io := t.Total()-t.Idle-t.Iowait, t.Iowait

	s.totalPercent = 100.0 * (total - s.total) / delta / float64(s.count)
	s.ioPercent = 100.0 * (io - s.io) / delta / float64(s.count)
	s.total, s.io = total, io
}

func (s *cpuStats) String() string {
	return fmt.Sprintf("CPU: %3.0f%%", s.totalPercent)
}

type memStats struct {
	total       uint64
	used        uint64
	usedPercent float64
}

func (s *memStats) update() {
	stats, err := mem.VirtualMemory()
	if err != nil {
		return
	}

	s.total = stats.Total
	s.used = stats.Used
	s.usedPercent = stats.UsedPercent
}

func (s *memStats) String() string {
	total, units := humanize.ComputeSI(float64(s.total))
	scale := total / float64(s.total)

	totalStr := strconv.FormatFloat(total, 'f', 0, 64)
	usedStr := strconv.FormatFloat(float64(s.used)*scale, 'f', 0, 64)

	return fmt.Sprintf("Mem: %s/%s%sB", usedStr, totalStr, units)
}

type diskStats struct {
	read  uint64
	write uint64

	readRate  float64
	writeRate float64
}

func (s *diskStats) update(delta float64) {
	stats, err := disk.IOCounters()
	if err != nil {
		return
	}

	var read, write uint64
	for _, c := range stats {
		read, write = read+c.ReadBytes, write+c.WriteBytes
	}

	s.readRate = float64(read-s.read) / delta
	s.writeRate = float64(write-s.write) / delta
	s.read, s.write = read, write
}

func (s *diskStats) String() string {
	read, readUnits := humanize.ComputeSI(s.readRate)
	write, writeUnits := humanize.ComputeSI(s.writeRate)

	readStr := strconv.FormatFloat(read, 'f', 0, 64)
	writeStr := strconv.FormatFloat(write, 'f', 0, 64)

	if s.readRate < 1 {
		readStr, readUnits = "0", ""
	}
	if s.writeRate < 1 {
		writeStr, writeUnits = "0", ""
	}

	return fmt.Sprintf("Disk: ⬇%s%sB/S, ⬆%s%sB/S", readStr, readUnits, writeStr, writeUnits)
}

type netStats struct {
	recvd uint64
	sent  uint64

	recvRate float64
	sendRate float64
}

func (s *netStats) update(delta float64) {
	const perNIC = false
	stats, err := net.IOCounters(perNIC)
	if err != nil {
		return
	}

	t := stats[0]
	s.recvRate = float64(t.BytesRecv-s.recvd) / delta
	s.sendRate = float64(t.BytesSent-s.sent) / delta
	s.recvd, s.sent = t.BytesRecv, t.BytesSent
}

func (s *netStats) String() string {
	recv, recvUnits := humanize.ComputeSI(s.recvRate)
	send, sendUnits := humanize.ComputeSI(s.sendRate)

	recvStr := strconv.FormatFloat(recv, 'f', 0, 64)
	sendStr := strconv.FormatFloat(send, 'f', 0, 64)

	if s.recvRate < 1 {
		recvStr, recvUnits = "0", ""
	}
	if s.sendRate < 1 {
		sendStr, sendUnits = "0", ""
	}

	return fmt.Sprintf("Net: ⬇%s%sB/S, ⬆%s%sB/S", recvStr, recvUnits, sendStr, sendUnits)
}

type systemStats struct {
	cpu  cpuStats
	mem  memStats
	disk diskStats
	net  netStats

	when time.Time
}

func (s *systemStats) update(now time.Time) bool {
	delta := now.Sub(s.when).Seconds()
	if delta < 1.0 {
		return false
	}
	s.when = now

	s.cpu.update(delta)
	s.mem.update()
	s.disk.update(delta)
	s.net.update(delta)
	return true
}

func (s *systemStats) line() string {
	cpu, mem, disk, net := s.cpu.String(), s.mem.String(), s.disk.String(), s.net.String()
	return strings.Join([]string{cpu, mem, disk, net}, " ")
}
