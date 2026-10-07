import httpClient from '../utils/httpClient'

export default {
  loginLogList (query, callback) {
    httpClient.getPage('/system/login-log', query, callback)
  }
}
