<template>
  <el-dialog
    title="重命名分组"
    :visible="value"
    width="420px"
    append-to-body
    :show-close="!saving"
    :close-on-click-modal="!saving"
    :close-on-press-escape="!saving"
    @open="reset"
    @update:visible="$emit('input', $event)">
    <div class="group-original">当前分组：{{groupName}}</div>
    <el-input
      ref="nameInput"
      v-model="name"
      :disabled="saving"
      placeholder="请输入新分组名称（最多 32 个字符）"
      @keyup.enter.native="submit">
    </el-input>
    <div v-if="error" class="group-error" role="alert">{{error}}</div>
    <div class="group-hint">组内全部任务同步更新分组名称，执行时间和启停状态保持不变。</div>
    <span slot="footer">
      <el-button :disabled="saving" @click="$emit('input', false)">取消</el-button>
      <el-button type="primary" :loading="saving" :disabled="name.trim() === groupName" @click="submit">保存名称</el-button>
    </span>
  </el-dialog>
</template>

<script>
import taskService from '../../api/task'
import {normalizeGroupName} from '../../utils/taskGroups'

export default {
  name: 'group-rename-dialog',
  props: {
    value: {type: Boolean, default: false},
    groupName: {type: String, default: ''}
  },
  data () {
    return {name: '', error: '', saving: false}
  },
  methods: {
    reset () {
      this.name = this.groupName
      this.error = ''
      this.$nextTick(() => this.$refs.nameInput && this.$refs.nameInput.focus())
    },
    submit () {
      if (this.saving || !this.groupName) return
      let name
      try {
        name = normalizeGroupName(this.name)
        if (!name) throw new Error('分组名称不能为空')
      } catch (error) {
        this.error = error.message
        return
      }
      if (name === this.groupName) return
      const oldName = this.groupName
      this.error = ''
      this.saving = true
      taskService.renameGroup(oldName, name, () => {
        this.saving = false
        this.$emit('renamed', {oldName, name})
        this.$emit('input', false)
        this.$message.success('分组已重命名')
      }, error => {
        this.saving = false
        this.error = error.message || '重命名失败，请重试'
      })
    }
  }
}
</script>

<style scoped>
.group-original { margin-bottom: 12px; color: #606266; overflow-wrap: anywhere; }
.group-hint { margin-top: 8px; color: #909399; font-size: 13px; line-height: 1.7; }
.group-error { margin-top: 8px; color: #f56c6c; font-size: 13px; }
</style>
