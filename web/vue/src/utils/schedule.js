// The existing task/task_log spec columns both have a 64-character limit.
export const MAX_SPEC_LENGTH = 64
export const MAX_INTERVAL_MINUTES = 60
export const MAX_INTERVAL_HOURS = 24

function expressions (spec) {
  return String(spec || '').trim().split(/[;\r\n]+/).map(line => line.trim().split(/\s+/).join(' ')).filter(Boolean)
}

function integer (value, max) {
  return /^\d+$/.test(String(value)) && Number(value) >= 0 && Number(value) <= max
}

export function parseSchedule (spec) {
  const state = {mode: 'advanced', minute: 0, intervalMinutes: 1, intervalHours: 1, times: ['07:30'], advanced: spec || ''}
  const lines = expressions(spec)
  if (lines.length === 1) {
    const fields = lines[0].split(' ')
    // This project's five-field form omits weekday, not seconds.
    if (fields.length === 5) fields.push('*')
    const interval = lines[0].match(/^@every (\d+)m$/)
    if (interval && integer(interval[1], MAX_INTERVAL_MINUTES) && Number(interval[1]) > 0) {
      return Object.assign(state, {mode: 'interval', intervalMinutes: Number(interval[1])})
    }
    if (lines[0] === '@hourly' || lines[0] === '0 0 * * * *') {
      return Object.assign(state, {mode: 'hourly'})
    }
    if (lines[0] === '0 * * * * *') {
      return Object.assign(state, {mode: 'minutely'})
    }
    const minuteStep = fields[1] && fields[1].match(/^\*\/(\d+)$/)
    if (fields.length === 6 && fields[0] === '0' && minuteStep &&
        integer(minuteStep[1], MAX_INTERVAL_MINUTES) && Number(minuteStep[1]) > 0 &&
        fields.slice(2).join(' ') === '* * * *') {
      return Object.assign(state, {mode: 'interval', intervalMinutes: Number(minuteStep[1])})
    }
    const hourStep = fields[2] && fields[2].match(/^\*\/(\d+)$/)
    if (fields.length === 6 && fields[0] === '0' && integer(fields[1], 59) && hourStep &&
        integer(hourStep[1], MAX_INTERVAL_HOURS) && Number(hourStep[1]) > 0 &&
        fields.slice(3).join(' ') === '* * *') {
      return Object.assign(state, {mode: 'hourly', minute: Number(fields[1]), intervalHours: Number(hourStep[1])})
    }
    if (fields.length === 6 && fields[0] === '0' && integer(fields[1], 59) && fields.slice(2).join(' ') === '* * * *') {
      return Object.assign(state, {mode: 'hourly', minute: Number(fields[1])})
    }
    if (lines[0] === '@daily' || lines[0] === '@midnight') {
      return Object.assign(state, {mode: 'daily', times: ['00:00']})
    }
  }
  const times = []
  for (const line of lines) {
    const fields = line.split(' ')
    if (fields.length === 5) fields.push('*')
    if (fields.length !== 6 || fields[0] !== '0' || !integer(fields[1], 59) || fields.slice(3).join(' ') !== '* * *') {
      return state
    }
    const hours = fields[2].split(',')
    if (!hours.every(hour => integer(hour, 23))) {
      return state
    }
    hours.forEach(hour => times.push(`${String(Number(hour)).padStart(2, '0')}:${String(Number(fields[1])).padStart(2, '0')}`))
  }
  if (times.length) {
    return Object.assign(state, {mode: 'daily', times: Array.from(new Set(times)).sort()})
  }
  return state
}

export function buildSchedule (state) {
  let spec = ''
  if (state.mode === 'minutely') {
    spec = '0 * * * * *'
  } else if (state.mode === 'interval') {
    if (!integer(state.intervalMinutes, MAX_INTERVAL_MINUTES) || Number(state.intervalMinutes) < 1) {
      throw new Error(`请输入执行间隔（1–${MAX_INTERVAL_MINUTES} 的整数分钟）`)
    }
    // Calendar steps begin at minute 00 each hour, independent of saving.
    spec = Number(state.intervalMinutes) === 1 ? '0 * * * * *' : `0 */${Number(state.intervalMinutes)} * * * *`
  } else if (state.mode === 'hourly') {
    if (!integer(state.minute, 59)) throw new Error('请选择每小时执行的分钟（0–59）')
    const hours = Object.prototype.hasOwnProperty.call(state, 'intervalHours') ? state.intervalHours : 1
    if (!integer(hours, MAX_INTERVAL_HOURS) || Number(hours) < 1) {
      throw new Error(`请输入执行间隔（1–${MAX_INTERVAL_HOURS} 的整数小时）`)
    }
    spec = `0 ${Number(state.minute)} ${Number(hours) === 1 ? '*' : `*/${Number(hours)}`} * * *`
  } else if (state.mode === 'daily') {
    if (!state.times.length) throw new Error('每天至少添加一个执行时间')
    const groups = {}
    for (const time of state.times) {
      if (!/^([01]\d|2[0-3]):[0-5]\d$/.test(time || '')) throw new Error('请为每个时间点选择有效时间')
      const [hour, minute] = time.split(':').map(Number)
      if (!groups[minute]) groups[minute] = new Set()
      groups[minute].add(hour)
    }
    // Group by minute, not by all hours/minutes independently: no cross-product.
    spec = Object.keys(groups).map(Number).sort((a, b) => a - b).map(minute => {
      const hours = Array.from(groups[minute]).sort((a, b) => a - b).join(',')
      return `0 ${minute} ${hours} * * *`
    }).join('\n')
  } else {
    const lines = Array.from(new Set(expressions(state.advanced)))
    if (lines.length > 1 && lines.some(line => /^@every\s/.test(line))) throw new Error('@every 间隔规则请单独使用，多个时间点请使用六字段 Cron 表达式')
    spec = lines.join('\n')
    if (!spec) throw new Error('请输入 crontab 表达式')
  }
  if (spec.length > MAX_SPEC_LENGTH) throw new Error(`调度表达式超过 ${MAX_SPEC_LENGTH} 个字符，请减少时间点`)
  return spec
}

export function describeSchedule (spec) {
  const state = parseSchedule(spec)
  if (state.mode === 'minutely') return '每分钟'
  if (state.mode === 'interval') return `每 ${state.intervalMinutes} 分钟执行一次（从 00 分起）`
  if (state.mode === 'hourly') {
    const minute = String(state.minute).padStart(2, '0')
    return state.intervalHours === 1 ? `每小时 ${minute} 分` : `每 ${state.intervalHours} 小时 ${minute} 分（从 00 时起）`
  }
  if (state.mode === 'daily') return `每天 ${state.times.join('、')}`
  return expressions(spec).join('；') || '—'
}
