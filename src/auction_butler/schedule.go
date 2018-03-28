package auction_butler

import (
	"fmt"
	"time"
)

type task int

const (
	nothing    task = iota
	endAuction
	reminderAnnouncement
	announceEventEnd
)

// Returns what to do next (start, stop or nothing) and when
func (bot *Bot) schedule() (task, time.Time) {
	auction := bot.db.GetCurrentAuction()
	if auction == nil {
		return nothing, time.Time{}
	}

	if auction.EndTime.Valid {
		return endAuction, auction.EndTime.Time
	}

	return nothing, time.Time{}

}

// Returns a more detailed version than `schedule()`
// of what to do next (including announcements).
func (bot *Bot) subSchedule() (task, time.Time) {
	tsk, future := bot.schedule()
	if tsk == nothing {
		return nothing, time.Time{}
	}

	// at what intervals to send the reminder for time left
	every := bot.config.ReminderAnnounceInterval.Duration

	announcements := time.Until(future) / every
	if announcements <= 0 {
		return tsk, future
	}

	nearFuture := future.Add(-announcements * every)
	switch tsk {
	case endAuction:
		return announceEventEnd, nearFuture
	default:
		log.Print("unsupported task to subSchedule")
		return nothing, time.Time{}
	}
}

func (bot *Bot) perform(tsk task) {
	event := bot.db.GetCurrentAuction()
	if event == nil {
		log.Print("failed to perform the scheduled task: no current auction")
		return
	}

	noctx := &Context{}
	switch tsk {

	default:
		log.Printf("unsupported task to perform: %v", tsk)
	}
}

func (bot *Bot) maintain() {
	bot.rescheduleChan = make(chan int)
	defer func() {
		close(bot.rescheduleChan)
	}()
	var timer *time.Timer
	for {

		tsk, future := bot.subSchedule()
		if tsk == nothing {
			<-bot.rescheduleChan
			continue
		}

		if timer == nil {
			timer = time.NewTimer(time.Until(future))
		} else {
			timer.Reset(time.Until(future))
		}
		select {
		case <-timer.C:
			bot.perform(tsk)
		case <-bot.rescheduleChan:
			if !timer.Stop() {
				<-timer.C
			}
		}
	}
}

// Cause a reschedule to happen. Call this if you modify events, so that the
// bot could wake itself up at correct times for automatic announcements and
// event starting/stopping.
func (bot *Bot) Reschedule() {
	bot.rescheduleChan <- 1
}
