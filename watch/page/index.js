import { createWidget, widget, align, prop } from '@zos/ui'
import { Battery } from '@zos/sensor'
import { queryPermission, requestPermission } from '@zos/app'
import * as appService from '@zos/app-service'
import { BasePage } from '@zeppos/zml/base-page'

const SERVICE = 'app-service/index'
const PERMISSIONS = ['device:os.bg_service']

// Opening the app does the two things that cannot happen by themselves: it
// starts the service that reports from then on, which the system only lets a
// page do, and it reports once with the answer on screen, which is the only
// way to see from the wrist whether the phone reached todoistik.
Page(
  BasePage({
    build() {
      const level = new Battery().getCurrent()
      createWidget(widget.TEXT, {
        x: 0,
        y: 150,
        w: 480,
        h: 120,
        text: `${level}%`,
        text_size: 96,
        color: 0xffffff,
        align_h: align.CENTER_H,
        align_v: align.CENTER_V,
      })
      const answer = createWidget(widget.TEXT, {
        x: 40,
        y: 280,
        w: 400,
        h: 50,
        text: '',
        text_size: 32,
        color: 0x999999,
        align_h: align.CENTER_H,
        align_v: align.CENTER_V,
      })

      this.startService()
      this.request({ method: 'battery', params: { level } })
        .then((r) => answer.setProperty(prop.TEXT, r.status))
        .catch(() => answer.setProperty(prop.TEXT, 'no phone'))
    },

    startService() {
      if (appService.getAllAppServices().includes(SERVICE)) return
      const start = () => appService.start({ url: SERVICE, param: '' })
      const [granted] = queryPermission({ permissions: PERMISSIONS })
      if (granted === 2) return start()
      if (granted === 0) {
        requestPermission({
          permissions: PERMISSIONS,
          callback([answer]) {
            if (answer === 2) start()
          },
        })
      }
    },
  }),
)
