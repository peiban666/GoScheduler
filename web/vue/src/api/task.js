import httpClient from '../utils/httpClient'

export default {
  hosts (callback) {
    httpClient.get('/host/all', {}, callback)
  },

  groups (query, callback, failed) {
    httpClient.get('/task/groups', query, callback, failed)
  },

  createGroup (tag, callback, failed) {
    httpClient.post('/task/group/create', {tag}, callback, failed)
  },

  renameGroup (tag, newTag, callback, failed) {
    httpClient.post('/task/group/rename', {tag, new_tag: newTag}, callback, failed)
  },

  deleteGroup (tag, deleteTasks, expectedCount, callback, failed) {
    httpClient.post('/task/group/delete', {tag, delete_tasks: deleteTasks ? 1 : 0, expected_count: expectedCount}, callback, failed)
  },

  groupTasks (query, callback, failed) {
    httpClient.getPage('/task', query, callback, failed)
  },

  assignGroup (ids, tag, callback, failed) {
    httpClient.post('/task/group', {ids: ids.join(','), tag}, callback, failed)
  },

  // 任务列表
  list (query, callback) {
    httpClient.batchGet([
      {
        uri: '/task',
        params: query
      },
      {
        uri: '/host/all'
      }
    ], callback)
  },

  detail (id, callback) {
    const requests = [{uri: '/host/all'}, {uri: '/task/groups'}]
    if (!id) {
      httpClient.batchGet(requests, (hosts, groups) => callback(null, hosts, groups))
      return
    }
    httpClient.batchGet([{uri: `/task/${id}`}].concat(requests), callback)
  },

  update (data, callback) {
    httpClient.post('/task/store', data, callback)
  },

  remove (id, callback) {
    httpClient.post(`/task/remove/${id}`, {}, callback)
  },

  enable (id, callback) {
    httpClient.post(`/task/enable/${id}`, {}, callback)
  },

  disable (id, callback) {
    httpClient.post(`/task/disable/${id}`, {}, callback)
  },

  run (id, callback) {
    httpClient.get(`/task/run/${id}`, {}, callback)
  }
}
