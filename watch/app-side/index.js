import { BaseSideService } from '@zeppos/zml/base-side'
import { report } from './report'

// The phone's half: the only part that can reach the network, and so the part
// that holds the settings and the rule (report.js). The watch asks in two
// ways - the page waits for the answer to show it, the background service
// does not - and both are the same report.
async function battery(params) {
  const store = settings.settingsStorage
  const level = Number(params && params.level)
  const status = await report(level, store, fetch)
  store.setItem('last', `${level}% · ${status} · ${new Date().toLocaleString()}`)
  return status
}

AppSideService(
  BaseSideService({
    onInit() {},
    onRun() {},
    onDestroy() {},

    onRequest(req, res) {
      if (req.method !== 'battery') return res({ message: 'unknown method' })
      battery(req.params).then(
        (status) => res(null, { status }),
        (e) => res({ message: String(e) }),
      )
    },

    onCall(req) {
      if (req && req.method === 'battery') battery(req.params)
    },
  }),
)
