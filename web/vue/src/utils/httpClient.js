import axios from 'axios'
import {Message} from 'element-ui'
import router from '../router/index'
import store from '../store/index'
import Qs from 'qs'
import {normalizePageResponse, requireObjectResponse} from './responseData'

const errorMessage = '加载失败, 请稍后再试'
// 成功状态码
const SUCCESS_CODE = 0
// 认证失败
const AUTH_ERROR_CODE = 401
// 应用未安装
const APP_NOT_INSTALL_CODE = 801

axios.defaults.baseURL = 'api'
axios.defaults.timeout = 10000
axios.defaults.responseType = 'json'
axios.interceptors.request.use(config => {
  config.headers['Auth-Token'] = store.getters.user.token
  return config
}, error => {
  Message.error({
    message: errorMessage
  })

  return Promise.reject(error)
})

axios.interceptors.response.use(data => {
  return data
}, error => {
  Message.error({
    message: errorMessage
  })

  return Promise.reject(error)
})

function handle (promise, next, failed) {
  promise.then((res) => successCallback(res, next, failed))
    .catch((error) => {
      failureCallback(error)
      if (failed) failed(error)
    })
}

function checkResponseCode (code, msg) {
  switch (code) {
    // 应用未安装
    case APP_NOT_INSTALL_CODE:
      router.push('/install')
      return false
    // 认证失败
    case AUTH_ERROR_CODE:
      router.push('/user/login')
      return false
  }
  if (code !== SUCCESS_CODE) {
    Message.error({
      message: msg
    })
    return false
  }

  return true
}

function successCallback (res, next, failed) {
  if (!checkResponseCode(res.data.code, res.data.message)) {
    if (failed) failed(new Error(res.data.message))
    return
  }
  if (!next) {
    return
  }
  next(res.data.data, res.data.code, res.data.message)
}

function failureCallback (error) {
  Message.error({
    message: '请求失败 - ' + error
  })
}

export default {
  getPage (uri, params, next, failed) {
    handle(axios.get(uri, {params}), data => next(normalizePageResponse(data)), failed)
  },

  getObject (uri, params, next, failed) {
    handle(axios.get(uri, {params}), data => next(requireObjectResponse(data)), failed)
  },

  get (uri, params, next, failed) {
    const promise = axios.get(uri, {params})
    handle(promise, next, failed)
  },

  batchGet (uriGroup, next) {
    const requests = []
    for (let item of uriGroup) {
      let params = {}
      if (item.params !== undefined) {
        params = item.params
      }
      requests.push(axios.get(item.uri, {params}))
    }

    axios.all(requests).then(axios.spread(function (...res) {
      const result = []
      for (let item of res) {
        if (!checkResponseCode(item.data.code, item.data.message)) {
          return
        }
        result.push(item.data.data)
      }
      next(...result)
    })).catch((error) => failureCallback(error))
  },

  post (uri, data, next, failed) {
    const promise = axios.post(uri, Qs.stringify(data), {
      headers: {
        post: {
          'Content-Type': 'application/x-www-form-urlencoded'
        }
      }
    })
    handle(promise, next, failed)
  }
}
