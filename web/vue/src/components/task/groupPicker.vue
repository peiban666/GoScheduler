<template>
  <div class="group-picker">
    <el-select
      ref="select"
      :value="selectedValue"
      filterable
      :filter-method="filterGroups"
      clearable
      placeholder="选择分组"
      class="group-select"
      @change="selectValue"
      @visible-change="query = ''">
      <el-option label="未分组" value="ungrouped"></el-option>
      <el-option v-for="name in namedGroups" :key="name" :label="name" :value="'group:' + name"></el-option>
      <el-option value="create" label="＋ 新建分组" class="group-create-option">
        <i class="el-icon-plus"></i> 新建分组
      </el-option>
    </el-select>
    <group-create-dialog v-model="createDialog" @created="groupCreated"></group-create-dialog>
  </div>
</template>

<script>
import groupCreateDialog from './groupCreateDialog'

export default {
  name: 'task-group-picker',
  components: {groupCreateDialog},
  props: {
    value: {type: String, default: ''},
    groups: {type: Array, default: () => []}
  },
  data () {
    return {
      createDialog: false,
      createdNames: [],
      query: ''
    }
  },
  computed: {
    namedGroups () {
      const names = this.groups.map(group => group.name).concat(this.createdNames, this.value)
      return Array.from(new Set(names.filter(Boolean))).filter(name => name.toLowerCase().includes(this.query.toLowerCase()))
    },
    selectedValue () {
      return this.value ? 'group:' + this.value : 'ungrouped'
    }
  },
  methods: {
    selectValue (value) {
      if (value === 'create') {
        this.$refs.select.blur()
        this.createDialog = true
        return
      }
      this.$emit('input', value && value.startsWith('group:') ? value.slice(6) : '')
    },
    filterGroups (query) {
      this.query = query
    },
    groupCreated (name) {
      this.createdNames.push(name)
      this.$emit('input', name)
      this.$emit('created', name)
    }
  }
}
</script>

<style scoped>
.group-picker, .group-select { width: 100%; }
.group-create-option { border-top: 1px solid #ebeef5; color: #409eff; font-weight: 500; }
</style>
