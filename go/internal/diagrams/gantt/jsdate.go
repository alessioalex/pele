package gantt

import (
	"math"
	"time"
)

// A JavaScript Date as V8 implements it: milliseconds since the epoch, NaN when invalid, with
// fields read and written in the local time of time.Local.

const (
	msPerSecond = 1000.0
	msPerMinute = 60000.0
	msPerHour   = 3600000.0
	msPerDay    = 86400000.0

	maxTimeInMs          = 8.64e15
	maxTimeBeforeUTCInMs = maxTimeInMs + 864e6

	msPerDayInt = 86400000
)

type jsDate struct{ t float64 }

// newDate is `new Date(ms)`.
func newDate(ms float64) *jsDate { return &jsDate{timeClip(ms)} }

// copyDate is `new Date(date)`.
func copyDate(d *jsDate) *jsDate { return &jsDate{d.t} }

// dateNow is Date.now().
func dateNow() float64 { return float64(time.Now().UnixMilli()) }

func doubleToInteger(x float64) float64 {
	if math.IsNaN(x) {
		return 0
	}
	if math.IsInf(x, 0) {
		return x
	}
	return math.Trunc(x)
}

func timeClip(t float64) float64 {
	if -maxTimeInMs <= t && t <= maxTimeInMs {
		return math.Trunc(t) + 0
	}
	return math.NaN()
}

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func makeTime(h, m, s, ms float64) float64 {
	if finite(h) && finite(m) && finite(s) && finite(ms) {
		return math.Trunc(h)*msPerHour + math.Trunc(m)*msPerMinute + math.Trunc(s)*msPerSecond + math.Trunc(ms)
	}
	return math.NaN()
}

var (
	daysBeforeMonth     = [12]int{0, 31, 59, 90, 120, 151, 181, 212, 243, 273, 304, 334}
	daysBeforeMonthLeap = [12]int{0, 31, 60, 91, 121, 152, 182, 213, 244, 274, 305, 335}
)

// makeDay follows V8, which works in 32-bit integers over a bounded range of years and months.
func makeDay(year, month, date float64) float64 {
	if !(-1000000 <= year && year <= 1000000 && -10000000 <= month && month <= 10000000 && finite(date)) {
		return math.NaN()
	}
	y := int(year)
	m := int(month)
	y += m / 12
	m %= 12
	if m < 0 {
		m += 12
		y--
	}
	const delta = 399999
	const base = 365*(1970+delta) + (1970+delta)/4 - (1970+delta)/100 + (1970+delta)/400
	day := 365*(y+delta) + (y+delta)/4 - (y+delta)/100 + (y+delta)/400 - base
	if y%4 != 0 || (y%100 == 0 && y%400 != 0) {
		day += daysBeforeMonth[m]
	} else {
		day += daysBeforeMonthLeap[m]
	}
	return float64(day-1) + doubleToInteger(date)
}

func makeDate(day, t float64) float64 {
	if finite(day) && finite(t) {
		if t == 0 && day != 0 {
			return day * msPerDay
		}
		return t + day*msPerDay
	}
	return math.NaN()
}

// localOffset is the offset of local time from UTC at a UTC instant, in milliseconds.
func localOffset(utc float64) float64 {
	if time.Local == time.UTC {
		return 0
	}
	_, off := time.UnixMilli(int64(math.Floor(utc))).In(time.Local).Zone()
	return float64(off) * 1000
}

// toUTC turns a local time into a UTC instant, taking the offset that applied just before it.
func toUTC(local float64) float64 {
	if time.Local == time.UTC {
		return local
	}
	off := localOffset(local - localOffset(local))
	return local - off
}

// setLocal stores a time computed in local time, as V8's SetLocalDateValue does.
func (d *jsDate) setLocal(local float64) {
	if local >= -maxTimeBeforeUTCInMs && local <= maxTimeBeforeUTCInMs {
		d.t = timeClip(toUTC(local))
	} else {
		d.t = math.NaN()
	}
}

// newLocalDate is `new Date(year, month, day, hours, minutes, seconds, ms)`.
func newLocalDate(year, month, day, h, m, s, ms float64) *jsDate {
	year = fullYear(year)
	d := &jsDate{}
	d.setLocal(makeDate(makeDay(year, month, day), makeTime(h, m, s, ms)))
	return d
}

// dateUTC is Date.UTC(year, month, day, hours, minutes, seconds, ms).
func dateUTC(year, month, day, h, m, s, ms float64) float64 {
	year = fullYear(year)
	return timeClip(makeDate(makeDay(year, month, day), makeTime(h, m, s, ms)))
}

