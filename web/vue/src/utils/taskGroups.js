export const MAX_GROUP_NAME_LENGTH = 32

export function normalizeGroupName (value) {
  const name = String(value || '').trim()
  if (Array.from(name).length > MAX_GROUP_NAME_LENGTH) throw new Error('分组名称最多 32 个字符')
  if (Array.from(name).some(char => {
    const code = char.codePointAt(0)
    return code < 32 || (code >= 127 && code <= 159)
  })) throw new Error('分组名称请使用单行文本')
  return name
}

// Prefix keys so named groups cannot collide with the default or object keys.
export function groupKey (name) {
  return name ? `group:${name}` : 'ungrouped'
}

export function groupQuery (key) {
  if (!key) return {tag: '', ungrouped: 0}
  if (key === 'ungrouped') return {tag: '', ungrouped: 1}
  if (key.startsWith('group:')) return {tag: key.slice(6), ungrouped: 0}
  throw new Error('无效的任务分组')
}

export function prepareGroups (groups) {
  return (groups || []).map(group => ({
    ...group, total: Number(group.total), key: groupKey(group.name), title: group.name || '未分组'
  }))
}
