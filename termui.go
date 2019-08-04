package main

import (
	"fmt"
	"strconv"
	"time"

	ui "github.com/gizak/termui"
)

func UiSetup(chanSize chan int64) (err error) {
	err = ui.Init()
	if err != nil {
		return
	}
	defer ui.Close()

	strs := [] string{
		"Up Time: 0 second",
		"Shared Size: 0",
		":PRESS q to quit",
	}

	ls := ui.NewList()
	ls.Items = strs
	ls.Width = 70
	ls.Height = len(strs) + 2
	ls.BorderLabel = "Status"
	ls.BorderFg = ui.ColorYellow
	ls.PaddingLeft = 1

	ui.Render(ls)
	ticker := time.NewTicker(time.Second)
	now := time.Now()
	go func() {
		var totalSize int64 = 0
		var t time.Time
		for {
		inner:
			for {
				select {
				case size := <-chanSize:
					totalSize += size
				case t = <-ticker.C:
					break inner
				}
			}
			dur := t.Sub(now)
			strs[0] = fmt.Sprintf("Up Time: %v", humanizeDur(dur))
			strs[1] = fmt.Sprintf("Shared Size: %v", humanizeSize(totalSize))

			ui.Render(ls)
		}
	}()

	ui.Handle("/sys/kbd/q", func(ui.Event) {
		ui.StopLoop()
	})
	ui.Loop()
	return
}

func humanizeDur(dur time.Duration) string {
	ret := ""
	secs := int64(dur.Seconds())

	plural := func(n int64, s string) string {
		if n > 1 {
			s += "s"
		}
		return fmt.Sprintf("%v %v", n, s)
	}

	var min, hour, hours, days int64
	// second
	sec := secs % 60
	ret = plural(sec, "second")
	mins := secs / 60
	if mins == 0 {
		goto end
	}

	// minute
	min = mins % 60
	ret = plural(min, "minute") + " " + ret
	hours = mins / 60
	if hours == 0 {
		goto end
	}

	// hour
	hour = hours % 24
	ret = plural(hour, "hour") + " " + ret
	days = hours / 24
	if days == 0 {
		goto end
	}

	// day
	ret = plural(days, "day") + " " + ret
end:
	return ret
}

func humanizeSize(size int64) string {
	if size < 1024 {
		return strconv.FormatInt(size, 10) + " byte"
	}

	const prec = 2
	const ffmt = 'f'
	const bitSize = 32
	// KB
	s := float64(size) / 1024
	if s < 1024 {
		return strconv.FormatFloat(s, ffmt, prec, bitSize) + " KB"
	}

	// MB
	s = float64(s) / 1024
	if s < 1024 {
		return strconv.FormatFloat(s, ffmt, prec, bitSize) + " MB"
	}

	// GB
	s = float64(s) / 1024
	if s < 1024 {
		return strconv.FormatFloat(s, ffmt, prec, bitSize) + " GB"
	}

	// TB
	s = float64(s) / 1024
	return strconv.FormatFloat(s, ffmt, prec, bitSize) + " GB"
}
