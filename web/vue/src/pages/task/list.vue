<template>
  <el-container>
    <task-sidebar></task-sidebar>
    <el-main>
      <el-form :inline="true" class="task-filters">
        <el-form-item label="任务ID"><el-input v-model.trim="searchParams.id"></el-input></el-form-item>
        <el-form-item label="任务名称"><el-input v-model.trim="searchParams.name"></el-input></el-form-item>
        <el-form-item label="任务分组">
          <el-select v-model="selectedGroup" filterable>
            <el-option label="全部分组" value=""></el-option>
            <el-option label="未分组" value="ungrouped"></el-option>
            <el-option v-for="group in namedGroupChoices" :key="group.key" :label="group.name" :value="group.key"></el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="执行方式">
          <el-select v-model="searchParams.protocol">
            <el-option label="全部" value=""></el-option>
            <el-option label="http" value="1"></el-option><el-option label="shell" value="2"></el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="任务节点">
          <el-select v-model="searchParams.host_id">
            <el-option label="全部" value=""></el-option>
            <el-option v-for="host in hosts" :key="host.id" :label="host.alias + ' - ' + host.name + ':' + host.port" :value="host.id"></el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="searchParams.status">
            <el-option label="全部" value=""></el-option>
            <el-option label="激活" value="2"></el-option><el-option label="停止" value="1"></el-option>
          </el-select>
        </el-form-item>
        <el-form-item class="filter-actions"><el-button type="primary" @click="search()">搜索</el-button></el-form-item>
      </el-form>
      <div class="group-toolbar">
        <span class="group-summary">共 {{groups.length}} 个分组，{{taskTotal}} 个任务</span>
        <el-button v-if="isAdmin" type="primary" @click="toEdit(null)">新增任务</el-button>
        <el-button v-if="isAdmin" icon="el-icon-plus" @click="createDialog = true">新建分组</el-button>
        <el-button v-if="isAdmin" :disabled="selectedIDs.length === 0" @click="moveDialog = true">移入分组<span v-if="selectedIDs.length">（{{selectedIDs.length}}）</span></el-button>
        <el-button v-if="isAdmin" :disabled="selectedIDs.length === 0" icon="el-icon-document-copy" @click="openCopyTasks()">复制到分组<span v-if="selectedIDs.length">（{{selectedIDs.length}}）</span></el-button>
        <el-button :disabled="expandedGroups.length === 0" @click="expandedGroups = []">全部折叠</el-button>
        <el-button @click="refresh">刷新</el-button>
      </div>
      <div class="group-hint">分组默认折叠，点击组名查看任务；勾选任务后可批量移入或复制到分组。</div>
      <div v-loading="loadingGroups" class="group-overview">
        <el-alert v-if="groupError" title="分组加载失败，请点击刷新重试" type="error" :closable="false"></el-alert>
        <div v-else-if="!loadingGroups && groups.length === 0" class="group-empty">暂无符合条件的任务</div>
        <el-collapse v-model="expandedGroups" @change="openGroups">
          <el-collapse-item v-for="group in groups" :key="group.key" :name="group.key">
            <template slot="title">
              <div class="group-title">
                <i class="el-icon-folder"></i><strong>{{group.title}}</strong>
                <el-tag size="small" type="info">{{group.total}} 个任务</el-tag>
                <el-button v-if="isAdmin" type="text" @click.stop="toEdit(null, group)">组内新增</el-button>
                <el-button v-if="isAdmin && group.name" type="text" @click.stop="openRenameGroup(group)">重命名</el-button>
                <el-button v-if="isAdmin && group.name" type="text" class="group-delete-button" @click.stop="openDeleteGroup(group)">删除分组</el-button>
              </div>
            </template>
            <div v-loading="groupStates[group.key].loading" class="group-content">
              <el-button v-if="groupStates[group.key].error" size="small" @click="loadGroup(group)">加载失败，点击重试</el-button>
              <template v-if="groupStates[group.key].loaded">
                <task-table :tasks="groupStates[group.key].tasks" :is-admin="isAdmin"
                  @selection-change="selectTasks(group.key, $event)" @edit="toEdit" @copy="openCopyTasks" @run="runTask" @remove="remove" @log="jumpToLog" @status="changeStatus">
                </task-table>
                <el-pagination background layout="prev, pager, next, sizes, total" :total="groupStates[group.key].total"
                  :pager-count="5"
                  :current-page="groupStates[group.key].page" :page-size="groupStates[group.key].pageSize" :page-sizes="[20, 50, 100]"
                  @current-change="changeGroupPage(group, $event)" @size-change="changeGroupSize(group, $event)">
                </el-pagination>
              </template>
            </div>
          </el-collapse-item>
        </el-collapse>
      </div>
      <el-dialog title="移入任务分组" :visible.sync="moveDialog" width="420px">
        <p>将选中的 {{selectedIDs.length}} 个任务移入同一组，不改变执行时间和启停状态。</p>
        <group-picker v-model="moveGroup" :groups="groupChoices" @created="groupCreated"></group-picker>
        <div class="group-hint">选择已有分组，或点击下拉框底部「＋ 新建分组」；选择未分组可移出当前分组。</div>
        <span slot="footer">
          <el-button :disabled="savingGroup" @click="moveDialog = false">取消</el-button>
          <el-button type="primary" :loading="savingGroup" :disabled="selectedIDs.length === 0" @click="assignGroup">确定移入</el-button>
        </span>
      </el-dialog>
      <el-dialog
        :title="copyIDs.length === 1 ? '复制任务' : '批量复制任务到分组'"
        :visible.sync="copyDialog"
        width="480px"
        :show-close="!copyingTasks"
        :close-on-click-modal="!copyingTasks"
        :close-on-press-escape="!copyingTasks">
        <p>将 {{copyIDs.length}} 个任务复制到以下分组，原任务保留不变。</p>
        <group-picker v-model="copyGroup" :groups="groupChoices" :disabled="copyingTasks" @created="copyGroupCreated"></group-picker>
        <div class="group-hint">可选择已有分组、未分组，或「＋ 新建分组」。</div>
        <div class="group-hint">执行时间、命令、请求体、节点、重试和通知配置一并复制；名称自动添加“副本”。</div>
        <el-alert title="新任务默认停用，不会自动执行；请检查后再启用。" type="info" :closable="false"></el-alert>
        <div class="group-hint">已选任务间的依赖指向新副本；未选中的依赖仍指向原任务，请启用前检查。</div>
        <div v-if="copyError" class="copy-error" role="alert">{{copyError}}</div>
        <span slot="footer">
          <el-button :disabled="copyingTasks" @click="copyDialog = false">取消</el-button>
          <el-button type="primary" :loading="copyingTasks" :disabled="copyIDs.length === 0" @click="confirmCopyTasks">确定复制</el-button>
        </span>
      </el-dialog>
      <group-create-dialog v-model="createDialog" @created="groupCreated"></group-create-dialog>
      <group-rename-dialog v-model="renameDialog" :group-name="renameTarget" @renamed="groupRenamed"></group-rename-dialog>
      <el-dialog
        title="删除任务分组"
        :visible.sync="deleteDialog"
        width="480px"
        :show-close="!deletingGroup"
        :close-on-click-modal="!deletingGroup"
        :close-on-press-escape="!deletingGroup">
        <div v-loading="deleteLoading" class="group-delete-body">
          <template v-if="deleteTarget">
            <p>分组「{{deleteTarget.name}}」共有 <strong>{{deleteTarget.total}}</strong> 个任务。</p>
            <p class="group-hint">以下操作针对整个分组，不仅是当前搜索结果或当前页。</p>
            <el-radio-group v-model="deleteTasks" :disabled="deletingGroup">
              <div class="group-delete-choice">
                <el-radio :label="false">仅删除分组，保留任务</el-radio>
                <div class="group-hint">任务移到「未分组」，执行时间和启停状态保持不变。</div>
              </div>
              <div class="group-delete-choice">
                <el-radio :label="true">删除分组及组内全部任务</el-radio>
                <div class="group-hint">删除组内全部 {{deleteTarget.total}} 个任务并移除后续调度；历史日志保留。</div>
              </div>
            </el-radio-group>
            <el-alert v-if="deleteTasks && deleteTarget.total > 0"
              :title="'将删除 ' + deleteTarget.total + ' 个任务，此操作不可撤销。已在运行的任务不会被强制终止。'"
              type="error" :closable="false">
            </el-alert>
          </template>
        </div>
        <span slot="footer">
          <el-button :disabled="deletingGroup" @click="deleteDialog = false">取消</el-button>
          <el-button
            :type="deleteTasks ? 'danger' : 'primary'"
            :loading="deletingGroup"
            :disabled="deleteLoading || !deleteTarget"
            @click="confirmDeleteGroup">
            {{deleteTasks ? '删除分组及任务' : '仅删除分组'}}
          </el-button>
        </span>
      </el-dialog>
    </el-main>
  </el-container>
