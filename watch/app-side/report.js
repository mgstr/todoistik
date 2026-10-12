// What a battery level means, and the one thing done about it. This is the
// whole of the connector's judgement and it touches nothing of Zepp's: the
// store and the fetch are handed in, so the rule is tested under node
// (test/report.test.js) and the watch and the phone only carry it.

export const SOURCE = 'watch'
export const DEFAULT_THRESHOLD = 20
export const DEFAULT_TEXT = 'Charge the watch'

// How far above the threshold a level has to climb before the next fall is a
// new one. A resting battery reads a percent up and a percent down, and
// without the gap 20, 21, 20 would be two captures for one discharge.
export const REARM = 5

// The words the watch shows and the settings page keeps. They are short
// because the wrist has one line for them.
export const OK = 'ok'
export const SENT = 'sent'
export const ALREADY = 'already sent'
export const NOT_SET = 'not set up'
export const NOT_REACHED = 'not reached'
export const REFUSED = 'refused'

export function threshold(store) {
  const n = parseInt(store.getItem('threshold'), 10)
  return n > 0 && n < 100 ? n : DEFAULT_THRESHOLD
}

// report is told a level and answers with one of the words above.
//
// A discharge is filed once. The text is the same every time on purpose: the
// inbox collapses a capture identical to one still sitting in it, so a level
// in the text would make every percent a new item. Collapse only covers the
// time the item is in the inbox, though, and the first thing done with it is
// to process it, which is why "filed" is remembered here and cleared only by
// charging.
//
// It is remembered only once the app has said it has the text. A phone away
// from the network, a wrong token, a server that is down: each leaves it
// unfiled, and the next level reported tries again.
export async function report(level, store, fetch) {
  const limit = threshold(store)
  if (level >= limit + REARM) store.setItem('filed', '')
  if (level > limit) return OK
  if (store.getItem('filed')) return ALREADY

  const base = (store.getItem('url') || '').trim().replace(/\/+$/, '')
  if (!base) return NOT_SET
  const text = (store.getItem('text') || '').trim() || DEFAULT_TEXT

  let res
  try {
    res = await fetch({
      url: base + '/api/capture',
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: 'Bearer ' + (store.getItem('token') || '').trim(),
      },
      body: JSON.stringify({ text, source: SOURCE }),
    })
  } catch (e) {
    return NOT_REACHED
  }
  if (!res || res.status < 200 || res.status > 299) return REFUSED

  // accepted and duplicate both mean the inbox has it
  store.setItem('filed', '1')
  return SENT
}
