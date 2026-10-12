import { BaseApp } from '@zeppos/zml/base-app'

// BaseApp is what opens the channel to the phone, and it does so for the
// background service as much as for the page: both find it on globalData.
App(BaseApp({ globalData: {} }))
