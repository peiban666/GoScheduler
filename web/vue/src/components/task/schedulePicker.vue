<template>
  <div class="schedule-picker">
    <el-radio-group :value="pickerMode" size="small" @input="changeMode">
      <el-radio-button label="interval">每 X 分钟</el-radio-button>
      <el-radio-button label="hourly">每小时</el-radio-button>
      <el-radio-button label="daily">每天</el-radio-button>
      <el-radio-button label="advanced">高级 Cron</el-radio-button>
    </el-radio-group>
    <div class="schedule-options">
      <div v-if="pickerMode === 'interval'">
        每
        <el-input-number
          v-model="intervalMinutes"
          :min="1"
          :max="maxIntervalMinutes"
          :step="1"
          :precision="0"
          controls-position="right"
          label="执行间隔（分钟）"
          size="small"
          class="schedule-interval">
        </el-input-number>
        分钟执行一次。
        <div class="schedule-hint">填 1 表示每分钟，填 5 表示每 5 分钟。</div>
        <div v-if="state.mode === 'minutely'" class="schedule-hint">当前保留原有逐分钟规则，在第 0 秒触发。</div>
        <div v-else class="schedule-hint">按整分钟执行，秒固定为 00；以当前分钟为起点，跨小时保持相同间隔。</div>
      </div>
      <div v-else-if="state.mode === 'hourly'">
        每小时的第
        <el-select v-model="state.minute" size="small" placeholder="选择分钟" aria-label="每小时执行分钟" class="schedule-minute">
          <el-option v-for="minute in 60" :key="minute - 1" :label="String(minute - 1).padStart(2, '0')" :value="minute - 1"></el-option>
        </el-select>
        分钟执行一次。
      </div>
      <div v-else-if="state.mode === 'daily'">
        <div class="schedule-hint">同一个任务可以设置多个时间点，按调度器所在时区执行。</div>
        <div v-for="(time, index) in state.times" :key="index" class="schedule-time">
          <el-time-picker
            v-model="state.times[index]"
            format="HH:mm"
            value-format="HH:mm"
            :clearable="false"
            placeholder="选择执行时间"
            size="small">
          </el-time-picker>
          <el-button type="text" :disabled="state.times.length === 1" @click="state.times.splice(index, 1)">删除</el-button>
        </div>
        <el-button size="small" icon="el-icon-plus" @click="state.times.push('')">添加时间点</el-button>
      </div>
      <el-input v-else v-model="state.advanced" type="textarea" :rows="3" placeholder="秒 分 时 天 月 周；多个表达式每行一个"></el-input>
    </div>
    <div v-if="result.error" class="schedule-error" role="alert">{{result.error}}</div>
    <div v-else-if="pickerMode !== 'advanced'" class="schedule-preview">
      <strong>{{description}}</strong>
    </div>
  </div>
</template>

<script>
import {buildSchedule, describeSchedule, parseSchedule, MAX_INTERVAL_MINUTES} from '../../utils/schedule'

export default {
  name: 'schedule-picker',
  props: {
    value: {type: String, default: ''}
  },
  data () {
    return {state: parseSchedule(this.value), maxIntervalMinutes: MAX_INTERVAL_MINUTES}
  },
  computed: {
    pickerMode () {
      // Keep legacy whole-minute Cron semantics behind the same visual control.
      return this.state.mode === 'minutely' ? 'interval' : this.state.mode
    },
    intervalMinutes: {
      get () {
        return this.state.intervalMinutes
      },
      set (minutes) {
        this.state.intervalMinutes = minutes
        if (this.state.mode === 'minutely' && minutes !== 1) this.state.mode = 'interval'
      }
    },
    result () {
      try {
        return {spec: buildSchedule(this.state), error: ''}
      } catch (error) {
        return {spec: '', error: error.message}
      }
    },
    description () {
      return describeSchedule(this.result.spec)
    }
  },
  methods: {
    changeMode (mode) {
      const spec = this.result.spec
      const parsed = parseSchedule(spec)
      if (parsed.mode === mode || (parsed.mode === 'minutely' && mode === 'interval')) {
        this.state = parsed
      } else {
        if (spec) this.state.advanced = spec
        this.state.mode = mode
      }
    }
  },
  watch: {
    value (value) {
      if (value !== this.result.spec) this.state = parseSchedule(value)
    },
    'result.spec' (spec) {
      this.$emit('input', spec)
    }
  }
}
</script>

<style scoped>
.schedule-options { margin-top: 16px; }
.schedule-minute { width: 100px; }
.schedule-interval { width: 140px; }
.schedule-hint { color: #909399; font-size: 13px; line-height: 1.6; }
.schedule-time { display: flex; align-items: center; gap: 12px; margin: 10px 0; }
.schedule-time .el-date-editor { width: 160px; }
.schedule-preview { margin-top: 16px; padding: 12px 16px; background: #f5f7fa; border-radius: 4px; line-height: 1.7; }
.schedule-error { color: #f56c6c; margin-top: 8px; }
</style>