</template>

<script>
import taskSidebar from './sidebar'
import taskTable from '../../components/task/taskTable'
import groupPicker from '../../components/task/groupPicker'
import groupCreateDialog from '../../components/task/groupCreateDialog'
import groupRenameDialog from '../../components/task/groupRenameDialog'
import taskService from '../../api/task'
import {groupKey, groupQuery, prepareGroups, normalizeGroupName} from '../../utils/taskGroups'

export default {
  name: 'task-list',
  components: {taskSidebar, taskTable, groupPicker, groupCreateDialog, groupRenameDialog},
  data () {
    return {
      groups: [],
      groupChoices: [],
      hosts: [],
      groupStates: {},
      expandedGroups: [],
      selection: {},
      selectedGroup: '',
      taskTotal: 0,
      loadingGroups: false,
      groupError: false,
      groupRequest: 0,
      choicesRequest: 0,
      moveDialog: false,
      moveGroup: '',
      savingGroup: false,
      copyDialog: false,
      copyIDs: [],
      copyGroup: '',
      copyError: '',
      copyingTasks: false,
      createDialog: false,
      renameDialog: false,
      renameTarget: '',
      deleteDialog: false,
      deleteTarget: null,
      deleteTasks: false,
      deletingGroup: false,
      deleteLoading: false,
      deleteRequest: 0,
      searchParams: {id: '', name: '', protocol: '', host_id: '', status: ''},
      isAdmin: this.$store.getters.user.isAdmin
    }
  },
  computed: {
    namedGroupChoices () { return prepareGroups(this.groupChoices).filter(group => group.name) },
    selectedIDs () {
      return Array.from(new Set(Object.keys(this.selection).reduce((ids, key) => ids.concat(this.selection[key]), [])))
    }
  },
  created () {
    if (this.$route.query.host_id) this.searchParams.host_id = this.$route.query.host_id
    taskService.hosts(hosts => { this.hosts = hosts || [] })
    this.loadChoices()
    this.loadGroups()
  },
  methods: {
    openRenameGroup (group) {
      if (!this.isAdmin || !group.name) return
      this.renameTarget = group.name
      this.renameDialog = true
    },
    groupRenamed ({oldName, name}) {
      const oldKey = groupKey(oldName)
      const newKey = groupKey(name)
      if (this.selectedGroup === oldKey) this.selectedGroup = newKey
      if (this.moveGroup === oldName) this.moveGroup = name
      if (this.copyGroup === oldName) this.copyGroup = name
      this.expandedGroups = this.expandedGroups.map(key => key === oldKey ? newKey : key)
      this.selection = {}
      this.groupStates = {}
      this.loadChoices()
      this.loadGroups()
    },
    groupCreated () {
      this.loadChoices()
      this.loadGroups()
    },
    openDeleteGroup (group) {
      if (!this.isAdmin || !group.name) return
      this.deleteTasks = false
      this.deleteTarget = null
      this.deleteDialog = true
      this.deleteLoading = true
      const request = ++this.deleteRequest
      taskService.groups({tag: group.name}, groups => {
        if (request !== this.deleteRequest || !this.deleteDialog) return
        this.deleteLoading = false
        const target = (groups || []).find(item => item.name === group.name)
        if (!target) {
          this.deleteDialog = false
          this.$message.error('分组已不存在，请刷新后重试')
          this.refresh()
          return
        }
        this.deleteTarget = {name: target.name, total: Number(target.total)}
      }, () => {
        if (request !== this.deleteRequest) return
        this.deleteLoading = false
        this.deleteDialog = false
      })
    },
    confirmDeleteGroup () {
      if (!this.deleteTarget || this.deleteLoading || this.deletingGroup) return
      const {name, total} = this.deleteTarget
      this.deletingGroup = true
      taskService.deleteGroup(name, this.deleteTasks, total, () => {
        this.deletingGroup = false
        this.deleteDialog = false
        if (this.selectedGroup === groupKey(name)) this.selectedGroup = ''
        if (this.moveGroup === name) this.moveGroup = ''
        this.selection = {}
        this.groupStates = {}
        this.expandedGroups = this.deleteTasks ? [] : ['ungrouped']
        this.$message.success(this.deleteTasks ? '分组及组内任务已删除' : '分组已删除，任务已移到未分组')
        this.loadChoices()
        this.loadGroups()
      }, () => {
        this.deletingGroup = false
        this.deleteTarget = null
        this.deleteDialog = false
        this.refresh()
      })
    },
    loadChoices () {
      const request = ++this.choicesRequest
      taskService.groups({}, groups => {
        if (request === this.choicesRequest) this.groupChoices = groups || []
      })
    },
    queryFor (key) { return Object.assign({}, this.searchParams, groupQuery(key)) },
    loadGroups (callback) {
      const request = ++this.groupRequest
      this.loadingGroups = true
      this.groupError = false
      taskService.groups(this.queryFor(this.selectedGroup), groups => {
        if (request !== this.groupRequest) return
        this.groups = prepareGroups(groups)
        this.taskTotal = this.groups.reduce((total, group) => total + group.total, 0)
        this.groups.forEach(group => {
          if (!this.groupStates[group.key]) this.$set(this.groupStates, group.key, {tasks: [], total: 0, page: 1, pageSize: 20, loaded: false, loading: false, error: false, request: 0})
          const state = this.groupStates[group.key]
          state.page = Math.min(state.page, Math.max(1, Math.ceil(group.total / state.pageSize)))
        })
        this.expandedGroups = this.expandedGroups.filter(key => this.groups.some(group => group.key === key))
        this.loadingGroups = false
        this.expandedGroups.forEach(key => this.loadGroup(this.groups.find(group => group.key === key)))
        if (callback) callback()
      }, () => {
        if (request !== this.groupRequest) return
        this.loadingGroups = false
        this.groupError = true
      })
    },
    openGroups (keys) {
      keys.forEach(key => {
        const state = this.groupStates[key]
        if (state && !state.loaded && !state.loading) this.loadGroup(this.groups.find(group => group.key === key))
      })
    },
    loadGroup (group) {
      if (!group) return
      const state = this.groupStates[group.key]
      const request = ++state.request
      state.loading = true
      state.error = false
      this.selectTasks(group.key, [])
      const query = Object.assign(this.queryFor(group.key), {page: state.page, page_size: state.pageSize})
      taskService.groupTasks(query, result => {
        if (this.groupStates[group.key] !== state || request !== state.request) return
        state.tasks = result.data || []
        state.total = Number(result.total || 0)
        state.loaded = true
        state.loading = false
      }, () => {
        if (this.groupStates[group.key] !== state || request !== state.request) return
        state.loading = false
        state.error = true
      })
    },
    changeGroupPage (group, page) { this.groupStates[group.key].page = page; this.loadGroup(group) },
    changeGroupSize (group, size) { this.groupStates[group.key].pageSize = size; this.groupStates[group.key].page = 1; this.loadGroup(group) },
    selectTasks (key, tasks) { this.$set(this.selection, key, tasks.map(task => task.id)) },
    search () {
      this.expandedGroups = []
      this.groupStates = {}
      this.selection = {}
      this.loadGroups()
    },
    refresh () {
      this.selection = {}
      this.loadChoices()
      this.loadGroups(() => this.$message.success('刷新成功'))
    },
    assignGroup () {
      let name
      try { name = normalizeGroupName(this.moveGroup) } catch (error) { this.$message.error(error.message); return }
      const ids = this.selectedIDs
      if (!ids.length) return
      this.savingGroup = true
      taskService.assignGroup(ids, name, () => {
        this.savingGroup = false
        this.moveDialog = false
        this.selection = {}
        this.selectedGroup = ''
        this.expandedGroups = [groupKey(name)]
        this.$message.success('任务已移入分组')
        this.loadChoices()
        this.loadGroups()
      }, () => { this.savingGroup = false })
    },
    openCopyTasks (task) {
      if (!this.isAdmin || this.copyingTasks) return
      const ids = task ? [task.id] : this.selectedIDs
      if (!ids.length) return
      // Freeze the chosen IDs: refreshing group choices must not change the batch.
      this.copyIDs = ids.slice()
      this.copyGroup = task ? (task.tag || '') : ''
      this.copyError = ''
      this.copyDialog = true
      this.loadChoices()
    },
    copyGroupCreated () {
      // Do not reload task pages here: that would clear the user's selection.
      this.loadChoices()
    },
    confirmCopyTasks () {
      if (!this.isAdmin || this.copyingTasks || !this.copyIDs.length) return
      let name
      try { name = normalizeGroupName(this.copyGroup) } catch (error) { this.copyError = error.message; return }
      this.copyError = ''
      this.copyingTasks = true
      taskService.copyTasks(this.copyIDs.slice(), name, result => {
        this.copyingTasks = false
        this.copyDialog = false
        this.copyIDs = []
        this.selection = {}
        this.groupStates = {}
        this.selectedGroup = ''
        this.searchParams = {id: '', name: '', protocol: '', host_id: '', status: ''}
        this.expandedGroups = [groupKey(name)]
        this.$message.success(`已复制 ${result.copied} 个任务，副本默认停用`)
        this.loadChoices()
        this.loadGroups()
      }, error => {
        this.copyingTasks = false
        this.copyError = (error && error.message) || '复制失败，请重试'
      })
    },
    changeStatus (task) {
      const method = task.status ? 'enable' : 'disable'
      taskService[method](task.id)
    },
    runTask (task) {
      this.$appConfirm(() => taskService.run(task.id, () => this.$message.success('任务已开始执行')), true)
    },
    remove (task) {
      this.$appConfirm(() => taskService.remove(task.id, () => this.refresh()))
    },
    jumpToLog (task) { this.$router.push(`/task/log?task_id=${task.id}`) },
    toEdit (task, group) {
      const path = task ? `/task/edit/${task.id}` : '/task/create'
      this.$router.push(!task && group ? {path, query: {group: group.name}} : path)
    }
  }
}
</script>

<style scoped>
.group-toolbar { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; margin-bottom: 10px; }
.group-toolbar .el-button { margin-left: 0; }
.group-summary { margin-right: auto; color: #606266; }
.group-hint { color: #909399; font-size: 13px; line-height: 1.7; margin: 10px 0; }
.copy-error { color: #f56c6c; margin-top: 10px; }
.group-overview { min-height: 80px; }
.group-title { display: flex; flex: 1; min-width: 0; flex-wrap: wrap; align-items: center; gap: 8px; padding: 8px 12px; line-height: 1.6; }
.group-title strong { overflow-wrap: anywhere; }
.group-title i { color: #409eff; font-size: 18px; }
.group-title .group-delete-button { color: #f56c6c; }
.group-content { min-height: 60px; padding: 12px; }
.group-content .el-pagination { margin-top: 12px; }
.group-empty { padding: 28px; text-align: center; color: #909399; }
.group-delete-body { min-height: 160px; }
.group-delete-choice { margin: 16px 0; }
.group-delete-choice .group-hint { margin: 6px 0 0 24px; }
@media (max-width: 900px) {
  .group-summary { flex-basis: 100%; margin-bottom: 4px; }
}
</style>
