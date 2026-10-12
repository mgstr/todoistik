import test from 'node:test'
import assert from 'node:assert/strict'
import { report, OK, SENT, ALREADY, NOT_SET, NOT_REACHED, REFUSED } from '../app-side/report.js'

function store(values = {}) {
  const m = new Map(Object.entries(values))
  return { getItem: (k) => m.get(k), setItem: (k, v) => m.set(k, v) }
}

function app(status = 200) {
  const calls = []
  const fetch = async (req) => {
    calls.push(req)
    return { status, body: '{"status":"accepted"}' }
  }
  return { calls, fetch }
}

const SET = { url: 'http://mac:8390/', token: 'tok' }

test('above the threshold nothing is sent', async () => {
  const a = app()
  assert.equal(await report(21, store(SET), a.fetch), OK)
  assert.equal(a.calls.length, 0)
})

test('at the threshold the capture is filed, as the watch, under the token', async () => {
  const a = app()
  assert.equal(await report(20, store(SET), a.fetch), SENT)
  assert.equal(a.calls.length, 1)
  assert.equal(a.calls[0].url, 'http://mac:8390/api/capture')
  assert.equal(a.calls[0].headers.Authorization, 'Bearer tok')
  assert.deepEqual(JSON.parse(a.calls[0].body), { text: 'Charge the watch', source: 'watch' })
})

test('one discharge is one capture, however far it falls', async () => {
  const a = app()
  const s = store(SET)
  assert.equal(await report(20, s, a.fetch), SENT)
  assert.equal(await report(19, s, a.fetch), ALREADY)
  assert.equal(await report(5, s, a.fetch), ALREADY)
  assert.equal(a.calls.length, 1)
})

test('a percent of bounce is not a charge, and a charge is', async () => {
  const a = app()
  const s = store(SET)
  await report(20, s, a.fetch)
  await report(22, s, a.fetch)
  assert.equal(await report(20, s, a.fetch), ALREADY)
  await report(100, s, a.fetch)
  assert.equal(await report(20, s, a.fetch), SENT)
  assert.equal(a.calls.length, 2)
})

test('an app that was not reached is asked again', async () => {
  const s = store(SET)
  const down = async () => {
    throw new Error('network')
  }
  assert.equal(await report(18, s, down), NOT_REACHED)
  assert.equal(await report(18, s, app(401).fetch), REFUSED)
  assert.equal(await report(17, s, app().fetch), SENT)
})

test('the threshold and the text are the settings, when there are any', async () => {
  const a = app()
  const s = store({ ...SET, threshold: '35', text: ' Put the watch on its dock ' })
  assert.equal(await report(36, s, a.fetch), OK)
  assert.equal(await report(35, s, a.fetch), SENT)
  assert.equal(JSON.parse(a.calls[0].body).text, 'Put the watch on its dock')
})

test('with no address there is nothing to ask', async () => {
  const a = app()
  assert.equal(await report(10, store(), a.fetch), NOT_SET)
  assert.equal(a.calls.length, 0)
})
