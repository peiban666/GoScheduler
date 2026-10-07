import httpClient from '../utils/httpClient'

export default {
  slack (callback) {
    httpClient.getObject('/system/slack', {}, callback)
  },
  updateSlack (data, callback) {
    httpClient.post('/system/slack/update', data, callback)
  },
  createSlackChannel (channel, callback) {
    httpClient.post('/system/slack/channel', {channel}, callback)
  },
  removeSlackChannel (channelId, callback) {
    httpClient.post(`/system/slack/channel/remove/${channelId}`, {}, callback)
  },
  mail (callback) {
    httpClient.getObject('/system/mail', {}, callback)
  },
  updateMail (data, callback) {
    httpClient.post('/system/mail/update', data, callback)
  },
  createMailUser (data, callback) {
    httpClient.post('/system/mail/user', data, callback)
  },
  removeMailUser (userId, callback) {
    httpClient.post(`/system/mail/user/remove/${userId}`, {}, callback)
  },
  webhook (callback) {
    httpClient.getObject('/system/webhook', {}, callback)
  },
  updateWebHook (data, callback, failed) {
    httpClient.post('/system/webhook/update', data, callback, failed)
  },
  webhooks (callback) {
    httpClient.get('/system/webhook/list', {}, callback)
  },
  webhookOptions (callback) {
    httpClient.get('/system/webhook/options', {}, callback)
  },
  storeWebhook (data, callback, failed) {
    httpClient.post('/system/webhook/store', data, callback, failed)
  },
  removeWebhook (id, callback, failed) {
    httpClient.post(`/system/webhook/remove/${id}`, {}, callback, failed)
  },
  testWebhook (data, callback, failed) {
    httpClient.post('/system/webhook/test', data, callback, failed)
  }
}
