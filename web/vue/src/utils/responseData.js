export function normalizePageResponse (value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('列表接口返回了空数据，请检查服务接口后重试')
  }
  const total = Number(value.total)
  if (value.total === undefined || value.total === null || !Number.isSafeInteger(total) || total < 0) {
    throw new Error('列表接口返回的总数无效')
  }
  // Some backends serialize an empty slice as null, rather than [].
  const data = value.data === null && total === 0 ? [] : value.data
  if (!Array.isArray(data)) throw new Error('列表接口返回的数据格式无效')
  return {...value, total, data}
}

export function requireObjectResponse (value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('配置接口返回了空数据，请检查服务接口后重试')
  }
  return value
}
