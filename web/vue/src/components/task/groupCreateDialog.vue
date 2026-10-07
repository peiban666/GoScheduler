<template>
  <el-dialog
    title="新建分组"
    :visible="value"
    width="380px"
    append-to-body
    :show-close="!saving"
    :close-on-click-modal="!saving"
    :close-on-press-escape="!saving"
    @open="reset"
    @update:visible="$emit('input', $event)">
    <el-input
      ref="nameInput"
      v-model="name"
      :disabled="saving"
      placeholder="请输入分组名称（最多 32 个字符）"
      @keyup.enter.native="submit">
    </el-input>
    <div v-if="error" class="group-error" role="alert">{{error}}</div>
    <div class="group-hint">可以先建空分组，再添加或移入任务。</div>
    <span slot="footer">
      <el-button :disabled="saving" @click="$emit('input', false)">取消</el-button>
      <el-button type="primary" :loading="saving" @click="submit">创建分组</el-button>
    </span>
  </el-dialog>
</template>

<script>
import taskService from '../../api/task'
import {normalizeGroupName} from '../../utils/taskGroups'

export default {
  name: 'group-create-dialog',
  props: {value: {type: Boolean, default: false}},
  data () {
    return {name: '', error: '', saving: false}
  },
  methods: {
    reset () {
      this.name = ''
      this.error = ''
      this.$nextTick(() => this.$refs.nameInput && this.$refs.nameInput.focus())
    },
    submit () {
      if (this.saving) return
      let name
      try {
        name = normalizeGroupName(this.name)
        if (!name) throw new Error('分组名称不能为空')
      } catch (error) {
        this.error = error.message
        return
      }
      this.error = ''
      this.saving = true
      taskService.createGroup(name, () => {
        this.saving = false
        this.$emit('created', name)
        this.$emit('input', false)
        this.$message.success('分组已创建')
      }, error => {
        this.saving = false
        this.error = error.message || '创建失败，请重试'
      })
    }
  }
}
</script>

<style scoped>
.group-hint { margin-top: 8px; color: #909399; font-size: 13px; }
.group-error { margin-top: 8px; color: #f56c6c; font-size: 13px; }
</style>
