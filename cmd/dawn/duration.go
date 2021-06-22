package main

import (
	"fmt"
	"math"
	"time"
)

type duration time.Duration

func (d duration) String() string {
	td := time.Duration(d)

	hours, minutes, seconds := int(math.Trunc(td.Hours())), int(math.Trunc(td.Minutes()/60)), int(math.Trunc(td.Seconds()/60))
	if hours == 0 {
		return fmt.Sprintf("%d:%02d", minutes, seconds)
	}

	return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
}
