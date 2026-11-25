package intl

import (
	"context"
	"strconv"
	"strings"
	"time"
)

// Calendar is what a month calendar writes in a language, from Intl: gsxui's ui.CalendarLocale, field for field, so
// <ui.Calendar locale={ ui.CalendarLocale(intl.CalendarOf(ctx)) } weekStartsOn={ intl.WeekStart(ctx) }/> is in the
// request's language. Months are the standalone names a caption uses ("январь 2026"), MonthsFormat the names inside a
// date, which day labels use ("15 января 2026 г.").
type Calendar struct {
	Months        [12]string // wide standalone names, January first
	MonthsFormat  [12]string // wide names inside a date, January first
	MonthsShort   [12]string // abbreviated names, January first
	Weekdays      [7]string  // wide names, Sunday first
	WeekdaysShort [7]string  // narrow names, Sunday first
	Caption       string     // the month and year: "{month} {year}", "{year}年{month}"
	DayLabel      string     // a day: "{weekday}, {month} {day}, {year}"
	Digits        string     // the ten digits 0-9 the language writes, "" for ASCII
}

// CalendarOf is the request's language's Calendar (the source locale's without a request), "" fields where Intl
// can't (gsxui then writes English).
func CalendarOf(ctx context.Context) Calendar {
	c, _ := Current(ctx).calendar()
	return c
}

// WeekStart is the first day of the week in the request's locale (Intl.Locale's week info, from its region:
// Monday in Germany, Sunday in the United States, Saturday in Egypt).
func WeekStart(ctx context.Context) time.Weekday {
	_, first := Current(ctx).calendar()
	return first
}

// calendar is asked once per locale and kept: it is the same for every request.
func (l *Locale) calendar() (Calendar, time.Weekday) {
	l.calOnce.Do(func() { l.cal, l.weekStart = l.askCalendar() })
	return l.cal, l.weekStart
}

func (l *Locale) askCalendar() (Calendar, time.Weekday) {
	var c Calendar
	s, ok := call("calendar", l.id)
	f := strings.Split(s, "\u001e")
	if !ok || len(f) != 9 {
		return c, time.Sunday
	}
	copy(c.Months[:], strings.Split(f[0], "\u001f"))
	copy(c.MonthsFormat[:], strings.Split(f[1], "\u001f"))
	copy(c.MonthsShort[:], strings.Split(f[2], "\u001f"))
	copy(c.Weekdays[:], strings.Split(f[3], "\u001f"))
	copy(c.WeekdaysShort[:], strings.Split(f[4], "\u001f"))
	c.Caption, c.DayLabel, c.Digits = f[5], f[6], f[7]
	first, _ := strconv.Atoi(f[8])
	return c, time.Weekday(first % 7)
}
