package main

import (
	"fmt"
	"time"

	ui "github.com/gizak/termui"
)

func UiSetup() (err error) {
	err = ui.Init()
	if err != nil {
		return
	}
	defer ui.Close()

	strs := [] string{
		"Up Time: 0 second",
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
		for {
			t := <-ticker.C
			dur := t.Sub(now)
			strs[0] = fmt.Sprintf("Up Time: %v", humanizeDur(dur))
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
