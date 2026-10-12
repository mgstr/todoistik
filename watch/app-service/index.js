import { Battery, Time } from '@zos/sensor'

// The half that runs with the app closed. It decides nothing: it says the
// level to the phone, and the phone knows the threshold, what was already
// filed and where todoistik is (app-side/report.js). A watch that knew any of
// that would need telling again every time a setting moved.
const battery = new Battery()
const time = new Time()

function report() {
  const { messaging } = getApp()._options.globalData
  // call, not request: nothing here waits for the answer, and a phone that is
  // out of reach is told again at the next percent or the next hour
  messaging.call({ method: 'battery', params: { level: battery.getCurrent() } })
}

// A percent lost is the event, and the hour is the retry: a level that fell
// while the phone was away from the network todoistik is on would otherwise
// wait for the next percent, which on this watch is most of a day.
function hourly() {
  if (time.getMinutes() === 0) report()
}

AppService({
  onInit() {
    battery.onChange(report)
    time.onPerMinute(hourly)
    report()
  },
  onDestroy() {
    battery.offChange(report)
  },
})