// fullYear maps the years 0 to 99 to 1900 to 1999, as the Date constructor and Date.UTC do.
func fullYear(year float64) float64 {
	if !math.IsNaN(year) {
		if y := doubleToInteger(year); y >= 0 && y <= 99 {
			return 1900 + y
		}
	}
	return year
}

// civil is a local date broken into fields.
type civil struct {
	year, month, day, weekday int
	days                      int64
	timeInDay                 int64
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func civilFromLocal(local float64) civil {
	ms := int64(local)
	days := floorDiv(ms, msPerDayInt)
	c := civil{days: days, timeInDay: ms - days*msPerDayInt}
	c.weekday = int(((days+4)%7 + 7) % 7)
	z := days + 719468
	era := z
	if era < 0 {
		era -= 146096
	}
	era /= 146097
	doe := z - era*146097
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	y := yoe + era*400
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	c.day = int(doy - (153*mp+2)/5 + 1)
	if mp < 10 {
		c.month = int(mp + 2)
	} else {
		c.month = int(mp - 10)
	}
	if c.month <= 1 {
		y++
	}
	c.year = int(y)
	return c
}

func (d *jsDate) valid() bool { return !math.IsNaN(d.t) }

func (d *jsDate) getTime() float64 { return d.t }

func (d *jsDate) local() civil { return civilFromLocal(d.t + localOffset(d.t)) }

func (d *jsDate) getFullYear() float64 {
	if !d.valid() {
		return math.NaN()
	}
	return float64(d.local().year)
}

func (d *jsDate) getMonth() float64 {
	if !d.valid() {
		return math.NaN()
	}
	return float64(d.local().month)
}

func (d *jsDate) getDate() float64 {
	if !d.valid() {
		return math.NaN()
	}
	return float64(d.local().day)
}

func (d *jsDate) getDay() float64 {
	if !d.valid() {
		return math.NaN()
	}
	return float64(d.local().weekday)
}

func (d *jsDate) getHours() float64 {
	if !d.valid() {
		return math.NaN()
	}
	return float64(d.local().timeInDay / 3600000)
}

func (d *jsDate) getMinutes() float64 {
	if !d.valid() {
		return math.NaN()
	}
	return float64(d.local().timeInDay / 60000 % 60)
}

func (d *jsDate) getSeconds() float64 {
	if !d.valid() {
		return math.NaN()
	}
	return float64(d.local().timeInDay / 1000 % 60)
}

func (d *jsDate) getMilliseconds() float64 {
	if !d.valid() {
		return math.NaN()
	}
	return float64(d.local().timeInDay % 1000)
}

func (d *jsDate) getUTCSeconds() float64 {
	if !d.valid() {
		return math.NaN()
	}
	return float64(civilFromLocal(d.t).timeInDay / 1000 % 60)
}

func (d *jsDate) getTimezoneOffset() float64 {
	if !d.valid() {
		return math.NaN()
	}
	return -localOffset(d.t) / msPerMinute
}

func (d *jsDate) setTime(t float64) { d.t = timeClip(t) }

func (d *jsDate) setDate(date float64) {
	if !d.valid() {
		return
	}
	c := d.local()
	d.setLocal(makeDate(makeDay(float64(c.year), float64(c.month), date), float64(c.timeInDay)))
}

func (d *jsDate) setMonth(month float64) {
	if !d.valid() {
		return
	}
	c := d.local()
	d.setLocal(makeDate(makeDay(float64(c.year), month, float64(c.day)), float64(c.timeInDay)))
}

func (d *jsDate) setMonthDate(month, date float64) {
	if !d.valid() {
		return
	}
	c := d.local()
	d.setLocal(makeDate(makeDay(float64(c.year), month, date), float64(c.timeInDay)))
}

func (d *jsDate) setFullYear(year float64) {
	m, dt, tid := 0.0, 1.0, 0.0
	if d.valid() {
		c := d.local()
		m, dt, tid = float64(c.month), float64(c.day), float64(c.timeInDay)
	}
	d.setLocal(makeDate(makeDay(year, m, dt), tid))
}

func (d *jsDate) setHours(h, m, s, ms float64) {
	if !d.valid() {
		return
	}
	c := d.local()
	d.setLocal(makeDate(float64(c.days), makeTime(h, m, s, ms)))
}

// dateValue is `+date`, with undefined as NaN.
func dateValue(d *jsDate) float64 {
	if d == nil {
		return math.NaN()
	}
	return d.t
}
